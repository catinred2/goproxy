package dns

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"

	"github.com/miekg/dns"
)

func writeMsg(conn net.Conn, msg *dns.Msg) (err error) {
	p, err := msg.Pack()
	if err != nil {
		return
	}
	size := len(p)
	if size > 2<<16-1 {
		return ErrMessageTooLarge
	}

	buf := make([]byte, 2, size+2)
	binary.BigEndian.PutUint16(buf, uint16(size))
	p = append(buf, p...)
	_, err = io.Copy(conn, bytes.NewReader(p))
	return
}

func readMsg(conn net.Conn) (msg *dns.Msg, err error) {
	var buf_size [2]byte
	n, err := io.ReadFull(conn, buf_size[:])
	if err != nil {
		logger.Error(err.Error())
		return
	}
	size := binary.BigEndian.Uint16(buf_size[:n])

	buf := make([]byte, size)
	n, err = io.ReadFull(conn, buf[:size])
	if err != nil {
		logger.Error(err.Error())
		return
	}

	msg = new(dns.Msg)
	err = msg.Unpack(buf[:n])
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}
