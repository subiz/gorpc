package gorpc

import (
	"fmt"
	"io"
	"net"
)

type reverseWorker struct {
	stop      chan bool
	handler   HandlerFunc
	proxyaddr string
}

func (s *reverseWorker) Stop() {
	close(s.stop)
	s.stop = nil
}

// proxyConnection represents a connection from a proxy to a worker
type proxyConnection struct {
	clientAddr string
	conn       io.ReadWriteCloser
}

func newReverseWorker(proxyaddr string, handler HandlerFunc) *reverseWorker {
	return &reverseWorker{proxyaddr: proxyaddr, handler: handler, stop: make(chan bool)}
}

// Serve starts rpc server and blocks until it stopped.
func (s *reverseWorker) Run() {
	if s.stop == nil {
		return
	}
	connC := make(chan proxyConnection, 1)
	end := make(chan bool)
	client := &Client{Addr: s.proxyaddr, Dial: defaultDial}
	client.OnConnect = func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
		connC <- proxyConnection{clientAddr: clientAddr, conn: conn}
		<-end
		return nil, fmt.Errorf("just exit")
	}
	client.Start()

	// wait until connection to the proxy is established
	var conn proxyConnection
	select {
	case conn = <-connC:
	case <-s.stop:
		client.OnConnect = nil // make sure we only call OnConnect once
		close(end)
		client.Stop()
		return
	}
	client.OnConnect = nil // make sure we only call OnConnect once

	server := &Server{
		Handler:     s.handler,
		Listener:    newReverseListener(conn.conn, conn.clientAddr),
		Concurrency: DefaultConcurrency,
	}
	if err := server.Start(); err != nil {
		fmt.Println("Server ERR", err.Error())
	}

	go func() {
		var err error
		for dump := make([]byte, 0); err == nil; _, err = conn.conn.Read(dump) {
		}
		close(end)
	}()

	select {
	case <-end:
	case <-s.stop:
	}
	conn.conn.Close()
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
