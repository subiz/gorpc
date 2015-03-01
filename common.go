package gorpc

import (
	"encoding/gob"
	"io"
	"log"
	"sync/atomic"
)

// Error logging function to pass to gorpc.SetErrorLogger().
type LoggerFunc func(format string, args ...interface{})

var errorLogger = LoggerFunc(log.Printf)

// Use the given error logger in gorpc.
//
// By default log.Printf is used for error logging.
func SetErrorLogger(f LoggerFunc) {
	errorLogger = f
}

// Registers the given type to send via rpc.
//
// The client must register all the response types the server may send.
// The server must register all the request types the client may send.
//
// There is no need in registering base Go types such as int, string, bool,
// float64, etc. or arrays, slices and maps containing base Go types.
func RegisterType(x interface{}) {
	gob.Register(x)
}

type wireMessage struct {
	ID   uint64
	Data interface{}
}

func logError(format string, args ...interface{}) {
	errorLogger(format, args...)
}

// Connection statistics. Applied to both gorpc.Client and gorpc.Server.
type ConnStats struct {
	// The number of bytes written to the underlying connections.
	BytesWritten uint64

	// The number of bytes read from the underlying connections.
	BytesRead uint64

	// The number of Read() calls.
	ReadCalls uint64

	// The number of Write() calls.
	WriteCalls uint64

	// The number of Dial() calls.
	DialCalls uint64

	// The number of Accept() calls.
	AcceptCalls uint64
}

type writerCounter struct {
	W            io.Writer
	BytesWritten *uint64
	WriteCalls   *uint64
}

func (w *writerCounter) Write(p []byte) (int, error) {
	n, err := w.W.Write(p)
	atomic.AddUint64(w.WriteCalls, 1)
	atomic.AddUint64(w.BytesWritten, uint64(n))
	return n, err
}

type readerCounter struct {
	R         io.Reader
	BytesRead *uint64
	ReadCalls *uint64
}

func (r *readerCounter) Read(p []byte) (int, error) {
	n, err := r.R.Read(p)
	atomic.AddUint64(r.ReadCalls, 1)
	atomic.AddUint64(r.BytesRead, uint64(n))
	return n, err
}
