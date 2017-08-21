package tunnel

import (
	"errors"

	logging "github.com/op/go-logging"
)

const (
	AUTH_TIMEOUT  = 10000
	DIAL_TIMEOUT  = 20000
	WRITE_TIMEOUT = 10000
	WINDOWSIZE    = 4 * 1024 * 1024
	// WINDOWSIZE = 100
)

const (
	MSG_UNKNOWN = iota
	MSG_RESULT
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_WND
	MSG_FIN
	MSG_RST
)

const (
	ST_UNKNOWN  = 0x00
	ST_SYN_RECV = 0x01
	ST_SYN_SENT = 0x02
	ST_EST      = 0x03
	ST_FIN_RECV = 0x04
	ST_FIN_SENT = 0x06
)

var StatusText = map[uint8]string{
	ST_UNKNOWN:  "UNKNOWN",
	ST_SYN_RECV: "SYN_RECV",
	ST_SYN_SENT: "SYN_SENT",
	ST_EST:      "ESTAB",
	ST_FIN_RECV: "FIN_RECV",
	ST_FIN_SENT: "FIN_SENT",
}

const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
	ERR_CLOSED
)

var ErrnoText = map[uint32]string{
	ERR_NONE:       "none",
	ERR_AUTH:       "auth failed",
	ERR_IDEXIST:    "stream id existed",
	ERR_CONNFAILED: "connected failed",
	ERR_TIMEOUT:    "timeout",
	ERR_CLOSED:     "connect closed",
}

var (
	ErrFrameOverFlow  = errors.New("marshal overflow in frame")
	ErrUnknownNetwork = errors.New("unknown network.")
	ErrStreamOutOfID  = errors.New("stream out of id.")
	ErrUnexpectedPkg  = errors.New("unexpected package.")
	ErrIdExist        = errors.New("frame sync stream id exist.")
	ErrState          = errors.New("status error.")
)

var (
	logger = logging.MustGetLogger("msocks")
)

type Tunnel interface {
	String() string
	GetSize() int
	Loop()
	Close() error
}
