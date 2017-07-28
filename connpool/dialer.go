package connpool

import (
	"math/rand"
	"net"
	"sync"

	"github.com/shell909090/goproxy/msocks"
)

type SessionPoolDialer struct {
	*SessionPool
	MinSess       int
	MaxConn       int
	creators_lock sync.Mutex
	creators      []*msocks.DialerCreator
}

func NewDialer(MinSess, MaxConn int) (spd *SessionPoolDialer) {
	if MinSess == 0 {
		MinSess = 1
	}
	if MaxConn == 0 {
		MaxConn = 32
	}
	spd = &SessionPoolDialer{
		SessionPool: NewSessionPool(),
		MinSess:     MinSess,
		MaxConn:     MaxConn,
	}
	return
}

func (spd *SessionPoolDialer) AddDialerCreator(dc *msocks.DialerCreator) {
	spd.creators_lock.Lock()
	defer spd.creators_lock.Unlock()
	spd.creators = append(spd.creators, dc)
}

// Get one or create one.
func (spd *SessionPoolDialer) Get() (sess *msocks.Session, err error) {
	sess_len := spd.GetSize()

	if sess_len == 0 {
		err = spd.createSession(func() bool {
			return spd.GetSize() == 0
		})
		if err != nil {
			return nil, err
		}
		sess_len = spd.GetSize()
	}

	sess, size := spd.getMinimumSess()
	if sess == nil {
		return nil, ErrNoSession
	}

	if size > spd.MaxConn || sess_len < spd.MinSess {
		go spd.createSession(func() bool {
			if spd.GetSize() < spd.MinSess {
				return true
			}
			// normally, size == -1 should never happen
			_, size := spd.getMinimumSess()
			return size > spd.MaxConn
		})
	}
	return
}

// Randomly select a server, try to connect with it. If it is failed, try next.
// Repeat for DIAL_RETRY times.
// Each time it will take 2 ^ (net.ipv4.tcp_syn_retries + 1) - 1 second(s).
// eg. net.ipv4.tcp_syn_retries = 4, connect will timeout in 2 ^ (4 + 1) -1 = 31s.
func (spd *SessionPoolDialer) createSession(checker func() bool) (err error) {
	var sess *msocks.Session
	spd.creators_lock.Lock()

	if checker != nil && !checker() {
		spd.creators_lock.Unlock()
		return
	}

	start := rand.Int()
	end := start + DIAL_RETRY*len(spd.creators)
	for i := start; i < end; i++ {
		c := spd.creators[i%len(spd.creators)]
		sess, err = c.Create()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		break
	}
	spd.creators_lock.Unlock()

	if err != nil {
		logger.Critical("can't connect to any server, quit.")
		return
	}
	logger.Notice("session created.")

	spd.Add(sess)
	go spd.sessRun(sess)
	return
}

func (spd *SessionPoolDialer) getMinimumSess() (sess *msocks.Session, size int) {
	size = -1
	spd.sess_lock.RLock()
	defer spd.sess_lock.RUnlock()
	for s, _ := range spd.sess {
		ssize := s.GetSize()
		if size == -1 || ssize < size {
			sess = s
			size = s.GetSize()
		}
	}
	return
}

// Don't need to check less session here.
// Mostly, less sess counter in here will not more then the counter in Get.
// The only exception is that the closing session is the one and only one
// lower then max_conn
// but we can think that as over max_conn line just happened.
func (spd *SessionPoolDialer) sessRun(sess *msocks.Session) {
	defer func() {
		err := spd.Remove(sess)
		if err != nil {
			logger.Error(err.Error())
		}
	}()

	sess.Run()
	logger.Warning("session runtime quit.")
	return
}

func (spd *SessionPoolDialer) Dial(network, address string) (net.Conn, error) {
	sess, err := spd.Get()
	if err != nil {
		return nil, err
	}
	return sess.Dial(network, address)
}

func (spd *SessionPoolDialer) LookupIP(host string) (addrs []net.IP, err error) {
	sess, err := spd.Get()
	if err != nil {
		return
	}
	return sess.LookupIP(host)
}
