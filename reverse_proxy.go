package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// ReverseProxy proxied HTTP requests to its behine-NAT workers
type ReverseProxy struct {
	// LogError is used for error logging.
	// By default the function set via SetErrorLogger() is used.
	LogError LoggerFunc

	// Connection statistics.
	// The stats doesn't reset automatically. Feel free resetting it
	// any time you wish.
	Stats ConnStats

	// keeps track of total request proxied
	requestcount uint64

	// protects array of workers inside 'rules' and 'defaults'
	// we intentionally only read 'rules' and 'defaults' after initialized them,
	// since maps allow concurrent read with no write, so we don't have to lock
	// when use them, but the value inside them changed often when workers are
	// connected or disconnected, therfore it need to be protected.
	lock *sync.Mutex
	// map domain to its routing rules, rules is represent as root of a tree
	rules map[string]*node
	// map domain with its default workers, those workers will handle requests
	// which doesn't match any rules defined in roots for a givien domain
	defaults map[string]Handle

	// holds all routing rules
	config *Config
}

// NewReverseProxy setups a new ReverseProxy server
// The configuration in /etc/gorpc.json will be loaded
// After this, user can start the server by calling Serve().
func NewReverseProxy(config *Config) *ReverseProxy {
	if config == nil {
		config = loadConfig()
	}

	s := &ReverseProxy{
		lock:     &sync.Mutex{},
		rules:    make(map[string]*node),
		defaults: make(map[string]Handle),
		config:   config,
	}

	// register
	for _, host := range config.GetHosts() {
		for _, domain := range host.GetDomains() {
			rule := &node{}
			s.rules[domain] = rule
			for _, path := range host.GetPaths() {
				println("ADD ROUTE", domain, path)
				rule.addRoute(path, &handle{})
			}

			s.defaults[domain] = &handle{}
		}
	}
	return s
}

// Serve starts reverse proxy server and http server and blocks forever
func (me *ReverseProxy) Serve(rpc_addr, http_addr string) {
	fmt.Println("HTTP SERVER IS LISTENING AT", http_addr)
	go func() {
		if err := fasthttp.ListenAndServe(http_addr, me.handleHTTPRequest); err != nil {
			fmt.Println(err)
		}
	}()

	if me.LogError == nil {
		me.LogError = errorLogger
	}

	fmt.Println("RPC SERVER IS LISTENING AT", rpc_addr)

	listener := &defaultListener{}
	if err := listener.Init(rpc_addr); err != nil {
		panic(err)
	}
	// the main tcp accept loop
	for {
		conn, _, err := listener.Accept()
		if err != nil {
			me.Stats.incAcceptErrors()
			me.LogError("gorpc.Server: [%s]. Cannot accept new connection: [%s]", rpc_addr, err)
			continue
		}
		me.Stats.incAcceptCalls()
		go me.handleNewWorker(conn)
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

	query := make(map[string][]byte)
	ctx.QueryArgs().VisitAll(func(key, val []byte) { query[string(key)] = val })

	form := make(map[string][]byte)
	ctx.PostArgs().VisitAll(func(key, val []byte) { form[string(key)] = val })

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
		Query:      query,
		Form:       form,
	}
}

// handleHTTPRequest received a HTTP request, forwarded it to matched workers
// then sent back response to the client
func (me *ReverseProxy) handleHTTPRequest(ctx *fasthttp.RequestCtx) {
	domain, path := string(ctx.Host()), string(ctx.Path())

	// find the matched workers for request domain and path
	rule := me.rules[domain]
	if rule == nil {
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found domain [%s], path %s", domain, path)
		return
	}
	var workers []*Client // workers matched request domain and path
	h, _, _ := rule.getValue(path)
	if handler, _ := h.(Handle); h != nil {
		workers = handler.workers
	} else { // no handler found, fallback to default workers
		workers = me.defaults[domain].workers
	}

	if len(workers) == 0 {
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found any client %s", path)
		return
	}

	// pick client in workers using round robin strategy
	count := int(atomic.AddUint64(&me.requestcount, 1))
	me.lock.Lock()
	worker := workers[count%len(workers)]
	me.lock.Unlock()

	// TODO: implement retry, circuit breaker
	res, err := worker.Call(convertRequest(ctx))
	if err != nil {
		ctx.Response.Header.SetStatusCode(500)
		fmt.Fprintf(ctx, "err: %s", err.Error())
		return
	}

	for _, cookie := range res.Cookies {
		ctx.Response.Header.Cookie(toFastHTTPCookie(cookie))
	}
	ctx.Response.Header.SetStatusCode(int(res.StatusCode))
	for k, v := range res.Header {
		ctx.Response.Header.SetBytesV(k, v)
	}
	ctx.Response.SetBody(res.Body)
}

