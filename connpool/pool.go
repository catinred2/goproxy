package connpool

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"sync"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/msocks"
)

const (
	DIAL_RETRY   = 2
	AUTH_TIMEOUT = 10
)

var (
	ErrNoSession       = errors.New("session in pool but can't pick one.")
	ErrSessionNotFound = errors.New("session not found.")
)

var (
	logger = logging.MustGetLogger("connpool")
)

type SessionPool struct {
	sess_lock sync.RWMutex // sess pool locker
	sess      map[*msocks.Session]struct{}
}

func NewSessionPool() (sp *SessionPool) {
	sp = &SessionPool{
		sess: make(map[*msocks.Session]struct{}, 0),
	}
	return
}

func (sp *SessionPool) CutAll() {
	sp.sess_lock.Lock()
	defer sp.sess_lock.Unlock()
	for s, _ := range sp.sess {
		s.Close()
	}
	sp.sess = make(map[*msocks.Session]struct{}, 0)
}

func (sp *SessionPool) GetSize() int {
	sp.sess_lock.RLock()
	defer sp.sess_lock.RUnlock()
	return len(sp.sess)
}

func (sp *SessionPool) GetSessions() (sessions []*msocks.Session) {
	sp.sess_lock.RLock()
	defer sp.sess_lock.RUnlock()
	for sess, _ := range sp.sess {
		sessions = append(sessions, sess)
	}
	return
}

func (sp *SessionPool) Add(s *msocks.Session) {
	sp.sess_lock.Lock()
	defer sp.sess_lock.Unlock()
	sp.sess[s] = struct{}{}
}

func (sp *SessionPool) Remove(s *msocks.Session) (err error) {
	sp.sess_lock.Lock()
	defer sp.sess_lock.Unlock()
	if _, ok := sp.sess[s]; !ok {
		return ErrSessionNotFound
	}
	delete(sp.sess, s)
	return
}

func (sp *SessionPool) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", sp.HandlerMain)
	mux.HandleFunc("/lookup", HandlerLookup)
	mux.HandleFunc("/cutoff", sp.HandlerCutoff)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
