package gorpc

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type ReverseClient struct {
	serverAddrs []string // list of all server address
	handler     HandlerFunc
}

func NewReverseClient(serverAddrs []string, handler HandlerFunc) *ReverseClient {
	return &ReverseClient{serverAddrs: serverAddrs, handler: handler}
}

func (c *ReverseClient) Start() {
	for _, addr := range c.serverAddrs {
		go (&reverseClientServer{ServerAddr: addr, Handler: c.handler}).Serve()
	}
}

// Server implements RPC server.
//
// Default server settings are optimized for high load, so don't override
// them without valid reason.
type reverseClientServer struct {
	server *Server
	ServerAddr string
	Handler HandlerFunc
	serverStopChan chan struct{}
	client *Client
}

func (s *reverseClientServer) Serve() {
	if s.Handler == nil {
		panic("gorpc.Server: Server.Handler cannot be nil")
	}

	if s.serverStopChan != nil {
		panic("gorpc.Server: server is already running. Stop it before starting it again")
	}
	s.serverStopChan = make(chan struct{})

	workersCh := make(chan struct{}, 1)
	s.serverHandler(workersCh)
}

// Stop stops rpc server. Stopped server can be started again.
func (s *reverseClientServer) Stop() {
	s.server.Stop()
	close(s.serverStopChan)
	s.serverStopChan = nil
}

func (s *reverseClientServer) serverHandler(workersCh chan struct{}) {
	stopChan := make(chan struct{})
	readerDone := make(chan struct{})
	writerDone := make(chan struct{})
	end := make(chan bool, 0)

	s.client = &Client{
		Addr: s.ServerAddr,
		Dial: defaultDial,
		OnConnect: func(clientAddr string, conn io.ReadWriteCloser) (io.ReadWriteCloser, error) {
			s.server = &Server{
				Addr:             "",
				Handler:          s.Handler,
				Listener:         newReverseListener(conn, clientAddr),
				Concurrency:      DefaultConcurrency,
				FlushDelay:       DefaultFlushDelay,
				PendingResponses: DefaultPendingMessages,
				SendBufferSize:   DefaultBufferSize,
				RecvBufferSize:   DefaultBufferSize,
			}

			println("START---------")
			if err := s.server.Start(); err != nil {
				println("DDDDDDDDD", err.Error())
			}
			println("HERE")
			<-end
			conn.Close()
			return nil, fmt.Errorf("just exit")
		},
	}
	s.client.Start()

	defer func() {
		select {
		case end <- true:
		default:
		}
	}()

	select {
	case <-readerDone:
		close(stopChan)
		<-writerDone
	case <-writerDone:
		close(stopChan)
		<-readerDone
	case <-s.serverStopChan:
		close(stopChan)
		<-readerDone
		<-writerDone
	}
	println("EXITED")
}

type reverseListener struct {
	*sync.Mutex
	clientaddr string
	conn       io.ReadWriteCloser
	L          net.Listener
}

func newReverseListener(conn io.ReadWriteCloser, clientaddr string) *reverseListener {
	return &reverseListener{Mutex: &sync.Mutex{}, conn: conn, clientaddr: clientaddr}
}

func (ln *reverseListener) Init(addr string) (err error) { return nil }

func (ln *reverseListener) ListenAddr() net.Addr { return nil }

func (ln *reverseListener) Accept() (conn io.ReadWriteCloser, clientAddr string, err error) {
	ln.Lock()
	if ln.conn == nil {
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
	ln.Unlock()
	return nil
}
