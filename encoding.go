package gorpc

import (
	"bufio"
	"compress/flate"
	"github.com/golang/protobuf/proto"
	"io"
)

type messageEncoder struct {
	w  *bufio.Writer
	bw *bufio.Writer
	zw *flate.Writer
	ww *bufio.Writer
}

func (e *messageEncoder) Close() error {
	if e.zw != nil {
		return e.zw.Close()
	}
	return nil
}

func (e *messageEncoder) Flush() error {
	if e.zw != nil {
		if err := e.ww.Flush(); err != nil {
			return err
		}
		if err := e.zw.Flush(); err != nil {
			return err
		}
	}
	if err := e.bw.Flush(); err != nil {
		return err
	}
	return nil
}

func (e *messageEncoder) Encode(msg *Request) error {
	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = e.w.Write(b)
	return err
}

func newMessageEncoder(w io.Writer, bufferSize int, enableCompression bool, s *ConnStats) *messageEncoder {
	w = newWriterCounter(w, s)
	bw := bufio.NewWriterSize(w, bufferSize)
	var mainwriter = bw

	ww := bw
	var zw *flate.Writer
	if enableCompression {
		zw, _ = flate.NewWriter(bw, flate.BestSpeed)
		ww = bufio.NewWriterSize(zw, bufferSize)
		mainwriter = ww
	}

	return &messageEncoder{w: mainwriter, bw: bw, zw: zw, ww: ww}
}

type messageDecoder struct {
	buffer *proto.Buffer
	zr io.ReadCloser
}

func (d *messageDecoder) Close() error {
	if d.zr != nil {
		return d.zr.Close()
	}
	return nil
}

func (d *messageDecoder) Decode(msg *Request) error {
	return d.buffer.DecodeMessage(msg)
}

func newMessageDecoder(r io.Reader, bufferSize int, enableCompression bool, s *ConnStats) *messageDecoder {
	r = newReaderCounter(r, s)
	br := bufio.NewReaderSize(r, bufferSize)

	rr := br
	var zr io.ReadCloser
	if enableCompression {
		zr = flate.NewReader(br)
		rr = bufio.NewReaderSize(zr, bufferSize)
	}

	return &messageDecoder{buffer: proto.NewBuffer(),zr: zr}
}
