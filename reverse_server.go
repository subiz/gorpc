package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

type ReverseServer struct {
	// Address to listen to for incoming connections.
	RPCAddr string

	HttpPort int

	// OnConnect is called whenever connection from client is accepted.
	// The callback can be used for authentication/authorization/encryption
	// and/or for custom transport wrapping.
	//
	// See also Listener, which can be used for sophisticated transport
	// implementation.
	OnConnect OnConnectFunc

	// The server obtains new client connections via Listener.Accept().
	//
	// Override the listener if you want custom underlying transport
	// and/or client authentication/authorization.
	// Don't forget overriding Client.Dial() callback accordingly.
	//
	// See also OnConnect for authentication/authorization purposes.
	//
	// * NewTLSClient() and NewTLSServer() can be used for encrypted rpc.
	// * NewUnixClient() and NewUnixServer() can be used for fast local
	//   inter-process rpc.
	//
	// By default it returns TCP connections accepted from Server.RPCAddr.
	Listener Listener

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
	clients         map[string]*Client // client by client addr
}

// NewReverseServer creates a new ReverseServer object
func NewReverseServer(rpc_addr string, http_port int) *ReverseServer {
	return &ReverseServer{
		HttpPort:        http_port,
		RPCAddr:         rpc_addr,
		lock:            &sync.Mutex{},
		clients_by_path: make(map[string][]*Client),
		clients:         make(map[string]*Client),
	}
}

// Serve starts rpc server and blocks until it is stopped.
func (me *ReverseServer) Serve() error {
	if me.LogError == nil {
		me.LogError = errorLogger
	}

	if me.Listener == nil {
		me.Listener = &defaultListener{}
	}
	if err := me.Listener.Init(me.RPCAddr); err != nil {
		err = fmt.Errorf("gorpc.Server: [%s]. Cannot listen to: [%s]", me.RPCAddr, err)
		me.LogError("%s", err)
		return err
	}

	workersCh := make(chan struct{}, 1000)
	go me.serverHandler(workersCh)

	return fasthttp.ListenAndServe(fmt.Sprintf(":%d", me.HttpPort), me.requestHandler)
}

var SLASH = []byte("/")

func (me *ReverseServer) requestHandler(ctx *fasthttp.RequestCtx) {
	firstpath := bytes.Split(ctx.Path(), SLASH)[0]
	host := ctx.Host()

	route := string(append(host, firstpath...))
	count := int(atomic.AddUint64(&me.requestcount, 1))
	me.lock.Lock()
	clients := me.clients_by_path[route]
	if len(clients) == 0 {
		me.lock.Unlock()
		ctx.Response.Header.SetStatusCode(404)
		fmt.Fprintf(ctx, "not found any client %s", ctx.Path())
		return
	}
	client := clients[count%len(clients)]
	me.lock.Unlock()

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
	res, err := client.Call(Request{
		Version:     "0.1",
		Body:        ctx.Request.Body(),
		Method:      ctx.Request.Header.Method(),
		Uri:         ctx.Request.Header.RequestURI(),
		ContentType: ctx.Request.Header.ContentType(),
		UserAgent:   ctx.Request.Header.UserAgent(),
		Cookies:     cookies,
		Headers:     headers,
	})

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
	ctx.Response.Header.SetStatusCode(int(res.StatusCode))
	for k, v := range res.Headers {
		ctx.Response.Header.SetBytesV(k, v)
	}
	ctx.Response.SetBody(res.Body)
}

func (me *ReverseServer) serverHandler(workersCh chan struct{}) {
	for {
		conn, clientAddr, err := me.Listener.Accept()
		if err != nil {
			me.Stats.incAcceptErrors()
			me.LogError("gorpc.Server: [%s]. Cannot accept new connection: [%s]",
				me.RPCAddr, err)
		}
		me.Stats.incAcceptCalls()
		go me.newConnection(conn, clientAddr, workersCh)
	}
}

func (me *ReverseServer) newConnection(conn io.ReadWriteCloser, clientAddr string, workersCh chan struct{}) {
	client := &Client{
		Addr: clientAddr,
		Dial: func(_ string) (io.ReadWriteCloser, error) { return conn, nil },
	}
	client.Start()
	me.clients[clientAddr] = client

	response, err := client.Call(Request{Uri: []byte("_status")})
	if err != nil {
		me.LogError("gorpc.Server: [%s]. Error [%s]", me.RPCAddr, err)
		conn.Close()
	}

	status := &StatusResponse{}
	if err := json.Unmarshal(response.Body, status); err != nil {
		me.LogError("gorpc.Server: [%s]. unable to get status [%s]", me.RPCAddr, err)
		conn.Close()
	}

	me.lock.Lock()
	for _, p := range status.GetPaths() {
		clients := me.clients_by_path[p]
		me.clients_by_path[p] = append(clients, client)
	}
	me.lock.Unlock()
}
