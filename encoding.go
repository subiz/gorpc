package gorpc

import (
	"bufio"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"io"
)

type messageEncoder struct{ bw *bufio.Writer }

func (e *messageEncoder) Close() error { return nil }

func (e *messageEncoder) Flush() error { return e.bw.Flush() }

func (e *messageEncoder) Encode(msg proto.Message) error {
	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	// first send the size of the payload
	var bytes4 [4]byte
	var sizeb = bytes4[:]
	binary.LittleEndian.PutUint32(sizeb, uint32(len(b)))
	if _, err := e.bw.Write(sizeb); err != nil {
		return err
	}

	// then send the actual payload
	_, err = e.bw.Write(b)
	return err
}

func newMessageEncoder(w io.Writer, bufferSize int, s *ConnStats) *messageEncoder {
	w = newWriterCounter(w, s)
	bw := bufio.NewWriterSize(w, bufferSize)
	return &messageEncoder{bw: bw}
}

type messageDecoder struct {
	buffer *proto.Buffer
	br     io.Reader
}

func (d *messageDecoder) Close() error { return nil }

func (d *messageDecoder) Decode(msg proto.Message) error {
	var bytes4 [4]byte
	var sizeb = bytes4[:]
	if _, err := d.br.Read(sizeb); err != nil {
		return err
	}

	size := binary.LittleEndian.Uint32(sizeb)
	payload := make([]byte, size, size)

	if _, err := d.br.Read(payload); err != nil {
		return err
	}

	d.buffer.Reset()
	d.buffer.SetBuf(payload)
	return d.buffer.DecodeMessage(msg)
}

func newMessageDecoder(r io.Reader, bufferSize int, s *ConnStats) *messageDecoder {
	r = newReaderCounter(r, s)
	br := bufio.NewReaderSize(r, bufferSize)
	return &messageDecoder{buffer: proto.NewBuffer(nil), br: br}
}
