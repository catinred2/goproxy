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
	return fmt.Sprintf(
		"%s->%s",
		t.Conn.LocalAddr().String(),
		t.Conn.RemoteAddr().String())
}

func (t *Tunnel) PutIntoNextId(f Fiber) (id uint16, err error) {
	t.plock.Lock()
	defer t.plock.Unlock()

	startid := t.next_id
	for _, ok := t.weaves[t.next_id]; ok; _, ok = t.weaves[t.next_id] {
		t.next_id += 2
		if t.next_id == startid {
			err = ErrStreamOutOfID
			logger.Error(err.Error())
			return
		}
	}
	id = t.next_id
	t.next_id += 2
	t.weaves[id] = f

	logger.Debugf("%s put %p into %s.", t.String(), f, id)
	return
}

func (t *Tunnel) PutIntoId(id uint16, f Fiber) (err error) {
	t.plock.Lock()
	defer t.plock.Unlock()

	_, ok := t.weaves[id]
	if ok {
		return ErrIdExist
	}
	t.weaves[id] = f

	logger.Debugf("%s put %p into %d.", t.String(), f, id)
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
	logger.Debugf("%s wrote len(%d).", t.String(), len(b))
	return
}

func (t *Tunnel) CloseFiber(streamid uint16) (err error) {
	t.plock.Lock()
	defer t.plock.Unlock()
	if _, ok := t.weaves[streamid]; !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(t.weaves, streamid)

	logger.Infof("%s remove port %d.", t.String(), streamid)
	return
}

func (t *Tunnel) Close() (err error) {
	defer t.Conn.Close()

	t.plock.RLock()
	defer t.plock.RUnlock()
	if t.closed {
		return
	}
	t.closed = true

	logger.Warningf("%s close all connects (%d).", t.String(), len(t.weaves))
	for i, f := range t.weaves {
		go func(streamid uint16, fiber Fiber) {
			// conn.CloseFiber may call tunnel.CloseFiber,
			// which will try to lock tunnel.plock.
			// use goroutine to provent daedlock.
			err := fiber.CloseFiber(streamid)
			if err != nil {
				logger.Error(err.Error())
				return
			}
		}(i, f)
	}
	return
}

func (t *Tunnel) Loop() {
	defer t.Close()

	for {
		f, err := ReadFrame(t.Conn, nil)
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
		fiber, ok := t.weaves[f.Header.Streamid]
		t.plock.RUnlock()
		if !ok || fiber == nil {
			fiber = t.dft_fiber
		}

		err = fiber.SendFrame(f)
		if err != nil {
			logger.Errorf("send %s => %s(%d) failed, err: %s.",
				f.Debug(), t.String(),
				f.Header.Streamid, err.Error())
			return
		}
	}
	return
}
