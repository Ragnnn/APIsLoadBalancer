package loadBalancer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.epitekin.eu/APIsLoadBalancer/backend"
	"git.epitekin.eu/APIsLoadBalancer/services"
)

const (
	Attempts int = iota
	Retry

	localURL = "http://localhost"

	lbRoute     = "/lb/"
	tokenRoute  = "/auth/token"
	adminRoute  = "/admin"
	simpleRoute = "/simple"

	authAccepted = "202 Accepted"
)

type LB struct {
	serverPoolsStock *services.Services
	portStr          string
	authPortStr      string
}

func New(serverList []string, portStr, authPortStr string) *LB {
	lb := &LB{
		serverPoolsStock: services.New(),
		portStr:          portStr,
		authPortStr:      authPortStr,
	}

	lb.serverPoolsStock.AddService(adminRoute)
	lb.serverPoolsStock.AddService(simpleRoute)

	for _, server := range serverList {
		if strings.HasSuffix(server, adminRoute) {
			lb.AddInstance(server[:len(server)-len(adminRoute)], adminRoute)
		} else if strings.HasSuffix(server, simpleRoute) {
			lb.AddInstance(server[:len(server)-len(simpleRoute)], simpleRoute)
		}
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

func (lb *LB) RemoveInstance(serverUrl, route string) {
	lb.serverPoolsStock.FromServerPoolRemoveBackend(route, serverUrl)
	log.Printf("Removed server: %s\n", serverUrl)

}

func (lb *LB) AddInstance(server string, route string) {
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

		lb.serverPoolsStock.FromServerPoolMarkBackendStatus(route, serverUrl, false)

		attempts := lb.GetAttemptsFromContext(request)
		log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
		ctx := context.WithValue(request.Context(), Attempts, attempts+1)
		lb.LB(writer, request.WithContext(ctx))
	}

	bck := backend.New(serverUrl, proxy, true)

	lb.serverPoolsStock.FromServerPoolAddBackend(route, bck)
	log.Printf("Configured server: %s\n", serverUrl)
}

func (lb *LB) LB(w http.ResponseWriter, r *http.Request) {
	attempts := lb.GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	var (
		client = &http.Client{
			Timeout: time.Second * 5,
		}
		authRequest *http.Request
		resp        *http.Response
		err         error
	)

	authRequest, err = http.NewRequest(r.Method, strings.Replace("http://"+r.Host+r.URL.String(), lb.portStr, lb.authPortStr, 1), nil)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	authRequest.Header.Set("Authorization", r.Header.Get("Authorization"))

	resp, err = client.Do(authRequest)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if resp.Status != authAccepted {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.String() == tokenRoute {
		w.Header().Set("Authorization", resp.Header.Get("Authorization"))
		w.WriteHeader(resp.StatusCode)
		return
	}

	route := r.URL.String()
	if strings.HasPrefix(r.URL.Path, lbRoute) {
		args := strings.SplitN(r.URL.String()[1:], "/", 4)
		if _, err := strconv.Atoi(args[2]); err != nil {
			http.Error(w, "need a valid port", http.StatusBadRequest)
			return
		}
		if args[1] == "add" {
			lb.AddInstance(fmt.Sprintf("%s:%s", localURL, args[2]), "/"+args[3])
		} else if args[1] == "remove" {
			lb.RemoveInstance(fmt.Sprintf("%s:%s", localURL, args[2]), "/"+args[3])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "Service added")
		return
	} else if strings.HasPrefix(r.URL.Path, simpleRoute) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, simpleRoute)
	} else if strings.HasPrefix(r.URL.Path, adminRoute) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, adminRoute)
	}

	lb.serverPoolsStock.FromServerPoolBestPeerHandle(route, w, r)
}

func (lb *LB) healthCheck() {
	t := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-t.C:
			lb.serverPoolsStock.HealthCheck()
		}
	}
}
