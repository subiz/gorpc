package gorpc

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type ReverseServer struct {
	// Address to listen to for incoming connections.
	Addr string

	// OnConnect is called whenever connection from client is accepted.
	// The callback can be used for authentication/authorization/encryption
	// and/or for custom transport wrapping.
	//
	// See also Listener, which can be used for sophisticated transport
	// implementation.
	OnConnect OnConnectFunc

	// The server obtains new client connections via Listener.Accept().
	//
	// Override the listener if you want custom underlying transport
	// and/or client authentication/authorization.
	// Don't forget overriding Client.Dial() callback accordingly.
	//
	// See also OnConnect for authentication/authorization purposes.
	//
	// * NewTLSClient() and NewTLSServer() can be used for encrypted rpc.
	// * NewUnixClient() and NewUnixServer() can be used for fast local
	//   inter-process rpc.
	//
	// By default it returns TCP connections accepted from Server.Addr.
	Listener Listener

	// LogError is used for error logging.
	//
	// By default the function set via SetErrorLogger() is used.
	LogError LoggerFunc

	// Connection statistics.
	//
	// The stats doesn't reset automatically. Feel free resetting it
	// any time you wish.
	Stats ConnStats

	serverStopChan chan struct{}
	stopWg         sync.WaitGroup

	clients_by_path map[string][]*Client
	clients         map[string]*Client // client by client addr
}

// Stop stops rpc server. Stopped server can be started again.
func (me *ReverseServer) Stop() {
	if me.serverStopChan == nil {
		panic("gorpc.Server: server must be started before stopping it")
	}
	close(me.serverStopChan)
	me.stopWg.Wait()
	me.serverStopChan = nil
}

// Serve starts rpc server and blocks until it is stopped.
func (me *ReverseServer) Serve() error {

	if me.LogError == nil {
		me.LogError = errorLogger
	}

	if me.serverStopChan != nil {
		panic("gorpc.Server: server is already running. Stop it before starting it again")
	}
	me.serverStopChan = make(chan struct{})

	if me.Listener == nil {
		me.Listener = &defaultListener{}
	}
	if err := me.Listener.Init(me.Addr); err != nil {
		err = fmt.Errorf("gorpc.Server: [%s]. Cannot listen to: [%s]", me.Addr, err)
		me.LogError("%s", err)
		return err
	}

	// workersCh := make(chan struct{}, s.Concurrency)
	me.stopWg.Add(1)
	workersCh := make(chan struct{}, 1000)
	go me.serverHandler(workersCh)
	me.stopWg.Wait()
	return nil
}

func (me *ReverseServer) serverHandler(workersCh chan struct{}) {
	defer me.stopWg.Done()

	var conn io.ReadWriteCloser
	var clientAddr string
	var err error
	var stopping atomic.Value

	for {
		acceptChan := make(chan struct{})
		go func() {
			if conn, clientAddr, err = me.Listener.Accept(); err != nil {
				if stopping.Load() == nil {
					me.LogError("gorpc.Server: [%s]. Cannot accept new connection: [%s]",
						me.Addr, err)
				}
			}
			close(acceptChan)
		}()

		select {
		case <-me.serverStopChan:
			stopping.Store(true)
			me.Listener.Close()
			<-acceptChan
			return
		case <-acceptChan:
			me.Stats.incAcceptCalls()
		}
		if err != nil {
			me.Stats.incAcceptErrors()
			select {
			case <-me.serverStopChan:
				return
			case <-time.After(time.Second):
			}
			continue
		}

		me.stopWg.Add(1)
		go me.newConnection(conn, clientAddr, workersCh)
	}
}

func (me *ReverseServer) newConnection(conn io.ReadWriteCloser, clientAddr string, workersCh chan struct{}) {
	defer me.stopWg.Done()

	// var err error
	// var stopping atomic.Value

	client := &Client{
		Addr: clientAddr,
		Dial: func(_ string) (io.ReadWriteCloser, error) { return conn, nil },
	}
	client.Start()
	//	clients_by_path
	me.clients[clientAddr] = client

	// client.Call()

}
