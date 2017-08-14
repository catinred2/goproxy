package connpool

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"sort"
	"sync"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/tunnel"
)

const (
	BALANCE_INTERVAL = 60
	DIAL_RETRY       = 2
	AUTH_TIMEOUT     = 10
)

var (
	ErrNoSession       = errors.New("session in pool but can't pick one.")
	ErrSessionNotFound = errors.New("session not found.")
)

var (
	logger = logging.MustGetLogger("connpool")
)

type Pool struct {
	lock    sync.RWMutex // sess pool locker
	tunpool map[tunnel.Tunnel]struct{}
}

func NewPool() (pool *Pool) {
	pool = &Pool{
		tunpool: make(map[tunnel.Tunnel]struct{}, 0),
	}
	return
}

func (pool *Pool) CutAll() {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	for t, _ := range pool.tunpool {
		t.Close()
	}
	pool.tunpool = make(map[tunnel.Tunnel]struct{}, 0)
}

func (pool *Pool) GetSize() int {
	pool.lock.RLock()
	defer pool.lock.RUnlock()
	return len(pool.tunpool)
}

func (pool *Pool) getMinimum() (tun tunnel.Tunnel, size int) {
	size = -1
	pool.lock.RLock()
	defer pool.lock.RUnlock()
	for t, _ := range pool.tunpool {
		n := t.GetSize()
		if size == -1 || n < size {
			tun = t
			size = n
		}
	}
	return
}

type TunSlice []tunnel.Tunnel

func (ts TunSlice) Len() int      { return len(ts) }
func (ts TunSlice) Swap(i, j int) { ts[i], ts[j] = ts[j], ts[i] }
func (ts TunSlice) Less(i, j int) bool {
	return ts[i].String() < ts[j].String()
}

func (pool *Pool) GetTunnels() (tuns TunSlice) {
	tuns = func() (tuns []tunnel.Tunnel) {
		pool.lock.RLock()
		defer pool.lock.RUnlock()
		for t, _ := range pool.tunpool {
			tuns = append(tuns, t)
		}
		return
	}()
	sort.Sort(tuns)
	return
}

func (pool *Pool) Add(tun tunnel.Tunnel) {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	pool.tunpool[tun] = struct{}{}
}

func (pool *Pool) Remove(tun tunnel.Tunnel) (err error) {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	if _, ok := pool.tunpool[tun]; !ok {
		return ErrSessionNotFound
	}
	delete(pool.tunpool, tun)
	return
}

func (pool *Pool) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", pool.HandlerMain)
	mux.HandleFunc("/lookup", HandlerLookup)
	mux.HandleFunc("/cutoff", pool.HandlerCutoff)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
