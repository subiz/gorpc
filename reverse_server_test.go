package gorpc

import (
	"testing"
)

func TestReverseServer(t *testing.T) {
	server := NewReverseServer()
	server.Serve(":1992", ":1995")
}
