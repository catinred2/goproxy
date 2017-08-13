package netutil

import (
	"io"
	"net"
	"sync"
)

func EchoServer(wg *sync.WaitGroup) {
	listener, err := net.Listen("tcp", "127.0.0.1:14756")
	if err != nil {
		logger.Error(err)
		return
	}
	wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error(err)
			continue
		}
		go func() {
			var buf [1024]byte
			defer conn.Close()

			for {
				n, err := conn.Read(buf[:])
				switch err {
				default:
					logger.Error(err)
					return
				case io.EOF:
					return
				case nil:
				}

				_, err = conn.Write(buf[:n])
				if err != nil {
					logger.Error(err)
					return
				}
			}
		}()
	}
}
