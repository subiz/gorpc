package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

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

var TEXTPLAIN = []byte("text/plain")
var APPJSON = []byte("application/json")

type Marshaller interface {
	MarshalJSON() ([]byte, error)
}

type Unmarshaller interface {
	UnmarshalJSON(data []byte) error
}

func JSON(res *Response, v interface{}) {
	res.StatusCode = 200
	res.Header["content-type"] = APPJSON

	if mars, ok := v.(Marshaller); ok {
		res.Body, _ = mars.MarshalJSON()
	} else {
		res.Body, _ = json.Marshal(v)
	}
}

func Data(res *Response, contenttype string, data []byte) {
	res.StatusCode = 200
	res.Header["content-type"] = []byte(contenttype)
	res.Body = data
}

func String(res *Response, str string) {
	res.StatusCode = 200
	res.Header["content-type"] = TEXTPLAIN
	res.Body = []byte(str)
}
