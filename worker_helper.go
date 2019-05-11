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
	stop                          chan bool
}

func NewRouter(proxy_addrs, domains []string) *Router {
	return &Router{
		proxy_addrs: proxy_addrs,
		domains:     domains,
		get_root:    &node{},
		post_root:   &node{},
		del_root:    &node{},
		def: func(c *Context) {
			c.String(400, "dashboard not found :(( "+c.request.Path+".")
		},
		stop: make(chan bool),
	}
}

var STATUS = []byte("_status")

func (me *Router) Stop() {
	close(me.stop)
}

// Start makes connection to and starts waiting request from the proxy
func (me *Router) Run() {
	is_stopped := false
	workers := make([]*reverseWorker, 0)
	for _, addr := range me.proxy_addrs {
		worker := newReverseWorker(addr, func(clientAddr string, request Request) Response {
			if bytes.Compare(request.Uri, STATUS) == 0 {
				b, _ := json.Marshal(&StatusResponse{
					Paths:   me.paths,
					Domains: me.domains,
				})
				return Response{StatusCode: 200, Body: b}
			}
			return me.Handle(request)
		})

		workers = append(workers, worker)
		go func(addr string) {
			for !is_stopped {
				worker.Run()
				fmt.Println("disconnected from " + addr + " .retry in 2 sec")
				time.Sleep(2 * time.Second)
			}
		}(addr)
	}
	select {
	case <-me.stop:
	}
	for _, worker := range workers {
		worker.Stop()
	}
	is_stopped = true
}

func (me *Router) Handle(req Request) (res Response) {
	defer func() {
		if r := recover(); r != nil {
			res = Response{
				StatusCode: 500,
				Header:     map[string][]byte{"content-type": TEXTPLAIN},
				Body:       []byte(fmt.Sprintf("ROUTING ERR: %v", r)),
			}
		}
	}()
	var root *node
	switch req.Method {
	case "GET":
		root = me.get_root
	case "POST":
		root = me.post_root
	case "DELETE":
		root = me.del_root
	}

	if root == nil {
		me.def(&Context{request: req, params: Params{}, response: &res})
		return res
	}
	h, ps, _ := root.getValue(req.Path)
	handler, _ := h.(Handler)
	if handler == nil {
		me.def(&Context{request: req, params: ps, response: &res})
		return res
	}

	handler(&Context{request: req, params: ps, response: &res})
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
			if c.aborted {
				break
			}
		}
	}
}

var TEXTPLAIN = []byte("text/plain")
var APPJSON = []byte("application/json")
var JAVASCRIPT = []byte("text/javascript")
var TEXTHTML = []byte("text/html")

type Marshaller interface {
	MarshalJSON() ([]byte, error)
}

type Unmarshaller interface {
	UnmarshalJSON(data []byte) error
}

type Context struct {
	request  Request
	params   Params
	store    map[string]string
	response *Response
	aborted  bool
}

func (c *Context) Html(code int, html []byte) {
	c.response.StatusCode = int32(code)
	c.SetHeader("content-type", TEXTHTML)
	c.response.Body = html
}

func (c *Context) JSON(code int, v interface{}) {
	c.response.StatusCode = int32(code)
	c.SetHeader("content-type", APPJSON)

	if mars, ok := v.(Marshaller); ok {
		c.response.Body, _ = mars.MarshalJSON()
		return
	}
	c.response.Body, _ = json.Marshal(v)
}

func (c *Context) Data(code int, contenttype string, data []byte) {
	c.response.StatusCode = int32(code)

	var ct []byte = nil
	switch contenttype {
	case "text/plain":
		ct = TEXTPLAIN
	case "application/json":
		ct = APPJSON
	case "text/javascript":
		ct = JAVASCRIPT
	case "text/html":
		ct = TEXTHTML
	default:
		ct = []byte(contenttype)
	}

	if len(ct) > 0 {
		c.SetHeader("content-type", ct)
	}
	c.response.Body = data
}

func (c *Context) String(code int, str string) {
	c.response.StatusCode = int32(code)
	c.SetHeader("content-type", TEXTPLAIN)
	c.response.Body = []byte(str)
}

func (c *Context) Abort() { c.aborted = true }

func (c *Context) SetString(key, val string) {
	if c.store == nil {
		c.store = make(map[string]string)
	}
	c.store[key] = val
}

func (c *Context) GetString(key string) string {
	if c.store == nil {
		return ""
	}
	return c.store[key]
}

func (c *Context) Status(statuscode int) {
	c.response.StatusCode = int32(statuscode)
}

func (c *Context) Params(name string) string {
	if c.params == nil {
		return ""
	}

	return c.params[name]
}

func (c *Context) PostForm(name string) string {
	if c.request.Form == nil {
		return ""
	}

	return c.request.Form[name]
}

func (c *Context) Query(name string) string {
	if c.request.Query == nil {
		return ""
	}

	return c.request.Query[name]
}

// Cookie lookups cookie by key from the request
func (c *Context) Cookie(key string) string {
	if c.request.Cookie == nil {
		return ""
	}

	return string(c.request.Cookie[key])
}

// AddCookie add cookie header to response
func (c *Context) AddCookie(cook *Cookie) {
	c.response.Cookies = append(c.response.Cookies, cook)
}

// SetHeader adds a header to response, override last header with same key
func (c *Context) SetHeader(key string, val []byte) {
	if c.response.Header == nil {
		c.response.Header = make(map[string][]byte)
	}
	c.response.Header[key] = val
}

// SetHeader adds a header to response, override last header with same key
func (c *Context) Header(key string) []byte {
	if c.request.Header == nil {
		return nil
	}
	return c.request.Header[key]
}

func (c *Context) VisitHeader(f func(key string, val []byte)) {
	for k, v := range c.request.Header {
		f(k, v)
	}
}

func (c *Context) Method() string { return c.request.Method }

func (c *Context) Uri() string { return string(c.request.Uri) }

func (c *Context) Body() []byte { return c.request.Body }

func (c *Context) UserAgent() []byte { return c.request.UserAgent }

func (c *Context) RemoteAddr() string { return c.request.RemoteAddr }

func (c *Context) Referer() string { return c.request.Referer }

func (c *Context) Path() string { return c.request.Path }

func (c *Context) Host() string { return c.request.Host }

func (c *Context) RawQuery() map[string]string { return c.request.Query }

type H map[string]interface{}
