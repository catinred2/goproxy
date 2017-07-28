package connpool

import (
	"net"
	"time"

	"github.com/shell909090/goproxy/msocks"
)

type SessionPoolServer struct {
	*SessionPool
	*msocks.Server
}

func NewServer(auth map[string]string) (sps *SessionPoolServer) {
	sps = &SessionPoolServer{
		SessionPool: NewSessionPool(),
		Server:      msocks.NewServer(auth),
	}
	return
}

func (sps *SessionPoolServer) Handler(conn net.Conn) {
	logger.Notice("connection come from: %s => %s.", conn.RemoteAddr(), conn.LocalAddr())

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Second, func() {
		logger.Notice(msocks.ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})

	err := sps.Server.OnAuth(conn)
	if err != nil {
		logger.Error("%s", err.Error())
		return
	}
	ti.Stop()

	sess := msocks.NewSession(conn)

	sps.SessionPool.Add(sess)
	defer sps.SessionPool.Remove(sess)
	sess.Run()

	logger.Notice("server session %d quit: %s => %s.",
		sess.LocalPort(), conn.RemoteAddr(), conn.LocalAddr())
}

func (sps *SessionPoolServer) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Error("%s", err)
			continue
		}
		go func() {
			defer conn.Close()
			sps.Handler(conn)
		}()
	}
	return
}
