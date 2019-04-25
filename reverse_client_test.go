package gorpc

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestReverseClient(t *testing.T) {

	client := ReverseClient{ServerAddrs: []string{"127.0.0.1:1992"}}
	client.Start(func(clientAddr string, request Request) Response {
		if string(request.Uri) == "_status" {
			b, _ := json.Marshal(&StatusResponse{Paths: []string{"ukachi.com/van"}})
			println("GOT STATUS", string(b))

			return Response{StatusCode: 200, Body: b}
		}
		fmt.Println("clientAddr", clientAddr, string(request.Method), string(request.Uri))
		return Response{StatusCode: 200, Body: []byte("khoe khong")}
	})
	time.Sleep(100 * time.Hour)
}
