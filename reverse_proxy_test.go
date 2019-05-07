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
	go NewReverseProxy().Serve(":11991", ":11994")
	time.Sleep(100 * time.Millisecond)
	// run the client
	router := NewRouter([]string{"127.0.0.1:11991"}, []string{"api.com"})
	router.GET("/4.0/ping", func(c *Context) { c.String("pong") })
	go router.Run()

	time.Sleep(100 * time.Millisecond)
	code, body := doHTTP("GET", "http://127.0.0.1:11994", H{"host": "api.com"}, nil)
	if code != 503 {
		t.Fatalf("should be 503, got %d, %s", code, body)
	}

	code, body = doHTTP("GET", "http://127.0.0.1:11994/4.0/ping", H{"Host": "api.com"}, nil)
	if code != 200 || body != "pong" {
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

func TestProxyWorkerCrash(t *testing.T) {
	p := NewReverseProxy()
	p.Log = errorLogger
	go p.Serve(":11992", ":11995")

	time.Sleep(100 * time.Millisecond)
	// run the client
	router := NewRouter([]string{"127.0.0.1:11992"}, []string{"api.com"})
	router.GET("/4.0/ping", func(c *Context) { c.String("pong") })
	go router.Run()

	time.Sleep(100 * time.Millisecond)
	code, body := doHTTP("GET", "http://127.0.0.1:11995/4.0/ping", H{"Host": "api.com"}, nil)
	if code != 200 || body != "pong" {
		t.Errorf("should be ping, got [%d] %s", code, body)
		time.Sleep(10 * time.Second)
	}

	router.Stop()
	code, body = doHTTP("GET", "http://127.0.0.1:11995/4.0/ping", H{"Host": "api.com"}, nil)
	if code != 502 && code != 503 { // worker error
		t.Fatalf("should be 502 or 503, got [%d] %s", code, body)
	}
	time.Sleep(100 * time.Millisecond)
	code, body = doHTTP("GET", "http://127.0.0.1:11995/4.0/ping", H{"Host": "api.com"}, nil)
	if code != 503 { // no worker found
		t.Fatalf("should be 503, got [%d] %s", code, body)
	}
}

func TestMultipleWorkers(t *testing.T) {
	go NewReverseProxy().Serve(":11993", ":11996")
	time.Sleep(100 * time.Millisecond)

	// run the client 1
	router1 := NewRouter([]string{"127.0.0.1:11993"}, []string{"api.com"})
	router1.GET("/4.0/ping", func(c *Context) { c.String("pong 1") })
	go router1.Run()

	// run the client 2
	router2 := NewRouter([]string{"127.0.0.1:11993"}, []string{"api.com"})
	router2.GET("/4.0/ping", func(c *Context) { c.String("pong 2") })
	go router2.Run()

	// run the client 3
	router3 := NewRouter([]string{"127.0.0.1:11993"}, []string{"api.com"})
	router3.GET("/4.0/ping", func(c *Context) { c.String("pong 3") })
	go router3.Run()

	counter1, counter2, counter3 := 0, 0, 0
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 1000; i++ {
		code, body := doHTTP("GET", "http://127.0.0.1:11996/4.0/ping", H{"Host": "api.com"}, nil)
		if code != 200 {
			t.Fatalf("should be ping, got [%d] %s", code, body)
		}

		if body == "pong 1" {
			counter1++
		} else if body == "pong 2" {
			counter2++
		} else if body == "pong 3" {
			counter3++
		}
	}

	if abs(counter3-counter2) > 10 || abs(counter2-counter1) > 10 {
		t.Fatalf("NOT EVEN")
	}
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
