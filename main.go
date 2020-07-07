package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"git.epitekin.eu/APIsLoadBalancer/loadBalancer"
)

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Load balanced backends, use commas to separate")
	flag.IntVar(&port, "port", 8080, "Port to serve (default: 8080)")
	flag.Parse()

	var lb *loadBalancer.LB
	lb = loadBalancer.New(strings.Split(serverList, ","))

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb.LB),
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
