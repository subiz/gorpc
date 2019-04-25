package gorpc

import (
	"fmt"
	"io"
	"net"
	"sync"
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
				fmt.Println("disconnected from " + addr + " .retry in 1 sec")
				time.Sleep(1 * time.Second)
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
		panic("gorpc.Server: server is already running. Stop it before starting it again")
	}
	s.serverStopChan = make(chan struct{})
	end := make(chan bool, 0)
	client := &Client{Addr: s.ServerAddr, Dial: defaultDial}
	client.OnConnect = func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
		go func() { // detect connection closed
			for {
				if _, err := conn.Read(make([]byte, 0)); err != nil {
					close(end)
					return
				}
			}
		}()

		server := &Server{
			Handler:          handler,
			Listener:         newReverseListener(conn, clientAddr),
			Concurrency:      1,
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
		conn.Close()
		return nil, fmt.Errorf("just exit")
	}
	client.Start()

	<-s.serverStopChan
	select {
	case end <- true:
	default:
	}
}

// Stop stops rpc server. Stopped server can be started again.
func (s *reverseClientServer) Stop() {
	close(s.serverStopChan)
	s.serverStopChan = nil
}

type reverseListener struct {
	sync.Mutex
	clientaddr string
	conn       io.ReadWriteCloser
	L          net.Listener
}

func newReverseListener(conn io.ReadWriteCloser, clientaddr string) *reverseListener {
	return &reverseListener{conn: conn, clientaddr: clientaddr}
}

func (ln *reverseListener) Init(addr string) (err error) { return nil }

func (ln *reverseListener) ListenAddr() net.Addr { return nil }

func (ln *reverseListener) Accept() (conn io.ReadWriteCloser, clientAddr string, err error) {
	ln.Lock()
	if ln.conn == nil {
		println("SECOND CALLED")

		ln.Lock() // block here

		ln.Unlock()
		return nil, "", fmt.Errorf("CLOSED")
	}
	conn = ln.conn
	ln.conn = nil
	ln.Unlock()
	return conn, ln.clientaddr, err
}

func (ln *reverseListener) Close() error {
	println("CLOSED")
	ln.Unlock()
	return nil
}
