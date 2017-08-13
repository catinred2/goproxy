package connpool

import (
	"net"

	"github.com/shell909090/goproxy/tunnel"
)

type Server struct {
	*Pool
	tunnel.Server
	auth *map[string]string
}

func NewServer(auth *map[string]string) (server *Server) {
	if auth != nil && len(*auth) == 0 {
		auth = nil
	}
	server = &Server{
		Pool: NewPool(),
		auth: auth,
	}
	server.Server.Handler = server
	return
}

func (server *Server) AuthPass(username, password string) bool {
	if server.auth == nil {
		return true
	}
	password1, ok := (*server.auth)[username]
	if !ok {
		return false
	}
	if password1 != password {
		return false
	}
	return true
}

func (server *Server) Handle(conn net.Conn) (err error) {
	err = tunnel.AuthConn(server, conn)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	tun := tunnel.NewTunnelServer(conn)
	server.Pool.Add(tun)
	defer server.Pool.Remove(tun)
	tun.Loop()
	logger.Noticef("server session %s quit: %s => %s.",
		tun.String(), conn.RemoteAddr(), conn.LocalAddr())
	return
}
