package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --------------------------------------------------
// Prometheus Metrics
// --------------------------------------------------

var (
	userRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_requests_total",
			Help: "Total number of user service requests received.",
		},
	)

	userFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_failures_total",
			Help: "Total number of failed user service requests.",
		},
	)

	userDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "user_request_duration_seconds",
			Help: "Time spent processing user service requests.",
		},
	)
)

// Register Prometheus metrics
func init() {
	prometheus.MustRegister(userRequests)
	prometheus.MustRegister(userFailures)
	prometheus.MustRegister(userDuration)
}

// --------------------------------------------------
// Response Structure
// --------------------------------------------------

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// --------------------------------------------------
// Health Endpoint
// --------------------------------------------------

func healthHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set(
		"Access-Control-Allow-Origin",
		"http://localhost:5173",
	)

	w.Header().Set(
		"Access-Control-Allow-Methods",
		"GET, OPTIONS",
	)

	w.Header().Set(
		"Access-Control-Allow-Headers",
		"Content-Type",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	response := HealthResponse{
		Status:  "healthy",
		Service: "user-service",
	}

	json.NewEncoder(w).Encode(response)
}

// --------------------------------------------------
// Main
// --------------------------------------------------

func main() {

	// Health endpoint
	http.HandleFunc("/health", healthHandler)

	// Prometheus metrics endpoint
	http.Handle(
		"/metrics",
		promhttp.Handler(),
	)

	// --------------------------------------------------
	// Metrics middleware
	// --------------------------------------------------

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		userRequests.Inc()

		defer userDuration.Observe(
			time.Since(start).Seconds(),
		)

		if r.URL.Path != "/" {
			userFailures.Inc()

			http.NotFound(
				w,
				r,
			)

			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		response := map[string]string{
			"service": "user-service",
			"status":  "running",
		}

		json.NewEncoder(w).Encode(response)
	})

	// --------------------------------------------------
	// Start server
	// --------------------------------------------------

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	addr := ":" + port

	log.Printf("User Service listening on :%s", port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
