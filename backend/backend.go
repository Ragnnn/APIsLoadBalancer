package backend

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	URL              *url.URL
	alive            bool
	mux              sync.RWMutex
	ReverseProxy     *httputil.ReverseProxy
	timeSpentAverage int64
}

func New(url *url.URL, reverseProxy *httputil.ReverseProxy, alive bool) *Backend {
	backend := &Backend{
		URL:              url,
		alive:            alive,
		mux:              sync.RWMutex{},
		ReverseProxy:     reverseProxy,
		timeSpentAverage: 0,
	}
	return backend
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.alive = alive
	b.mux.Unlock()
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.alive
	b.mux.RUnlock()
	return
}

func (b *Backend) IsMe(url string) bool {
	return b.URL.String() == url
}

func (b *Backend) TimeSpentAverage() int64 {
	return atomic.LoadInt64(&b.timeSpentAverage)
}

func (b *Backend) AddTimeSpentAverage(add int64) {
	atomic.StoreInt64(&b.timeSpentAverage, (atomic.LoadInt64(&b.timeSpentAverage)+add)/2)
}
