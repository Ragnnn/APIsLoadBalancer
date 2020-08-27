package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"git.epitekin.eu/APIsLoadBalancer/loadBalancer"
)

func main() {
	var serverList, port, authPort string
	flag.StringVar(&serverList, "backends", "", "Load balanced backends, use commas to separate")
	flag.StringVar(&port, "port", "8080", "Port to serve (default: 8080)")
	flag.StringVar(&authPort, "authPort", "8090", "Port to auth service (default: 8090)")
	flag.Parse()

	var lb *loadBalancer.LB
	lb = loadBalancer.New(strings.Split(serverList, ","), port, authPort)

	server := http.Server{
		Addr:    ":" + port,
		Handler: http.HandlerFunc(lb.LB),
	}

	log.Println("Load Balancer started at : " + port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
