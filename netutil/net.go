package netutil

import (
	"io"
	"net"
	"sync"
	"time"

	logging "github.com/op/go-logging"
)

var (
	logger = logging.MustGetLogger("sutils")
)

var (
	BUFFERSIZE = 8 * 1024
	BufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, BUFFERSIZE)
		},
	}
)

func CopyLink(dst, src io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		buf := BufferPool.Get().([]byte)
		defer BufferPool.Put(buf)
		io.CopyBuffer(src, dst, buf)
	}()
	defer dst.Close()
	buf := BufferPool.Get().([]byte)
	defer BufferPool.Put(buf)
	io.CopyBuffer(dst, src, nil)
}

type Dialer interface {
	Dial(string, string) (net.Conn, error)
}

type TimeoutDialer interface {
	Dialer
	DialTimeout(string, string, time.Duration) (net.Conn, error)
}

type TcpDialer struct {
}

func (td *TcpDialer) Dial(network, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

func (td *TcpDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

var DefaultTcpDialer TimeoutDialer = &TcpDialer{}

type Tcp4Dialer struct {
}

func (td *Tcp4Dialer) Dial(network, address string) (net.Conn, error) {
	return net.Dial("tcp4", address)
}

func (td *Tcp4Dialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp4", address, timeout)
}

var DefaultTcp4Dialer TimeoutDialer = &Tcp4Dialer{}
