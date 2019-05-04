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

type ReverseServer struct {
	// LogError is used for error logging.
	//
	// By default the function set via SetErrorLogger() is used.
	LogError LoggerFunc

	// Connection statistics.
	//
	// The stats doesn't reset automatically. Feel free resetting it
	// any time you wish.
	Stats ConnStats

	requestcount uint64 // atomic, number of request sent, used to round robin

	lock        *sync.Mutex // protect roots and def_clients
	roots       map[string]*node
	def_clients map[string][]*Client

	config Config
}

// NewReverseServer creates a new ReverseServer object
func NewReverseServer() *ReverseServer {
	s := &ReverseServer{
		lock:        &sync.Mutex{},
		roots:       make(map[string]*node),
		def_clients: make(map[string][]*Client),
		config:      loadConfig(),
	}

	for _, r := range s.config.GetRules() {
		for _, domain := range r.GetDomains() {
			root := &node{}
			s.roots[domain] = root
			for _, path := range r.GetPaths() {
				root.addRoute(path, &handle{})
			}
		}
	}
	return s
}

// Serve starts reverse proxy tcp server and http server
func (me *ReverseServer) Serve(rpc_addr, http_addr string) {
	fmt.Println("HTTP SERVER IS LISTENING AT", http_addr)
	go func() {
		if err := fasthttp.ListenAndServe(http_addr, me.requestHandler); err != nil {
			fmt.Println(err)
		}
	}()

	if me.LogError == nil {
		me.LogError = errorLogger
	}

	listener := &defaultListener{}
	if err := listener.Init(rpc_addr); err != nil {
		panic(err)
	}

	go me.cleanFailedClient()
	fmt.Println("RPC SERVER IS LISTENING AT", rpc_addr)
	// the main tcp accept loop
	for {
		conn, _, err := listener.Accept()
		if err != nil {
			me.Stats.incAcceptErrors()
			me.LogError("gorpc.Server: [%s]. Cannot accept new connection: [%s]", rpc_addr, err)
			continue
		}
		me.Stats.incAcceptCalls()
		go me.newConnection(conn)
	}
}

var SLASH = []byte("/")

// convertRequest transforms an HTTP request to proto.Request
// so we can send it through the wire using TCP protocol
func convertRequest(ctx *fasthttp.RequestCtx, ps map[string]string) Request {
	cookies := make(map[string][]byte)
	ctx.Request.Header.VisitAllCookie(func(key, val []byte) { cookies[string(key)] = val })
	headers := make(map[string][]byte)
	ctx.Request.Header.VisitAll(func(k, val []byte) {
		key := string(bytes.ToLower(k))
		if key == "content-type" || key == "cookie" || key == "user-agent" {
			return
		}
		headers[key] = val
	})

	query := make(map[string][]byte)
	ctx.QueryArgs().VisitAll(func(key, val []byte) { query[string(key)] = val })

	form := make(map[string][]byte)
	ctx.PostArgs().VisitAll(func(key, val []byte) { form[string(key)] = val })

	ip, _, _ := net.SplitHostPort(ctx.RemoteAddr().String())
	return Request{
		Version:     "0.1",
		Body:        ctx.Request.Body(),
		Method:      string(ctx.Method()),
		Uri:         ctx.RequestURI(),
		ContentType: string(ctx.Request.Header.ContentType()),
		UserAgent:   ctx.Request.Header.UserAgent(),
		Cookie:      cookies,
		Header:      headers,
		RemoteAddr:  ip,
		Referer:     string(ctx.Referer()),
		Received:    time.Now().UnixNano() / 1e6,
		Path:        string(ctx.Path()),
		Host:        string(ctx.Host()),
		Query:       query,
		Form:        form,
		Param:       ps,
	}
}

