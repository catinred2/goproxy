package tunnel

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type Header struct {
	Type     uint8
	Length   uint16
	Streamid uint16
}

func (hdr *Header) Debug() string {
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

// TODO: use json in wnd may cause performance problem.
type Wnd uint32

type Frame struct {
	Header
	Data []byte
}

func ReadFrame(r io.Reader, v interface{}) (f *Frame, err error) {
	f = new(Frame)
	err = binary.Read(r, binary.BigEndian, &f.Header)
	if err != nil {
		return
	}

	f.Data = make([]byte, f.Header.Length)
	_, err = io.ReadFull(r, f.Data)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if v != nil {
		err = f.Unmarshal(v)
		if err != nil {
			return
		}
	}
	return
}

func SendFrame(fiber Fiber, tp uint8, streamid uint16, v interface{}) (err error) {
	f := NewFrame(tp, streamid)
	if v != nil {
		err = f.Marshal(v)
		if err != nil {
			return
		}
	}
	err = fiber.SendFrame(f)
	return
}

func WriteFrame(stream io.Writer, tp uint8, streamid uint16, v interface{}) (err error) {
	f := NewFrame(tp, streamid)
	if v != nil {
		err = f.Marshal(v)
		if err != nil {
			return
		}
	}
	err = f.WriteTo(stream)
	return
}

func NewFrame(tp uint8, streamid uint16) (f *Frame) {
	f = &Frame{
		Header: Header{
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
	f.Header.Length = uint16(len(f.Data))
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
	buf.Grow(int(5 + f.Header.Length))
	binary.Write(&buf, binary.BigEndian, f.Header)
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
		return io.ErrShortWrite
	}
	return
}

type Fiber interface {
	SendFrame(*Frame) error
	CloseFiber(uint16) error
}
