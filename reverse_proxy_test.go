package gorpc

import (
	"testing"
)

func TestReverseProxy(t *testing.T) {
	proxy := NewReverseProxy(&Config{
		Hosts: []*Host{{
			Domains: []string{"api.com"},
			Paths: []string{"/4.0/*name"},
		}},
	})
	proxy.Serve(":1992", ":1995")

	client := ReverseClient{ServerAddrs: []string{"127.0.0.1:1992"}}
	count := uint64(0)
	client.Start(func(clientAddr string, request Request) Response {
		if string(request.Uri) == "_status" {
			b, _ := json.Marshal(&StatusResponse{Paths: []string{"localhost:1995/hello"}})
			println("GOT STATUS", string(b))

			return Response{StatusCode: 200, Body: b}
		}

		c := atomic.AddUint64(&count, 1)
		fmt.Println(string(request.Method), string(request.Uri), "count", c)
		return Response{StatusCode: 200, Body: []byte("khoe khong--" + time.Now().String())}
	})

}

func TestRPClientCrash(t *testing.T) {

}
