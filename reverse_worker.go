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
				worker.Stop()
				fmt.Println("disconnected from " + addr + " .retry in 2 sec")
				time.Sleep(2 * time.Second)
			}
		}(addr)
	}
	for ; ; time.Sleep(10 * time.Minute) {
	} // just sleep forever
}

type reverseWorker struct {
	serverStopChan chan struct{}
}

// Serve starts rpc server and blocks until it stopped.
func (s *reverseWorker) Serve(proxyaddr string, handler HandlerFunc) {
	if s.serverStopChan != nil {
		panic("gorpc.Server: --- server is already running. Stop it before starting it again")
	}
	s.serverStopChan = make(chan struct{})
	end := make(chan bool)
	client := &Client{Addr: proxyaddr, Dial: defaultDial}

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
func (s *reverseWorker) Stop() {
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
