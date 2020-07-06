package serverPool

import (
	"git.epitekin.eu/APIsLoadBalancer/backend"
	"log"
	"net"
	"net/url"
	"sync/atomic"
	"time"
)

type ServerPool struct {
	backends    []*backend.Backend
	bestSrvPool uint64
}

func (s *ServerPool) AddBackend(backend *backend.Backend) {
	s.backends = append(s.backends, backend)
}

func (s *ServerPool) RemoveBackend(url string) {
	for index, back := range s.backends {
		if back.IsMe(url) {
			s.backends = append(s.backends[:index], s.backends[index+1:]...)
			break
		}
	}
}

func (s *ServerPool) GetIndex() int {
	return int(atomic.LoadUint64(&s.bestSrvPool))
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.IsMe(backendUrl.String()) {
			b.SetAlive(alive)
			break
		}
	}
}

func (s *ServerPool) GetBestPeer() *backend.Backend {
	best := s.GetIndex()
	l := len(s.backends) + best
	for i := best; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != best {
				atomic.StoreUint64(&s.bestSrvPool, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) HealthCheck() {
	var fastest = int64(^uint64(0) >> 1)
	var fastestIndex = 0

	for index, b := range s.backends {
		start := time.Now()
		alive := isBackendAlive(b.URL)
		end := time.Now()

		if fastest > end.UnixNano()-start.UnixNano() {
			fastest = end.UnixNano() - start.UnixNano()
			fastestIndex = index
		}

		b.SetAlive(alive)
	}

	atomic.StoreUint64(&s.bestSrvPool, uint64(fastestIndex))
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	defer conn.Close()
	return true
}
