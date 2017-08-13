package tunnel

import (
	"net"
	"sync"
)

type MockServer struct {
}

func RunMockServer(wg *sync.WaitGroup) (err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:14755")
	if err != nil {
		return
	}
	wg.Done()

	server := Server{
		Handler: &MockServer{},
	}
	err = server.Serve(listener)
	return
}

func (m *MockServer) AuthPass(username, password string) bool {
	return true
}

func (m *MockServer) Handle(conn net.Conn) (err error) {
	err = AuthConn(m, conn)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	tun := NewTunnelServer(conn)
	tun.Loop()
	logger.Warning("server loop quit")
	return
}