package main

import (
	"flag"
	"fmt"
	"git.epitekin.eu/APIsLoadBalancer/loadBalancer"
	"log"
	"net/http"
	"strings"
)

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Load balanced backends, use commas to separate")
	flag.IntVar(&port, "port", 3030, "Port to serve")
	flag.Parse()

	var lb *loadBalancer.LB
	lb = loadBalancer.New(strings.Split(serverList, ","))

	// create http server
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb.LB),
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}