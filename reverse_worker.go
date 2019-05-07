package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type reverseWorker struct{}

// proxyConnection represents a connection from a proxy to a worker
type proxyConnection struct {
	clientAddr string
	conn       io.ReadWriteCloser
}

// Serve starts rpc server and blocks until it stopped.
func (s *reverseWorker) Serve(proxyaddr string, handler HandlerFunc) {
	end := make(chan bool)
	connC := make(chan proxyConnection, 1)

	client := &Client{Addr: proxyaddr, Dial: defaultDial}
	client.OnConnect = func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
		connC <- proxyConnection{clientAddr: clientAddr, conn: conn}
		<-end
		return nil, fmt.Errorf("just exit")
	}
	client.Start()

	// wait until connection to the proxy is established
	conn := <-connC
	client.OnConnect = nil // make sure we only call OnConnect once

	go func() {
		var err error
		for dump := make([]byte, 0); err == nil; _, err = conn.conn.Read(dump) {
		}
		close(end)
	}()

	server := &Server{
		Handler:     handler,
		Listener:    newReverseListener(conn.conn, conn.clientAddr),
		Concurrency: DefaultConcurrency,
	}
	if err := server.Start(); err != nil {
		fmt.Println("Server ERR", err.Error())
	}
	<-end
	server.Stop()
	client.Stop()
}

type reverseListener struct {
	lock       chan bool
	clientaddr string
	conn       io.ReadWriteCloser
	L          net.Listener
}

func newReverseListener(conn io.ReadWriteCloser, clientaddr string) *reverseListener {
	return &reverseListener{lock: make(chan bool, 1), conn: conn, clientaddr: clientaddr}
}

func (ln *reverseListener) Init(addr string) (err error) { return nil }

func (ln *reverseListener) ListenAddr() net.Addr { return nil }

func (ln *reverseListener) Accept() (conn io.ReadWriteCloser, clientAddr string, err error) {
	defer func() {
		if r := recover(); r != nil {
			clientAddr, conn = "", nil
			err = fmt.Errorf("CLOSED")
			return
		}
	}()
	ln.lock <- true
	if ln.conn == nil {
		return nil, "", fmt.Errorf("CLOSED")
	}
	conn = ln.conn
	ln.conn = nil
	return conn, ln.clientaddr, err
}

func (ln *reverseListener) Close() error {
	defer func() { recover() }()
	lock := ln.lock
	ln.lock = make(chan bool, 1)
	close(lock)
	return nil
}

type Handler func(Request, Params, map[string]string, *Response) bool

// Router used by a reverse worker to bind handler to path
type Router struct {
	get_root, post_root, del_root *node
	def                           Handler
	proxy_addrs, domains, paths   []string
}

func NewRouter(proxy_addrs, domains, paths []string) *Router {
	return &Router{
		proxy_addrs: proxy_addrs,
		domains:     domains,
		paths:       paths,
		get_root:    &node{},
		post_root:   &node{},
		del_root:    &node{},
		def: func(req Request, _ Params, _ map[string]string, res *Response) bool {
			res.StatusCode = 400
			res.Body = []byte("dashboard not found :(( " + req.Path + ".")
			return true
		},
	}
}

var STATUS = []byte("_status")

// Start makes connection to and starts waiting request from the proxy
func (me *Router) Run() {
	for _, addr := range me.proxy_addrs {
		go func(addr string) {
			worker := &reverseWorker{}
			for {
				worker.Serve(addr, func(clientAddr string, request Request) Response {
					if bytes.Compare(request.Uri, STATUS) == 0 {
						b, _ := json.Marshal(&StatusResponse{
							Paths:   me.paths,
							Domains: me.domains,
						})
						return Response{StatusCode: 200, Body: b}
					}
					return me.Handle(request)
				})
				fmt.Println("disconnected from " + addr + " .retry in 2 sec")
				time.Sleep(2 * time.Second)
			}
		}(addr)
	}
	for ; ; time.Sleep(10 * time.Minute) {
	} // just sleep forever
}

func (me *Router) Handle(req Request) Response {
	res := Response{}
	res.Header = make(map[string][]byte)

	var root *node
	switch req.Method {
	case "GET":
		root = me.get_root
	case "POST":
		root = me.post_root
	case "DELETE":
		root = me.del_root
	default:
		me.def(req, nil, make(map[string]string), &res)
		return res
	}

	h, ps, _ := root.getValue(req.Path)
	handler, _ := h.(Handler)
	if handler == nil {
		me.def(req, nil, make(map[string]string), &res)
		return res
	}

	handler(req, ps, make(map[string]string), &res)
	return res
}

func (me *Router) NoRoute(handlers ...Handler) {
	me.def = me.wraps(handlers...)
}

func (me *Router) GET(path string, handles ...Handler) {
	me.get_root.addRoute(path, me.wraps(handles...))
}

func (me *Router) Any(path string, handles ...Handler) {
	me.POST(path, handles...)
	me.GET(path, handles...)
	me.DEL(path, handles...)
}

func (me *Router) POST(path string, handles ...Handler) {
	me.post_root.addRoute(path, me.wraps(handles...))
}

func (me *Router) DEL(path string, handles ...Handler) {
	me.del_root.addRoute(path, me.wraps(handles...))
}

func (me *Router) wraps(handles ...Handler) Handler {
	return func(req Request, ps Params, m map[string]string, res *Response) bool {
		for _, h := range handles {
			cont := h(req, ps, m, res)
			if !cont {
				return false
			}
		}
		return true
	}
}
