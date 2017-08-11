package tunnel

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	MSG_UNKNOWN = iota
	MSG_RESULT
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_WND
	MSG_FIN
	MSG_RST
	MSG_PING
	MSG_DNS
	MSG_SPAM
)

var (
	ErrFrameOverFlow = errors.New("marshal overflow in frame")
)

type FrameHeader struct {
	Type     uint8
	Length   uint16
	Streamid uint16
}

func (hdr *FrameHeader) Debug() string {
	return fmt.Sprintf("frame: type(%d), stream(%d), len(%d).",
		hdr.Type, hdr.Streamid, hdr.Length)
}

type Result uint32

type Auth struct {
	Username string
	Password string
}

type Syn struct {
	Network string
	Address string
}

type Wnd uint32

type Frame struct {
	FrameHeader
	Data []byte
}

func ReadFrame(r io.Reader) (f *Frame, err error) {
	f = new(Frame)
	err = binary.Read(r, binary.BigEndian, &f.FrameHeader)
	if err != nil {
		return
	}

	f.Data = make([]byte, f.FrameHeader.Length)
	n, err := r.Read(f.Data)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	if n != int(f.FrameHeader.Length) {
		return nil, ErrShortRead
	}
	return
}

func NewFrame(tp uint8, streamid uint16) (f *Frame) {
	f = &Frame{
		FrameHeader: FrameHeader{
			Type:     tp,
			Streamid: streamid,
		},
	}
	return
}

func (f *Frame) Marshal(v interface{}) (err error) {
	f.Data, err = json.Marshal(v)
	if err != nil {
		return
	}
	if len(f.Data) > (1<<16 - 1) {
		return ErrFrameOverFlow
	}
	f.FrameHeader.Length = uint16(len(f.Data))
	return
}

func (f *Frame) Unmarshal(v interface{}) (err error) {
	err = json.Unmarshal(f.Data, v)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}

func (f *Frame) Pack() (b []byte) {
	var buf bytes.Buffer
	buf.Grow(int(5 + f.FrameHeader.Length))
	binary.Write(&buf, binary.BigEndian, f.FrameHeader)
	buf.Write(f.Data)
	return buf.Bytes()
}

func (f *Frame) WriteTo(stream io.Writer) (err error) {
	b := f.Pack()
	n, err := stream.Write(b)
	if err != nil {
		return
	}
	if n != len(b) {
		return ErrShortWrite
	}
	return
}

type Fiber interface {
	SendFrame(*Frame) error
	CloseFiber(uint16) error
}
