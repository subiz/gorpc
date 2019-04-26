package gorpc

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestReverseClient(t *testing.T) {

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
	time.Sleep(100 * time.Hour)
}
