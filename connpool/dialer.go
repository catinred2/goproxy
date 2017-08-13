package connpool

import (
	"math/rand"
	"net"
	"sync"

	"github.com/shell909090/goproxy/netutil"
	"github.com/shell909090/goproxy/tunnel"
)

type Dialer struct {
	*Pool
	MinSess  int
	MaxConn  int
	lock     sync.RWMutex
	creators []*tunnel.DialerCreator
}

func NewDialer(MinSess, MaxConn int) (dialer *Dialer) {
	if MinSess == 0 {
		MinSess = 1
	}
	if MaxConn == 0 {
		MaxConn = 32
	}
	dialer = &Dialer{
		Pool:    NewPool(),
		MinSess: MinSess,
		MaxConn: MaxConn,
	}
	return
}

func (dialer *Dialer) AddDialerCreator(orig *tunnel.DialerCreator) {
	dialer.lock.Lock()
	defer dialer.lock.Unlock()
	dialer.creators = append(dialer.creators, orig)
}

// Get one or create one.
func (dialer *Dialer) Get() (tun tunnel.Tunnel, err error) {
	tsize := dialer.GetSize()

	if tsize == 0 {
		err = dialer.createSession(func() bool {
			return dialer.GetSize() == 0
		})
		if err != nil {
			return nil, err
		}
		tsize = dialer.GetSize()
	}

	tun, fsize := dialer.getMinimum()
	if tun == nil {
		return nil, ErrNoSession
	}

	if fsize > dialer.MaxConn || tsize < dialer.MinSess {
		go dialer.createSession(func() bool {
			if dialer.GetSize() < dialer.MinSess {
				return true
			}
			// normally, size == -1 should never happen
			_, fsize := dialer.getMinimum()
			return fsize > dialer.MaxConn
		})
	}
	return
}

// Randomly select a server, try to connect with it. If it is failed, try next.
// Repeat for DIAL_RETRY times.
// Each time it will take 2 ^ (net.ipv4.tcp_syn_retries + 1) - 1 second(s).
// eg. net.ipv4.tcp_syn_retries = 4, connect will timeout in 2 ^ (4 + 1) -1 = 31s.
func (dialer *Dialer) createSession(checker func() bool) (err error) {
	var tun tunnel.Tunnel
	dialer.lock.Lock()
	if checker != nil && !checker() {
		dialer.lock.Unlock()
		return
	}

	start := rand.Int()
	end := start + DIAL_RETRY*len(dialer.creators)
	for i := start; i < end; i++ {
		orig := dialer.creators[i%len(dialer.creators)]
		tun, err = orig.Create()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		break
	}
	dialer.lock.Unlock()

	if err != nil {
		logger.Critical("can't connect to any server, quit.")
		return
	}
	logger.Notice("session created.")

	dialer.Add(tun)
	go dialer.sessRun(tun)
	return
}

func (dialer *Dialer) getMinimum() (tun tunnel.Tunnel, size int) {
	size = -1
	dialer.lock.RLock()
	defer dialer.lock.RUnlock()
	for t, _ := range dialer.tunpool {
		n := t.GetSize()
		if size == -1 || n < size {
			tun = t
			size = n
		}
	}
	return
}

// Don't need to check less session here.
// Mostly, less sess counter in here will not more then the counter in Get.
// The only exception is that the closing session is the one and only one
// lower then max_conn
// but we can think that as over max_conn line just happened.
func (dialer *Dialer) sessRun(tun tunnel.Tunnel) {
	defer func() {
		err := dialer.Remove(tun)
		if err != nil {
			logger.Error(err.Error())
		}
	}()

	tun.Loop()
	logger.Warning("session runtime quit.")
	return
}

func (dialer *Dialer) Dial(network, address string) (net.Conn, error) {
	tun, err := dialer.Get()
	if err != nil {
		return nil, err
	}
	d, ok := tun.(netutil.Dialer)
	if !ok {
		panic("tunnel not a dialer in client side.")
	}
	return d.Dial(network, address)
}

// func (dialer *Dialer) LookupIP(host string) (addrs []net.IP, err error) {
// 	sess, err := dialer.Get()
// 	if err != nil {
// 		return
// 	}
// 	return sess.LookupIP(host)
// }
