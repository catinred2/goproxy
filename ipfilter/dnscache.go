package ipfilter

import (
	"errors"
	"net"
	"sync"

	"github.com/shell909090/goproxy/dns"
)

const maxCache = 512

var errType = errors.New("type error")

type DNSCache struct {
	lock  sync.Mutex
	cache *Cache
}

func CreateDNSCache() (dc *DNSCache) {
	dc = &DNSCache{
		cache: New(maxCache),
	}
	return
}

func (dc *DNSCache) LookupIP(hostname string) (addrs []net.IP, err error) {
	dc.lock.Lock()
	value, ok := dc.cache.Get(hostname)
	dc.lock.Unlock()

	if ok {
		addrs, ok = value.([]net.IP)
		if !ok {
			err = errType
		}
		logger.Debugf("hostname %s cached.", hostname)
		return
	}

	addrs, err = dns.DefaultResolver.LookupIP(hostname)
	if err != nil {
		return
	}

	if len(addrs) > 0 {
		dc.lock.Lock()
		logger.Noticef("hostname %s in caching.", hostname)
		dc.cache.Add(hostname, addrs)
		dc.lock.Unlock()
	}
	return
}
