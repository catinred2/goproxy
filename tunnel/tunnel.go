package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
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

type Tunnel struct {
	net.Conn
	wlock     sync.Mutex
	closed    bool
	plock     sync.RWMutex
	next_id   uint16
	weaves    map[uint16]Fiber
	dft_fiber Fiber
}

func NewTunnel(conn net.Conn, next_id uint16) (t *Tunnel) {
	t = &Tunnel{
		Conn:    conn,
		closed:  false,
		next_id: next_id,
		weaves:  make(map[uint16]Fiber, 0),
	}
	return
}

func (t *Tunnel) String() string {
	return t.Conn.LocalAddr().String()
}

func (t *Tunnel) PutIntoNextId(f Fiber) (id uint16, err error) {
	t.plock.Lock()

	startid := t.next_id
	for _, ok := t.weaves[t.next_id]; ok; _, ok = t.weaves[t.next_id] {
		t.next_id += 2
		if t.next_id == startid {
			err = errors.New("run out of stream id")
			logger.Error(err.Error())
			return
		}
	}
	id = t.next_id
	t.next_id += 2
	t.weaves[id] = f

	t.plock.Unlock()

	logger.Debugf("%p put into %s next id %d.", f, t.String(), id)
	return
}

func (t *Tunnel) PutIntoId(id uint16, f Fiber) (err error) {
	logger.Debugf("%s put into id %d: %p.", t.String(), id, f)

	t.plock.Lock()

	_, ok := t.weaves[id]
	if ok {
		return ErrIdExist
	}
	t.weaves[id] = f

	t.plock.Unlock()
	return
}

func (t *Tunnel) SendFrame(f *Frame) (err error) {
	logger.Debugf("sent %s", f.Debug())

	b := f.Pack()

	t.wlock.Lock()
	t.Conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT * time.Millisecond))
	n, err := t.Conn.Write(b)
	t.wlock.Unlock()

	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	logger.Debugf("%s wrote len:%d.", t.String(), len(b))
	return
}

func (t *Tunnel) CloseFiber(streamid uint16) (err error) {
	t.plock.Lock()
	_, ok := t.weaves[streamid]
	if !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(t.weaves, streamid)
	t.plock.Unlock()

	logger.Infof("%s remove port %d.", t.String(), streamid)
	return
}

func (t *Tunnel) Close() (err error) {
	defer t.Conn.Close()

	t.plock.RLock()
	defer t.plock.RUnlock()

	logger.Warningf("close all connects (%d) for session: %s.",
		len(t.weaves), t.String())

	for i, v := range t.weaves {
		go func() {
			err := v.CloseFiber(i)
			if err != nil {
				logger.Error(err.Error())
				return
			}
		}()
	}
	t.closed = true
	return
}

func (t *Tunnel) Loop() {
	defer t.Close()

	for {
		f, err := ReadFrame(t.Conn)
		switch err {
		default:
			logger.Error(err.Error())
			return
		case io.EOF:
			logger.Infof("%s read EOF", t.String())
			return
		case nil:
		}

		logger.Debugf("recv %s", f.Debug())

		t.plock.RLock()
		fiber, ok := t.weaves[f.FrameHeader.Streamid]
		t.plock.RUnlock()
		if !ok || fiber == nil {
			fiber = t.dft_fiber
		}

		err = fiber.SendFrame(f)
		if err != nil {
			logger.Errorf("send %s => %s(%d) failed, err: %s.",
				f.Debug(), t.String(),
				f.FrameHeader.Streamid, err.Error())
			return
		}
	}
	return
}
