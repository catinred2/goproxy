package msocks

import (
	"errors"
	"math/rand"
	"time"

	logging "github.com/op/go-logging"
)

const (
	AUTH_TIMEOUT  = 10000
	DIAL_TIMEOUT  = 30000
	DNS_TIMEOUT   = 30000
	WRITE_TIMEOUT = 60000
	WINDOWSIZE    = 4 * 1024 * 1024
)

const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
	ERR_CLOSED
)

var (
	ErrAuthFailed     = errors.New("auth failed %s.")
	ErrAuthTimeout    = errors.New("auth timeout %s.")
	ErrStreamNotExist = errors.New("stream not exist.")
	ErrQueueClosed    = errors.New("queue closed.")
	ErrUnexpectedPkg  = errors.New("unexpected package.")
	ErrNotSyn         = errors.New("frame result in conn which status is not syn.")
	ErrFinState       = errors.New("status not est or fin wait when get fin.")
	ErrIdExist        = errors.New("frame sync stream id exist.")
	ErrState          = errors.New("status error.")
	ErrUnknownState   = errors.New("unknown status.")
	ErrChanClosed     = errors.New("chan closed.")
	ErrDnsTimeOut     = errors.New("dns timeout.")
	ErrDnsMsgIllegal  = errors.New("dns message illegal.")
	ErrDnsLookuper    = errors.New("dns lookuper can't exchange.")
	ErrNoDnsServer    = errors.New("no proper dns server.")
)

var (
	logger = logging.MustGetLogger("msocks")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}
