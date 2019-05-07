package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type Handler func(context *Context)

// Router used by a reverse worker to bind handler to path
type Router struct {
	get_root, post_root, del_root *node
	def                           Handler
	proxy_addrs, domains, paths   []string
}

func NewRouter(proxy_addrs, domains []string) *Router {
	return &Router{
		proxy_addrs: proxy_addrs,
		domains:     domains,
		get_root:    &node{},
		post_root:   &node{},
		del_root:    &node{},
		def: func(c *Context) {
			c.SetCode(400)
			c.String("dashboard not found :(( " + c.Request.Path + ".")
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
	var root *node
	switch req.Method {
	case "GET":
		root = me.get_root
	case "POST":
		root = me.post_root
	case "DELETE":
		root = me.del_root
	}

	res := Response{}
	res.Header = make(map[string][]byte)
	if root == nil {
		me.def(&Context{Request: req, Response: &res})
		return res
	}
	h, ps, _ := root.getValue(req.Path)
	handler, _ := h.(Handler)
	if handler == nil {
		me.def(&Context{Request: req, Response: &res})
		return res
	}

	handler(&Context{Request: req, Params: ps, Store: make(map[string]string), Response: &res})
	return res
}

func (me *Router) NoRoute(handlers ...Handler) {
	me.def = me.wraps(handlers...)
	me.paths = append(me.paths, "")
}

func (me *Router) GET(path string, handles ...Handler) {
	me.get_root.addRoute(path, me.wraps(handles...))
	me.paths = append(me.paths, path)
}

func (me *Router) Any(path string, handles ...Handler) {
	me.get_root.addRoute(path, me.wraps(handles...))
	me.post_root.addRoute(path, me.wraps(handles...))
	me.del_root.addRoute(path, me.wraps(handles...))
	me.paths = append(me.paths, path)
}

func (me *Router) POST(path string, handles ...Handler) {
	me.post_root.addRoute(path, me.wraps(handles...))
	me.paths = append(me.paths, path)
}

func (me *Router) DEL(path string, handles ...Handler) {
	me.del_root.addRoute(path, me.wraps(handles...))
	me.paths = append(me.paths, path)
}

func (me *Router) wraps(handles ...Handler) Handler {
	return func(c *Context) {
		for _, h := range handles {
			h(c)
			if c.Aborted {
				break
			}
		}
	}
}

var TEXTPLAIN = []byte("text/plain")
var APPJSON = []byte("application/json")

type Marshaller interface {
	MarshalJSON() ([]byte, error)
}

type Unmarshaller interface {
	UnmarshalJSON(data []byte) error
}

type Context struct {
	Request  Request
	Params   Params
	Store    map[string]string
	Response *Response
	Aborted  bool
}

func (c *Context) JSON(v interface{}) {
	if c.Response.StatusCode == 0 {
		c.Response.StatusCode = 200
	}
	c.Response.Header["content-type"] = APPJSON

	if mars, ok := v.(Marshaller); ok {
		c.Response.Body, _ = mars.MarshalJSON()
	} else {
		c.Response.Body, _ = json.Marshal(v)
	}
}

func (c *Context) Data(contenttype string, data []byte) {
	if c.Response.StatusCode == 0 {
		c.Response.StatusCode = 200
	}
	c.Response.Header["content-type"] = []byte(contenttype)
	c.Response.Body = data
}

func (c *Context) String(str string) {
	if c.Response.StatusCode == 0 {
		c.Response.StatusCode = 200
	}
	c.Response.Header["content-type"] = TEXTPLAIN
	c.Response.Body = []byte(str)
}

func (c *Context) Abort() {
	c.Aborted = true
}

func (c *Context) SetCode(statuscode int) {
	c.Response.StatusCode = int32(statuscode)
}
