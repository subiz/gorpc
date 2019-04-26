package gorpc

import (
	"fmt"
	"io"
	"net"
	"time"
)

type ReverseClient struct {
	ServerAddrs []string // list of all server address
}

func (c *ReverseClient) Start(handler HandlerFunc) {
	for _, addr := range c.ServerAddrs {
		server := &reverseClientServer{ServerAddr: addr}
		go func(addr string) {
			for {
				server.Serve(handler)
				server.Stop()
				fmt.Println("disconnected from " + addr + " .retry in 2 sec")
				time.Sleep(2 * time.Second)
			}
		}(addr)
	}
}

// Server implements RPC server.
//
// Default server settings are optimized for high load, so don't override
// them without valid reason.
type reverseClientServer struct {
	ServerAddr     string
	serverStopChan chan struct{}
}

// Serve starts rpc server and blocks until it stopped.
func (s *reverseClientServer) Serve(handler HandlerFunc) {
	if s.serverStopChan != nil {
		panic("gorpc.Server: --- server is already running. Stop it before starting it again")
	}
	s.serverStopChan = make(chan struct{})
	end := make(chan bool)
	client := &Client{Addr: s.ServerAddr, Dial: defaultDial}

	connC := make(chan io.ReadWriteCloser, 1)
	client.OnConnect = func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
		connC <- conn
		server := &Server{
			Handler:          handler,
			Listener:         newReverseListener(conn, clientAddr),
			Concurrency:      DefaultConcurrency,
			FlushDelay:       DefaultFlushDelay,
			PendingResponses: DefaultPendingMessages,
			SendBufferSize:   DefaultBufferSize,
			RecvBufferSize:   DefaultBufferSize,
		}
		if err := server.Start(); err != nil {
			println("DDDDDDDDD", err.Error())
		}
		<-end
		server.Stop()
		return nil, fmt.Errorf("just exit")
	}
	client.Start()

	conn := <-connC
	go func() {
		for {
			if _, err := conn.Read(make([]byte, 0)); err != nil {
				close(end)
				return
			}
		}
	}()
	select {
	case <-s.serverStopChan:
	case <-end:
	}
	conn.Close()
}

// Stop stops rpc server. Stopped server can be started again.
func (s *reverseClientServer) Stop() {
	close(s.serverStopChan)
	s.serverStopChan = nil
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
