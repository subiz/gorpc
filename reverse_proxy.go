package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// ReverseProxy proxied HTTP requests to its behine-NAT workers
type ReverseProxy struct {
	// Log is used for error logging.
	// By default the function set via SetErrorLogger() is used.
	Log LoggerFunc

	// keeps track of total request proxied
	requestcount uint64

	// protects rules and defaults
	lock *sync.Mutex
	// map domain to its routing rules, rules is represent as root of a tree
	rules map[string]*node
	// map domain with its default workers, those workers will handle requests
	// which doesn't match any rules defined in roots for a givien domain
	defaults map[string]*Handle
}

// NewReverseProxy setups a new ReverseProxy server
// The configuration in /etc/gorpc.json will be loaded
// After this, user can start the server by calling Serve().
func NewReverseProxy() *ReverseProxy {
	return &ReverseProxy{
		lock:     &sync.Mutex{},
		rules:    make(map[string]*node),
		defaults: make(map[string]*Handle),
		Log:      muteLogger,
	}
}

// Serve starts reverse proxy server and http server and blocks forever
func (me *ReverseProxy) Serve(rpc_addr, http_addr string) {
	me.Log("HTTP SERVER IS LISTENING AT %s", http_addr)
	go func() {
		if err := fasthttp.ListenAndServe(http_addr, me.handleHTTPRequest); err != nil {
			me.Log("ERR %v", err)
		}
	}()

	me.Log("RPC SERVER IS LISTENING AT %s", rpc_addr)

	listener := &defaultListener{}
	if err := listener.Init(rpc_addr); err != nil {
		panic(err)
	}
	// the main tcp accept loop
	for {
		conn, addr, err := listener.Accept()
		if err != nil {
			me.Log("Cannot accept new connection: [%s]", err)
			continue
		}
		go me.handleNewWorker(conn, addr)
	}
}

// convertRequest transforms an fastHTTP request to a Request oject so we can
// marshal and send it through the wire.
// note that, for the sake of simplicity, this transform breaks some HTTP rules:
//  + all header keys will be lowercased
//  + when there are multiple headers with a same key, only take the last one
//  + when there are multiple cookies with a same key, only take the last one
//  + when there are mutiple query parameters with a same key, only take the
//      last one
func convertRequest(ctx *fasthttp.RequestCtx) Request {
	rawheader := ctx.Request.Header

	cookies := make(map[string][]byte)
	rawheader.VisitAllCookie(func(key, val []byte) { cookies[string(key)] = val })

	header := make(map[string][]byte)
	rawheader.VisitAll(func(k, val []byte) { header[string(bytes.ToLower(k))] = val })
	delete(header, "cookie")
	delete(header, "user-agent")

	query1 := make(map[string][]byte)
	query := make(map[string]string)
	ctx.QueryArgs().VisitAll(func(key, val []byte) {
		query[string(key)] = string(val)
		query1[string(key)] = val
	})

	form1 := make(map[string][]byte)
	form := make(map[string]string)
	ctx.PostArgs().VisitAll(func(key, val []byte) {
		form[string(key)] = string(val)
		form1[string(key)] = val
	})

	ip, _, _ := net.SplitHostPort(ctx.RemoteAddr().String())
	return Request{
		Version:    "0.1",
		Body:       ctx.Request.Body(),
		Method:     string(ctx.Method()),
		Uri:        ctx.RequestURI(),
		UserAgent:  rawheader.UserAgent(),
		Cookie:     cookies,
		Header:     header,
		RemoteAddr: ip,
		Referer:    string(ctx.Referer()),
		Received:   time.Now().UnixNano() / 1e6,
		Path:       string(ctx.Path()),
		Host:       string(ctx.Host()),
		Query1:     query1,
		Form1:      form1,
		Query:      query,
		Form:       form,
	}
}

// handleHTTPRequest received a HTTP request, forwarded it to matched workers
// then sent back response to the client
func (me *ReverseProxy) handleHTTPRequest(ctx *fasthttp.RequestCtx) {
	domain, path := string(ctx.Host()), string(ctx.Path())

	me.lock.Lock()
	// find the matched workers for request domain and path
	rule := me.rules[domain]
	if rule == nil {
		me.lock.Unlock()
		ctx.Response.Header.SetStatusCode(503)
		fmt.Fprintf(ctx, "no avaiable workers for domain %s%s", domain, path)
		return
	}
	var workers []*Client // workers matched request domain and path
	h, _, _ := rule.getValue(path)
	if handler, _ := h.(*Handle); handler != nil {
		workers = handler.workers
	} else { // no handler found, fallback to default workers
		if defhandler := me.defaults[domain]; defhandler != nil {
			workers = defhandler.workers
		}
	}

	if len(workers) == 0 {
		me.lock.Unlock()
		ctx.Response.Header.SetStatusCode(503)
		fmt.Fprintf(ctx, "no available workers for path %s%s", domain, path)
		return
	}

	// pick client in workers using round robin strategy
	count := int(atomic.AddUint64(&me.requestcount, 1))
	worker := workers[count%len(workers)]
	me.lock.Unlock()

	// TODO: implement retry, circuit breaker
	res, err := worker.Call(convertRequest(ctx))
	if err != nil {
		ctx.Response.Header.SetStatusCode(502)
		fmt.Fprintf(ctx, "err: %s", err.Error())
		return
	}

	for _, cookie := range res.Cookies {
		cook := toFastHTTPCookie(cookie)
		ctx.Response.Header.SetBytesV("Set-Cookie", cook.Cookie())
	}
	ctx.Response.Header.SetStatusCode(int(res.StatusCode))
	for k, v := range res.Header {
		ctx.Response.Header.SetBytesV(k, v)
	}
	ctx.Response.SetBody(res.Body)
}

