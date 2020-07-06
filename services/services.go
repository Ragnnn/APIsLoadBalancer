package services

import (
	"git.epitekin.eu/APIsLoadBalancer/backend"
	"git.epitekin.eu/APIsLoadBalancer/serverPool"
	"net/url"
	"strings"
	"sync"
)

type (
	Services struct {
		services map[string]*protected
	}
	protected struct {
		serverPool.ServerPool
		mux sync.Mutex
	}
)

func New() *Services {
	return &Services{
		services: make(map[string]*protected),
	}
}

func (s *Services) AddService(urlBase string) {
	s.services[urlBase] = &protected{
		ServerPool: serverPool.ServerPool{},
		mux:        sync.Mutex{},
	}
}

func (s *Services) getService(route string) *protected {
	for key, value := range s.services {
		if strings.HasPrefix(route, key) {
			return value
		}
	}
	return nil
}

func (s *Services) FromServerPoolAddBackend(route string, backend *backend.Backend) {
	service := s.getService(route)
	service.mux.Lock()
	defer service.mux.Unlock()
	service.AddBackend(backend)
}

func (s *Services) FromServerPoolRemoveBackend(route, serverURL string) {
	service := s.getService(route)
	service.mux.Lock()
	defer service.mux.Unlock()
	service.RemoveBackend(serverURL)
}

func (s *Services) FromServerPoolGetIndex(route string) int {
	service := s.getService(route)
	service.mux.Lock()
	defer service.mux.Unlock()
	return service.GetIndex()
}

func (s *Services) FromServerPoolMarkBackendStatus(route string, backendUrl *url.URL, alive bool) {
	service := s.getService(route)
	service.mux.Lock()
	defer service.mux.Unlock()
	service.MarkBackendStatus(backendUrl, alive)
}

func (s *Services) FromServerPoolGetBestPeer(route string) *backend.Backend {
	service := s.getService(route)
	service.mux.Lock()
	defer service.mux.Unlock()
	return service.GetBestPeer()
}

func (s *Services) HealthCheck() {
	for _, service := range s.services {
		service.mux.Lock()
		service.HealthCheck()
		service.mux.Unlock()
	}
}
