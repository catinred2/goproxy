package tunnel

import (
	"net"
	"sync"
	"testing"
)

func TunnelServer(t *testing.T, wg *sync.WaitGroup) (err error) {
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