// toFastHTTPCookie convert proto.Cookie to fastHTTP.Cookie
func toFastHTTPCookie(cookie *Cookie) *fasthttp.Cookie {
	cook := fasthttp.AcquireCookie()
	cook.SetHTTPOnly(cookie.HttpOnly)
	cook.SetKey(cookie.Name)
	cook.SetValueBytes(cookie.Value)
	cook.SetPath(cookie.Path)
	cook.SetDomain(cookie.Domain)
	// cook.SetSecure(cookie.Secure)
	cook.SetExpire(time.Unix(time.Now().Unix()+cookie.ExpiredSec, 0))
	return cook
}

// cleanFailedWorkers removes all disconnected or unresponsed workers
// in all roots and default workers
// TODO: this function is slow, it scan through all paths in rule, all workers
// when we call this function every time a client is disconnected, we may create
// a bottleneck. I leave it for now but we should must measure it more carefully
func (me *ReverseProxy) cleanFailedWorkers() {
	me.lock.Lock()
	for _, node := range me.rules {
		// remove failed workers in rules
		node.traversal(func(_ string, h interface{}) {
			handler, _ := h.(*Handle)
			if handler == nil {
				return
			}
			// quick check if there is failed workers
			hasFailedWorkers := false
			for _, client := range handler.workers {
				if client.IsStopped {
					hasFailedWorkers = true
					break
				}
			}
			if !hasFailedWorkers {
				return
			}

			newworkers := make([]*Client, 0)
			for _, client := range handler.workers {
				if !client.IsStopped {
					newworkers = append(newworkers, client)
				}
			}
			handler.workers = newworkers
		})
	}

	for _, handler := range me.defaults {
		// quick check if there is failed workers
		hasFailedWorkers := false
		for _, client := range handler.workers {
			if client.IsStopped {
				hasFailedWorkers = true
				break
			}
		}
		if !hasFailedWorkers {
			continue
		}

		// remove failed worker in defaults
		newworkers := make([]*Client, 0)
		for _, worker := range handler.workers {
			if !worker.IsStopped {
				newworkers = append(newworkers, worker)
			}
		}
		handler.workers = newworkers
	}
	me.lock.Unlock()
}

// handleNewWorker registers a new worker to the server
// this function is called after after a worker has established a new
// connection with the server.
func (me *ReverseProxy) handleNewWorker(conn io.ReadWriteCloser, addr string) {
	var dialed bool
	var worker *Client
	worker = &Client{
		Addr: addr,
		Dial: func(_ string) (io.ReadWriteCloser, error) {
			if !dialed {
				dialed = true
				return conn, nil
			}
			// old connection is dead, we will not reuse the client
			// because we are unable to reconnect to the NAT hided api server
			worker.IsStopped = true
			me.cleanFailedWorkers()
			go worker.Stop()
			time.Sleep(2 * time.Second)
			return nil, fmt.Errorf("STOPEED")
		},
	}
	worker.Start()

	// worker is now ready
	// first message is from the server to the endpoint
	// endpoint should return paths that its able to handle
	response, err := worker.Call(Request{Uri: []byte("_status")})
	if err != nil {
		// oops, wrong protocol, or there is something wrong, cleaning
		me.Log("CANNOT CALL %s", err)
		conn.Close()
		return
	}

	status := &StatusResponse{}
	if err := json.Unmarshal(response.Body, status); err != nil {
		// wrong answer
		me.Log("gorpc.Server: unable to get status [%s]", err)
		conn.Close()
		return
	}

	me.lock.Lock()
	for _, domain := range status.GetDomains() {
		rule := me.rules[domain]
		if rule == nil {
			rule = &node{}
			me.rules[domain] = rule
		}

		for _, path := range status.GetPaths() {
			if path == "" { // no route
				defhandler := me.defaults[domain]
				if defhandler == nil {
					defhandler = &Handle{}
					me.defaults[domain] = defhandler
				}
				defhandler.workers = appendOnce(defhandler.workers, worker)
				me.Log("REGISTERING DEF %s", domain)
				continue
			}

			h, _, _ := rule.getValue(path)
			handler, _ := h.(*Handle)
			if handler == nil {
				handler = &Handle{}
				rule.addRoute(path, handler)
				me.Log("REGISTERING %s %s", domain, path)
			}
			handler.workers = appendOnce(handler.workers, worker)
		}
	}
	me.lock.Unlock()
}

// handle holds a reference to slice of worker
type Handle struct {
	workers []*Client
}

// appendOnce adds client to workers if it doesn't existed in workers
func appendOnce(workers []*Client, client *Client) []*Client {
	isexisted := false
	for _, oldclient := range workers {
		if oldclient == client {
			isexisted = true
			break
		}
	}
	if !isexisted {
		return append(workers, client)
	}
	return workers
}
