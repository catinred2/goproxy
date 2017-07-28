package connpool

import (
	"net"

	"github.com/shell909090/goproxy/msocks"
)

type SessionPoolServer struct {
	*SessionPool
	auth *map[string]string
}

func NewServer(auth *map[string]string) (sps *SessionPoolServer) {
	sps = &SessionPoolServer{
		SessionPool: NewSessionPool(),
		auth:        auth,
	}
	return
}

func (sps *SessionPoolServer) Handler(conn net.Conn) {
	sess := msocks.NewSession(conn, 1)
	sps.SessionPool.Add(sess)
	defer sps.SessionPool.Remove(sess)
	sess.Run()

	logger.Noticef("server session %d quit: %s => %s.",
		sess.LocalPort(), conn.RemoteAddr(), conn.LocalAddr())
}

func (sps *SessionPoolServer) Serve(listener net.Listener) (err error) {
	var conn net.Conn
	listener = msocks.NewListener(listener, sps.auth)

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		go func() {
			defer conn.Close()
			sps.Handler(conn)
		}()
	}
	return
}
