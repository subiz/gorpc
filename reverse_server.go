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

	lock            *sync.Mutex // protect clients_by_path and clients
	clients_by_path map[string][]*Client
}

// NewReverseServer creates a new ReverseServer object
func NewReverseServer() *ReverseServer {
	return &ReverseServer{
		lock:            &sync.Mutex{},
		clients_by_path: make(map[string][]*Client),
	}
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
func convertRequest(ctx *fasthttp.RequestCtx) Request {
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
		Cookies:     cookies,
		Headers:     headers,
		RemoteAddr:  ip,
		Referer:     string(ctx.Referer()),
		Received:    time.Now().UnixNano() / 1e6,
		Path:        string(ctx.Path()),
		Host:        string(ctx.Host()),
		Query:       query,
		Form:        form,
	}
}

// pickClient selects a client from the pools based on the given url route
// it returns nil if there is no ready clients for the route
func (me *ReverseServer) pickClient(route string) *Client {
	// select the client
	count := int(atomic.AddUint64(&me.requestcount, 1))
	me.lock.Lock()
	clients := me.clients_by_path[route]
	if len(clients) == 0 {
		me.lock.Unlock()
		return nil
	}
	client := clients[count%len(clients)]
	me.lock.Unlock()
	return client
}

func (me *ReverseServer) requestHandler(ctx *fasthttp.RequestCtx) {
	firstpath := bytes.Split(ctx.Path(), SLASH)[1]
	host := ctx.Host()
	route := string(host) + "/" + string(firstpath)
	client := me.pickClient(route)
	if client == nil {
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found any client %s", route)
		return
	}
	// TODO: implement retry, circuit breaker
	res, err := client.Call(convertRequest(ctx))
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
	for k, v := range res.Headers {
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

			// removing client from the routing map
			me.lock.Lock()
			for k, clients := range me.clients_by_path {
				for i, c := range clients {
					if c == client {
						clients = append(clients[:i], clients[i+1:]...)
					}
				}
				me.clients_by_path[k] = clients
			}
			me.lock.Unlock()
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
	for _, p := range status.GetPaths() {
		clients := me.clients_by_path[p]
		me.clients_by_path[p] = append(clients, client)
	}
	me.lock.Unlock()
}
