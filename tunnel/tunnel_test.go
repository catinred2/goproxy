package tunnel

import (
	"bytes"
	"io"
	stdlog "log"
	"net"
	"os"
	"sync"
	"testing"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/sutils"
)

func echo_server(t *testing.T, wg *sync.WaitGroup) {
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

func tunnel_server(t *testing.T, wg *sync.WaitGroup) {
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
		}()
	}
}

func SetLogging() {
	logBackend := logging.NewLogBackend(os.Stderr, "",
		stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.Lshortfile)
	logging.SetBackend(logBackend)
	logging.SetFormatter(logging.MustStringFormatter("%{level}: %{message}"))
	lv, _ := logging.LogLevel("DEBUG")
	logging.SetLevel(lv, "")
	return
}

func TestTunnel(t *testing.T) {
	SetLogging()

	var wg sync.WaitGroup
	wg.Add(2)
	go echo_server(t, &wg)
	go tunnel_server(t, &wg)
	wg.Wait()

	dc := NewDialerCreator(sutils.DefaultTcpDialer, "127.0.0.1:14755", "", "")
	client, err := dc.Create()
	if err != nil {
		t.Error(err)
		return
	}
	go client.Loop()

	conn, err := client.Dial("tcp", "127.0.0.1:14756")
	if err != nil {
		t.Error(err)
		return
	}

	s := "foobar"
	b := []byte(s)

	n, err := conn.Write(b)
	if err != nil {
		t.Error(err)
		return
	}
	if n < len(b) {
		t.Error("short write")
	}

	var buf [100]byte
	n, err = conn.Read(buf[:])
	if err != nil {
		t.Error(err)
		return
	}
	if bytes.Compare(b, buf[:n]) != 0 {
		t.Error("data not match")
		return
	}
}