// toFastHTTPCookie convert proto.Cookie to fastHTTP.Cookie
func toFastHTTPCookie(cookie *Cookie) *fasthttp.Cookie {
	cook := &fasthttp.Cookie{}
	cook.SetHTTPOnly(cookie.HttpOnly)
	cook.SetKey(cookie.Name)
	cook.SetValueBytes(cookie.Value)
	cook.SetPath(cookie.Path)
	cook.SetSecure(cookie.Secure)
	cook.SetExpire(time.Unix(cookie.ExpiredSec, 0))
	return cook
}

// cleanFailedWorkers removes all disconnected or unresponsed workers
// in all roots and default workers
// TODO: this function is slow, it scan through all paths in rule, all workers
// when we call this function every time a client is disconnected, we may create
// a bottleneck. I leave it for now but we should must measure it more carefully
func (me *ReverseProxy) cleanFailedWorkers() {
	me.lock.Lock()
	for _, host := range me.config.GetHosts() {
		for _, domain := range host.GetDomains() {
			// remove failed worker in defaults
			newworkers := make([]*Client, 0)
			defhandler := me.defaults[domain]
			for _, worker := range defhandler.workers {
				if !worker.IsStopped {
					newworkers = append(newworkers, worker)
				}
			}
			defhandler.workers = newworkers

			// remove failed workers in rules
			rule := me.rules[domain]
			for _, path := range host.GetPaths() {
				h, _, _ := rule.getValue(path)
				handler, _ := h.(Handle)
				newworkers := make([]*Client, 0)
				for _, client := range handler.workers {
					if !client.IsStopped {
						newworkers = append(newworkers, client)
					}
				}
				handler.workers = newworkers
			}
		}
	}
	me.lock.Unlock()
}

// handleNewWorker registers a new worker to the server
// this function is called after after a worker has established a new
// connection with the server.
func (me *ReverseProxy) handleNewWorker(conn io.ReadWriteCloser) {
	var dialed bool
	var worker *Client
	worker = &Client{
		Dial: func(_ string) (io.ReadWriteCloser, error) {
			if !dialed {
				dialed = true
				return conn, nil
			}
			// old connection is dead, we will not reuse the client
			// because we are unable to reconnect to the NAT hided api server
			go worker.Stop()
			worker.IsStopped = true
			me.cleanFailedWorkers()
			return nil, fmt.Errorf("STOPEED")
		},
	}
	worker.Start()

	// first message is from the server to the endpoint
	// endpoint should return paths that its able to handle
	response, err := worker.Call(Request{Uri: []byte("_status")})
	if err != nil {
		// oops, wrong protocol, or there is something wrong, cleaning
		// me.LogError("gorpc.Server:. Error [%s]", err)
		conn.Close()
		return
	}

	status := &StatusResponse{}
	if err := json.Unmarshal(response.Body, status); err != nil {
		// wrong answer
		// me.LogError("gorpc.Server: unable to get status [%s]", err)
		conn.Close()
		return
	}

	for _, domain := range status.GetDomains() {
		rule := me.rules[domain]
		if rule == nil {
			fmt.Println("ignoring domain", domain)
			continue
		}
		for _, path := range status.GetPaths() {
			if path == "" { // no route
				defhandler := me.defaults[domain]
				me.lock.Lock()
				defhandler.workers = appendOnce(defhandler.workers, worker)
				me.lock.Unlock()
				continue
			}

			h, _, _ := rule.getValue(path)
			if handler, _ := h.(Handle); handler != nil {
				handler.workers = appendOnce(handler.workers, worker)
			}
		}
	}
}

// loadConfig reads configuration in /etc/gorpc.json.
// this function panic if the file is not found or contains malform content
// configuration format must follows message Config in ./message.proto. See
// ./gorpc.json for an example.
func loadConfig() *Config {
	b, err := ioutil.ReadFile("/etc/gorpc.json")
	if err != nil {
		fmt.Println("/etc/gorpc.json not found")
		panic(err)
	}
	config := &Config{}
	if err := json.Unmarshal(b, config); err != nil {
		fmt.Println("invalid config")
		panic(err)
	}
	return config
}

// handle holds a reference to slice of worker
type handle struct {
	workers []*Client
}

// Handler is null-able handler
type Handle *handle

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
