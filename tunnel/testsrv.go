package tunnel

import (
	"io"
	"net"
	"sync"
	"testing"
)

func EchoServer(t *testing.T, wg *sync.WaitGroup) {
	listener, err := net.Listen("tcp", "127.0.0.1:14756")
	if err != nil {
		t.Error(err)
		return
	}
	wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			continue
		}
		go func() {
			var buf [1024]byte
			defer conn.Close()

			for {
				n, err := conn.Read(buf[:])
				switch err {
				default:
					t.Error(err)
					return
				case io.EOF:
					return
				case nil:
				}

				_, err = conn.Write(buf[:n])
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
}

func TunnelServer(t *testing.T, wg *sync.WaitGroup) {
	listener, err := net.Listen("tcp", "127.0.0.1:14755")
	if err != nil {
		t.Error(err)
		return
	}

	listener = NewListener(listener, nil)
	wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			continue
		}
		go func() {
			defer conn.Close()
			srv := NewServer(conn)
			srv.Loop()
			logger.Warning("server loop quit")
		}()
	}
}
