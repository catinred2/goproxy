package msocks

import (
	"fmt"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type SessionFactory struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
}

func NewSessionFactory(dialer sutils.Dialer, serveraddr, username, password string) (sf *SessionFactory) {
	return &SessionFactory{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
}

func (sf *SessionFactory) CreateSession() (sess *Session, err error) {
	logger.Notice("msocks try to connect %s.", sf.serveraddr)

	conn, err := sf.Dialer.Dial("tcp4", sf.serveraddr)
	if err != nil {
		return
	}

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Second, func() {
		logger.Notice(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})
	defer func() {
		ti.Stop()
	}()

	if sf.username != "" || sf.password != "" {
		logger.Notice("auth with username: %s, password: %s.", sf.username, sf.password)
	}
	fb := NewFrameAuth(0, sf.username, sf.password)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return
	}

	f, err := ReadFrame(conn)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameResult)
	if !ok {
		return nil, ErrUnexpectedPkg
	}

	if ft.Errno != ERR_NONE {
		conn.Close()
		return nil, fmt.Errorf("create connection failed with code: %d.", ft.Errno)
	}

	logger.Notice("auth passed.")
	sess = NewSession(conn)
	sess.next_id = 0
	return
}
