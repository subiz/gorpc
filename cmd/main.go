package main

import (
	"fmt"
	"flag"
	"github.com/subiz/gorpc"
)

func main() {
	fmt.Println("REVERSE PROXY SERVER v0.5")
	var rpc_addr = *flag.String("rpc", ":5000", "address for the RPC server")
	var http_addr = *flag.String("http", ":80", "address for the Http server")
	flag.Parse()

	server := gorpc.NewReverseServer()
	server.Serve(rpc_addr, http_addr)
}