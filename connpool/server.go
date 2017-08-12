package connpool

import (
	"net"

	"github.com/shell909090/goproxy/tunnel"
)

type Server struct {
	*Pool
	auth *map[string]string
}

func NewServer(auth *map[string]string) (server *Server) {
	server = &Server{
		Pool: NewPool(),
		auth: auth,
	}
	return
}

func (server *Server) Handler(conn net.Conn) {
	tun := tunnel.NewServer(conn)
	server.Pool.Add(tun)
	defer server.Pool.Remove(tun)
	tun.Loop()

	logger.Noticef("server session %s quit: %s => %s.",
		tun.String(), conn.RemoteAddr(), conn.LocalAddr())
}

func (server *Server) Serve(listener net.Listener) (err error) {
	var conn net.Conn
	listener = tunnel.NewListener(listener, server.auth)

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		go func() {
			defer conn.Close()
			server.Handler(conn)
		}()
	}
	return
}
