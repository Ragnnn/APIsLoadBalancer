package serverPool

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"git.epitekin.eu/APIsLoadBalancer/backend"
)

const (
	maxInt64 int64 = 1<<63 - 1
)

type ServerPool struct {
	backends []*backend.Backend
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

func (s *ServerPool) getIndex() int {
	var (
		fastest      = maxInt64
		fastestIndex = len(s.backends)
	)

	for index, bck := range s.backends {
		tmp := bck.TimeSpentAverage()
		if tmp < fastest {
			fastest = tmp
			fastestIndex = index
		}
	}

	if fastestIndex == len(s.backends) {
		return 0
	}

	return fastestIndex
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.IsMe(backendUrl.String()) {
			b.SetAlive(alive)
			break
		}
	}
}

func (s *ServerPool) BestPeerHandle(w http.ResponseWriter, r *http.Request) {
	if len(s.backends) == 0 {
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintln(w, "No instance available")
		return
	}

	best := s.getIndex()
	l := len(s.backends) + best
	for i := best; i < l; i++ {
		idx := i % len(s.backends)
		if !s.backends[idx].IsAlive() {
			continue
		}
		start := time.Now()
		s.backends[idx].ReverseProxy.ServeHTTP(w, r)
		s.backends[idx].AddTimeSpentAverage(time.Since(start).Nanoseconds())
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		alive := isBackendAlive(b.URL)
		b.SetAlive(alive)
	}
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
