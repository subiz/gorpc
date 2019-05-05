package gorpc

import (
	"testing"
)

func TestReverseServer(t *testing.T) {
	server := NewReverseProxy(&Config{
		Hosts: []*Host{},
	})
	server.Serve(":1992", ":1995")
}
