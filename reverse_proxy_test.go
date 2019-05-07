package gorpc

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReverseProxy(t *testing.T) {
	// start the proxy
	go NewReverseProxy(&Config{
		Hosts: []*Host{{
			Domains: []string{"api.com"},
			Paths:   []string{"/4.0/*name"},
		}},
	}).Serve(":11992", ":11995")

	// run the client
	router := NewRouter([]string{"127.0.0.1:11992"}, []string{"api.com"})
	router.GET("/4.0/ping", func(c *Context) { c.String("pong") })
	go router.Run()

	time.Sleep(100 * time.Millisecond)
	code, body := doHTTP("GET", "http://127.0.0.1:11995", H{"host": "api.com"}, nil)
	if code != 404 {
		t.Fatalf("should be 404, got %d, %s", code, body)
	}

	code, body = doHTTP("GET", "http://127.0.0.1:11995/4.0/ping", H{"Host": "api.com"}, nil)
	if code != 200 && body != "pong" {
		t.Fatalf("should be ping, got [%d] %s", code, body)
	}
}

var g_HTTP = &http.Client{Timeout: 60 * time.Second}

type H map[string]string

func doHTTP(method, url string, header H, body []byte) (int, string) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}

	for k, v := range header {
		if strings.ToLower(k) == "host" {
			req.Host = v
		}
		req.Header.Add(k, v)
	}
	res, err := g_HTTP.Do(req)
	if err != nil {
		return -1, err.Error()
	}

	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	return res.StatusCode, string(b)
}

func TestRPClientCrash(t *testing.T) {

}
