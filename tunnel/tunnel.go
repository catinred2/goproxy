package tunnel

import (
	"errors"
	"net"

	logging "github.com/op/go-logging"
)

const (
	AUTH_TIMEOUT  = 10000
	DIAL_TIMEOUT  = 20000
	WRITE_TIMEOUT = 10000
	WINDOWSIZE    = 4 * 1024 * 1024
)

// FIXME:
const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
	ERR_CLOSED
)

var (
	ErrShortRead      = errors.New("short read.")
	ErrShortWrite     = errors.New("short write.")
	ErrUnknownNetwork = errors.New("unknown network.")
	ErrAuthFailed     = errors.New("auth failed %s.")
	ErrAuthTimeout    = errors.New("auth timeout %s.")
	ErrStreamNotExist = errors.New("stream not exist.")
	ErrStreamOutOfID  = errors.New("stream out of id.")
	ErrQueueClosed    = errors.New("queue closed.")
	ErrUnexpectedPkg  = errors.New("unexpected package.")
	ErrIdExist        = errors.New("frame sync stream id exist.")
	ErrState          = errors.New("status error.")
)

var (
	logger = logging.MustGetLogger("msocks")
)

type Tunnel interface {
	GetSize() int
	Dial(string, string) (net.Conn, error)
	Loop()
}
