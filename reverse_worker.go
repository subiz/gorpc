package gorpc

import (
	"fmt"
	"io"
	"net"
	"time"
)

// Start makes connection to and starts waiting request from the proxy
func StartReverseWorker(proxy_addrs []string, handler HandlerFunc) {
	for _, addr := range proxy_addrs {
		go func(addr string) {
			worker := &reverseWorker{}
			for {
				worker.Serve(addr, handler)
				fmt.Println("disconnected from " + addr + " .retry in 2 sec")
				time.Sleep(2 * time.Second)
			}
		}(addr)
	}
	for ; ; time.Sleep(10 * time.Minute) {
	} // just sleep forever
}

type reverseWorker struct{}

// proxyConnection represents a connection from a proxy to a worker
type proxyConnection struct {
	clientAddr string
	conn       io.ReadWriteCloser
}

// Serve starts rpc server and blocks until it stopped.
func (s *reverseWorker) Serve(proxyaddr string, handler HandlerFunc) {
	end := make(chan bool)
	connC := make(chan proxyConnection, 1)

	client := &Client{Addr: proxyaddr, Dial: defaultDial}
	client.OnConnect = func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
		connC <- proxyConnection{clientAddr: clientAddr, conn: conn}
		<-end
		return nil, fmt.Errorf("just exit")
	}
	client.Start()

	// wait until connection to the proxy is established
	conn := <-connC
	client.OnConnect = nil // make sure we only call OnConnect once

	go func() {
		var err error
		for dump := make([]byte, 0); err == nil; _, err = conn.conn.Read(dump) {
		}
		close(end)
	}()

	server := &Server{
		Handler:     handler,
		Listener:    newReverseListener(conn.conn, conn.clientAddr),
		Concurrency: DefaultConcurrency,
	}
	if err := server.Start(); err != nil {
		fmt.Println("Server ERR", err.Error())
	}
	<-end
	server.Stop()
	client.Stop()
}

type reverseListener struct {
	lock       chan bool
	clientaddr string
	conn       io.ReadWriteCloser
	L          net.Listener
}

func newReverseListener(conn io.ReadWriteCloser, clientaddr string) *reverseListener {
	return &reverseListener{lock: make(chan bool, 1), conn: conn, clientaddr: clientaddr}
}

func (ln *reverseListener) Init(addr string) (err error) { return nil }

func (ln *reverseListener) ListenAddr() net.Addr { return nil }

func (ln *reverseListener) Accept() (conn io.ReadWriteCloser, clientAddr string, err error) {
	defer func() {
		if r := recover(); r != nil {
			clientAddr, conn = "", nil
			err = fmt.Errorf("CLOSED")
			return
		}
	}()
	ln.lock <- true
	if ln.conn == nil {
		return nil, "", fmt.Errorf("CLOSED")
	}
	conn = ln.conn
	ln.conn = nil
	return conn, ln.clientaddr, err
}

func (ln *reverseListener) Close() error {
	defer func() { recover() }()
	lock := ln.lock
	ln.lock = make(chan bool, 1)
	close(lock)
	return nil
}
