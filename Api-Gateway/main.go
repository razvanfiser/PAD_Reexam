package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// RoundRobinLoadBalancer is a simple round-robin load balancer.
type RoundRobinLoadBalancer struct {
	mu      sync.Mutex
	servers []*url.URL
	index   int
}

// NextServer returns the next server in a round-robin fashion.
func (lb *RoundRobinLoadBalancer) NextServer() *url.URL {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.servers) == 0 {
		return nil
	}

	server := lb.servers[lb.index]
	lb.index = (lb.index + 1) % len(lb.servers)
	return server
}

// NewRoundRobinLoadBalancer creates a new RoundRobinLoadBalancer with the given servers.
func NewRoundRobinLoadBalancer(servers []string) *RoundRobinLoadBalancer {
	var urls []*url.URL
	for _, s := range servers {
		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}
		urls = append(urls, u)
	}

	return &RoundRobinLoadBalancer{servers: urls}
}

func main() {
	// Define the sports and imgur service endpoints
	sportsService := NewRoundRobinLoadBalancer([]string{
		"http://sports1-container.pad_reexam:5000",
		"http://sports2-container.pad_reexam:5000",
	})

	imgurService := NewRoundRobinLoadBalancer([]string{
		"http://imgur1-container.pad_reexam:5000",
		"http://imgur2-container.pad_reexam:5000",
	})

	// Create a new router using gorilla/mux
	router := http.NewServeMux()

	// Define handlers for sports service
	router.HandleFunc("/", createReverseProxyHandler(sportsService))
	router.HandleFunc("/_getsportlist", createReverseProxyHandler(sportsService))

	// Define handlers for imgur service
	router.HandleFunc("/search", createReverseProxyHandler(imgurService))
	router.HandleFunc("/get_db", createReverseProxyHandler(imgurService))

	// Start the API gateway on port 8080
	fmt.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// createReverseProxyHandler creates a reverse proxy handler for the given load balancer.
func createReverseProxyHandler(lb *RoundRobinLoadBalancer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the next server from the load balancer
		target := lb.NextServer()

		// Create a reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(target)

		// Update the request URL to the target URL
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = target.Host

		// Serve the request using the reverse proxy
		proxy.ServeHTTP(w, r)
	}
}