func (me *ReverseServer) requestHandler(ctx *fasthttp.RequestCtx) {
	// firstpath := bytes.Split(ctx.Path(), SLASH)[1]
	// host := ctx.Host()
	host := string(ctx.Method())
	path := string(ctx.Host()) + "|" + host + "|" + string(ctx.Path())

	root := me.roots[host]
	if root == nil {
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found %s", ctx.Path())
		return
	}

	clients := me.def_clients[host]
	h, ps, _ := root.getValue(path)
	if h != nil {
		clients = h.clients
	}

	if len(clients) == 0 {
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found any client %s", ctx.Path())
		return
	}

	count := int(atomic.AddUint64(&me.requestcount, 1))
	me.lock.Lock()
	client := clients[count%len(clients)]
	me.lock.Unlock()

	// TODO: implement retry, circuit breaker
	res, err := client.Call(convertRequest(ctx, ps))

	if err != nil {
		ctx.Response.Header.SetStatusCode(500)
		fmt.Fprintf(ctx, "internal err "+err.Error())
		return
	}
	if res.Error != "" {
		ctx.Response.Header.SetStatusCode(500)
		fmt.Fprintf(ctx, "internal err "+res.Error)
		return
	}
	if res.ContentType != "" {
		ctx.Response.Header.SetContentType(res.ContentType)
	}

	for _, cookie := range res.Cookies {
		ctx.Response.Header.Cookie(toFastHttpCookie(cookie))
	}

	ctx.Response.Header.SetStatusCode(int(res.StatusCode))
	for k, v := range res.Header {
		ctx.Response.Header.SetBytesV(k, v)
	}
	ctx.Response.SetBody(res.Body)
}

func toFastHttpCookie(cookie *Cookie) *fasthttp.Cookie {
	cook := &fasthttp.Cookie{}
	cook.SetHTTPOnly(cookie.GetHttpOnly())
	cook.SetKey(cookie.GetName())
	cook.SetValueBytes(cookie.GetValue())
	cook.SetPath(cookie.GetPath())
	cook.SetSecure(cookie.GetSecure())
	cook.SetExpire(time.Unix(cookie.GetExpiredSec(), 0))
	return cook
}

func (me *ReverseServer) cleanFailedClient() {
	for {
		// removing client from the routing map
		me.lock.Lock()
		for _, r := range me.config.GetRules() {
			for _, domain := range r.GetDomains() {
				root := me.roots[domain]
				for _, path := range r.GetPaths() {
					handle, _, _ := root.getValue(path)
					newclients := make([]*Client, 0)
					for _, client := range handle.clients {
						if !client.IsStopped {
							newclients = append(newclients, client)
						}
					}
					handle.clients = newclients
				}
			}
		}

		for k, clients := range me.def_clients {
			newclients := make([]*Client, 0)
			for _, client := range clients {
				if !client.IsStopped {
					newclients = append(newclients, client)
				}
			}
			me.def_clients[k] = newclients
		}

		me.lock.Unlock()
		time.Sleep(1 * time.Minute)
	}
}

func (me *ReverseServer) newConnection(conn io.ReadWriteCloser) {
	var dialed bool
	var client *Client
	client = &Client{
		Dial: func(_ string) (io.ReadWriteCloser, error) {
			if !dialed {
				dialed = true
				return conn, nil
			}
			// old connection is dead, we will not reuse the client
			// because we are unable to reconnect to the NAT hided api server
			go client.Stop()
			client.IsStopped = true
			return nil, fmt.Errorf("STOPEED")
		},
	}
	client.Start()

	// first message is from the server to the endpoint
	// endpoint should return paths that its able to handle
	response, err := client.Call(Request{Uri: []byte("_status")})
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

	me.lock.Lock()
	for _, domain := range status.GetDomains() {
		root := me.roots[domain]
		if root == nil {
			fmt.Println("ignoring domain", domain)
			continue
		}

		for _, path := range status.GetPaths() {
			if path == "" { // no route
				me.def_clients[domain] = appendOnce(me.def_clients[domain], client)
				continue
			}

			h, _, _ := root.getValue(path)
			if h == nil {
				fmt.Println("ignoring domain", domain)
				continue
			}
			h.clients = appendOnce(h.clients, client)
		}
	}
	me.lock.Unlock()
}

func loadConfig() Config {
	b, err := ioutil.ReadFile("/etc/gorpc/config.json")
	if err != nil {
		fmt.Println("/etc/gorpc/config.json not found")
		panic(err)
	}
	config := Config{}
	if err := json.Unmarshal(b, &config); err != nil {
		fmt.Println("invalid config")
		panic(err)
	}
	return config
}

type handle struct {
	clients []*Client
}
type Handle *handle

type Params map[string]string

func appendOnce(clients []*Client, client *Client) []*Client {
	isexisted := false
	for _, oldclient := range clients {
		if oldclient == client {
			isexisted = true
			break
		}
	}
	if !isexisted {
		clients = append(clients, client)
	}
	return clients
}
