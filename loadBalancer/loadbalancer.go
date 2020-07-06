package loadBalancer

import (
	"context"
	"fmt"
	"git.epitekin.eu/APIsLoadBalancer/backend"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.epitekin.eu/APIsLoadBalancer/serverPool"
)

const (
	Attempts int = iota
	Retry

	localURL = "http://localhost"
)

type LB struct {
	srvPool serverPool.ServerPool
	mux sync.Mutex
}

func New(serverList []string) *LB {
	lb := &LB{}
	for _, server := range serverList {
		lb.AddInstance(server)
	}

	go lb.healthCheck()

	return lb
}

func (lb *LB) GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

func (lb *LB) GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func (lb *LB) AddInstance(server string) {
	serverUrl, err := url.Parse(server)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(serverUrl)
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
		log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
		retries := lb.GetRetryFromContext(request)
		if retries < 3 {
			select {
			case <-time.After(10 * time.Millisecond):
				ctx := context.WithValue(request.Context(), Retry, retries+1)
				proxy.ServeHTTP(writer, request.WithContext(ctx))
			}
			return
		}

		lb.mux.Lock()
		lb.srvPool.MarkBackendStatus(serverUrl, false)
		lb.mux.Unlock()

		attempts := lb.GetAttemptsFromContext(request)
		log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
		ctx := context.WithValue(request.Context(), Attempts, attempts+1)
		lb.LB(writer, request.WithContext(ctx))
	}

	bck := &backend.Backend{
		URL:          serverUrl,
		ReverseProxy: proxy,
	}
	bck.SetAlive(true)

	lb.mux.Lock()
	lb.srvPool.AddBackend(bck)
	lb.mux.Unlock()
	log.Printf("Configured server: %s\n", serverUrl)
}

func (lb *LB) LB(w http.ResponseWriter, r *http.Request) {
	attempts := lb.GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	if strings.HasPrefix(r.URL.String(), "/lb/") {
		args := strings.SplitN(r.URL.String()[1:], "/", 2)
		if _, err := strconv.Atoi(args[1]); err != nil {
			http.Error(w, "need a valid port", http.StatusBadRequest)
			return
		}
		lb.AddInstance(fmt.Sprintf("%s:%s", localURL, args[1]))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "Service added")
		return
	}

	lb.mux.Lock()
	peer := lb.srvPool.GetNextPeer()
	lb.mux.Unlock()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

func (lb *LB) healthCheck() {
	t := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-t.C:
			lb.mux.Lock()
			lb.srvPool.HealthCheck()
			lb.mux.Unlock()
		}
	}
}
