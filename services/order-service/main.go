package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/lib/pq"
)

var db *sql.DB

// ==================================================
// Reliability configuration
// ==================================================

const (
	paymentTimeout     = 2 * time.Second
	maxPaymentAttempts = 3
	initialBackoff     = 100 * time.Millisecond

	// Circuit breaker configuration
	failureThreshold = 5
	openDuration     = 10 * time.Second
)

// ==================================================
// HTTP Client
// ==================================================

var paymentHTTPClient = &http.Client{
	Timeout: paymentTimeout,
}

// ==================================================
// Circuit Breaker
// ==================================================

type CircuitState string

const (
	CircuitClosed   CircuitState = "CLOSED"
	CircuitOpen     CircuitState = "OPEN"
	CircuitHalfOpen CircuitState = "HALF_OPEN"
)

type CircuitBreaker struct {
	mu sync.Mutex

	state CircuitState

	consecutiveFailures int

	openedAt time.Time
}

var paymentCircuitBreaker = &CircuitBreaker{
	state: CircuitClosed,
}

// --------------------------------------------------
// Check whether request is allowed
// --------------------------------------------------

func (cb *CircuitBreaker) AllowRequest() bool {

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {

	case CircuitClosed:
		return true

	case CircuitOpen:

		if time.Since(cb.openedAt) >= openDuration {

			log.Println(
				"Circuit Breaker transitioning OPEN -> HALF_OPEN",
			)

			cb.state = CircuitHalfOpen

			return true
		}

		return false

	case CircuitHalfOpen:

		return true
	}

	return false
}

// --------------------------------------------------
// Record successful request
// --------------------------------------------------

func (cb *CircuitBreaker) RecordSuccess() {

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != CircuitClosed {

		log.Printf(
			"Circuit Breaker transitioning %s -> CLOSED",
			cb.state,
		)
	}

	cb.state = CircuitClosed
	cb.consecutiveFailures = 0
}

// --------------------------------------------------
// Record failed request
// --------------------------------------------------

func (cb *CircuitBreaker) RecordFailure() {

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++

	log.Printf(
		"Circuit Breaker failure count: %d/%d",
		cb.consecutiveFailures,
		failureThreshold,
	)

	// HALF_OPEN request failed.
	if cb.state == CircuitHalfOpen {

		cb.state = CircuitOpen
		cb.openedAt = time.Now()

		log.Println(
			"Circuit Breaker transitioning HALF_OPEN -> OPEN",
		)

		return
	}

	// CLOSED reached failure threshold.
	if cb.state == CircuitClosed &&
		cb.consecutiveFailures >= failureThreshold {

		cb.state = CircuitOpen
		cb.openedAt = time.Now()

		log.Println(
			"Circuit Breaker transitioning CLOSED -> OPEN",
		)
	}
}

// --------------------------------------------------
// Get current state
// --------------------------------------------------

func (cb *CircuitBreaker) GetState() CircuitState {

	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state
}

// ==================================================
// Prometheus Metrics
// ==================================================

var (
	orderRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "order_requests_total",
			Help: "Total number of order requests received.",
		},
	)

	orderFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "order_failures_total",
			Help: "Total number of failed order requests.",
		},
	)

	orderDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "order_request_duration_seconds",
			Help: "Time spent processing order requests.",
		},
	)

	paymentRetryAttempts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_retry_attempts_total",
			Help: "Total number of retry attempts made when calling Payment Service.",
		},
	)

	circuitBreakerRejections = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_circuit_breaker_rejections_total",
			Help: "Total number of requests rejected by the Payment circuit breaker.",
		},
	)
)

func init() {

	prometheus.MustRegister(orderRequests)
	prometheus.MustRegister(orderFailures)
	prometheus.MustRegister(orderDuration)
	prometheus.MustRegister(paymentRetryAttempts)
	prometheus.MustRegister(circuitBreakerRejections)
}

// ==================================================
// Data structures
// ==================================================

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type OrderRequest struct {
	Product string  `json:"product"`
	Amount  float64 `json:"amount"`
}

type PaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type OrderResponse struct {
	OrderID       string          `json:"order_id"`
	Product       string          `json:"product"`
	Amount        float64         `json:"amount"`
	PaymentStatus string          `json:"payment_status"`
	PaymentResult json.RawMessage `json:"payment_response"`
}

// ==================================================
// Health - Liveness
// ==================================================

func livenessHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	response := HealthResponse{
		Status:  "alive",
		Service: "order-service",
	}

	json.NewEncoder(w).Encode(response)
}

// ==================================================
// Health - Readiness
// ==================================================

func readinessHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	// If database is unavailable, the service is not ready.
	if db == nil {

		w.WriteHeader(http.StatusServiceUnavailable)

		json.NewEncoder(w).Encode(
			HealthResponse{
				Status:  "not_ready",
				Service: "order-service",
			},
		)

		return
	}

	if err := db.Ping(); err != nil {

		log.Printf(
			"Readiness check failed: %v",
			err,
		)

		w.WriteHeader(http.StatusServiceUnavailable)

		json.NewEncoder(w).Encode(
			HealthResponse{
				Status:  "not_ready",
				Service: "order-service",
			},
		)

		return
	}

	response := HealthResponse{
		Status:  "ready",
		Service: "order-service",
	}

	json.NewEncoder(w).Encode(response)
}

// ==================================================
// Legacy Health Endpoint
// ==================================================

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
		Service: "order-service",
	}

	json.NewEncoder(w).Encode(response)
}

// ==================================================
// Database
// ==================================================

func createOrdersTable() error {

	query := `
	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		order_id VARCHAR(100) UNIQUE NOT NULL,
		product VARCHAR(255) NOT NULL,
		amount DECIMAL(10,2) NOT NULL,
		payment_status VARCHAR(50) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`

	_, err := db.Exec(query)

	return err
}

// ==================================================
// Payment Service
// ==================================================

func callPaymentService(
	paymentRequest PaymentRequest,
) ([]byte, error) {

	// --------------------------------------------------
	// Circuit breaker check
	// --------------------------------------------------

	if !paymentCircuitBreaker.AllowRequest() {

		circuitBreakerRejections.Inc()

		log.Println(
			"Payment circuit breaker is OPEN - request rejected",
		)

		return nil, fmt.Errorf(
			"payment service circuit breaker is OPEN",
		)
	}

	paymentBaseURL := os.Getenv("PAYMENT_SERVICE_URL")

	if paymentBaseURL == "" {
		paymentBaseURL = "http://payment-service:8083"
	}

	paymentURL := strings.TrimRight(paymentBaseURL, "/") + "/payments"
	body, err := json.Marshal(paymentRequest)

	if err != nil {

		return nil, fmt.Errorf(
			"failed to marshal payment request: %w",
			err,
		)
	}

	var lastErr error

	// --------------------------------------------------
	// Retry loop
	// --------------------------------------------------

	for attempt := 1; attempt <= maxPaymentAttempts; attempt++ {

		log.Printf(
			"Payment Service attempt %d/%d",
			attempt,
			maxPaymentAttempts,
		)

		requestBody := bytes.NewReader(body)

		response, err := paymentHTTPClient.Post(
			paymentURL,
			"application/json",
			requestBody,
		)

		// --------------------------------------------------
		// Network / timeout failure
		// --------------------------------------------------

		if err != nil {

			lastErr = err

			log.Printf(
				"Payment Service attempt %d failed: %v",
				attempt,
				err,
			)

		} else {

			// --------------------------------------------------
			// Read response
			// --------------------------------------------------

			paymentResponse, readErr := io.ReadAll(
				response.Body,
			)

			response.Body.Close()

			if readErr != nil {

				lastErr = readErr

			} else {

				// --------------------------------------------------
				// Success
				// --------------------------------------------------

				if response.StatusCode == http.StatusOK {

					log.Printf(
						"Payment Service succeeded on attempt %d",
						attempt,
					)

					paymentCircuitBreaker.RecordSuccess()

					return paymentResponse, nil
				}

				// --------------------------------------------------
				// 4xx = don't retry
				// --------------------------------------------------

				if response.StatusCode >= 400 &&
					response.StatusCode < 500 {

					return nil, fmt.Errorf(
						"payment request rejected with HTTP %d: %s",
						response.StatusCode,
						string(paymentResponse),
					)
				}

				// --------------------------------------------------
				// 5xx = retry
				// --------------------------------------------------

				lastErr = fmt.Errorf(
					"payment service returned HTTP %d: %s",
					response.StatusCode,
					string(paymentResponse),
				)
			}
		}

		// --------------------------------------------------
		// Final attempt
		// --------------------------------------------------

		if attempt == maxPaymentAttempts {
			break
		}

		// --------------------------------------------------
		// Record retry
		// --------------------------------------------------

		paymentRetryAttempts.Inc()

		backoff := initialBackoff *
			time.Duration(1<<(attempt-1))

		log.Printf(
			"Waiting %v before retry",
			backoff,
		)

		time.Sleep(backoff)
	}

	// --------------------------------------------------
	// All retries failed
	// --------------------------------------------------

	paymentCircuitBreaker.RecordFailure()

	return nil, fmt.Errorf(
		"payment service failed after %d attempts: %w",
		maxPaymentAttempts,
		lastErr,
	)
}

// ==================================================
// Create Order
// ==================================================

func createOrderHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

	start := time.Now()

	orderRequests.Inc()

	defer orderDuration.Observe(
		time.Since(start).Seconds(),
	)

	// --------------------------------------------------
	// HTTP method
	// --------------------------------------------------

	if r.Method != http.MethodPost {

		orderFailures.Inc()

		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	// --------------------------------------------------
	// Parse request
	// --------------------------------------------------

	var order OrderRequest

	if err := json.NewDecoder(
		r.Body,
	).Decode(&order); err != nil {

		orderFailures.Inc()

		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)

		return
	}

	// --------------------------------------------------
	// Generate unique Order ID
	// --------------------------------------------------

	orderID := fmt.Sprintf(
		"ORD-%d",
		time.Now().UnixNano(),
	)

	// --------------------------------------------------
	// Payment
	// --------------------------------------------------

	paymentRequest := PaymentRequest{
		OrderID: orderID,
		Amount:  order.Amount,
	}

	paymentResponse, err := callPaymentService(
		paymentRequest,
	)

	if err != nil {

		orderFailures.Inc()

		log.Printf(
			"Payment Service failed: %v",
			err,
		)

		if paymentCircuitBreaker.GetState() ==
			CircuitOpen {

			http.Error(
				w,
				"payment service temporarily unavailable",
				http.StatusServiceUnavailable,
			)

			return
		}

		http.Error(
			w,
			"payment service unavailable",
			http.StatusServiceUnavailable,
		)

		return
	}

	// --------------------------------------------------
	// Save order
	// --------------------------------------------------

	_, err = db.Exec(`
		INSERT INTO orders
		(order_id, product, amount, payment_status)
		VALUES ($1, $2, $3, $4)
	`,
		orderID,
		order.Product,
		order.Amount,
		"successful",
	)

	if err != nil {

		orderFailures.Inc()

		log.Printf(
			"Failed to save order: %v",
			err,
		)

		http.Error(
			w,
			"failed to save order",
			http.StatusInternalServerError,
		)

		return
	}

	// --------------------------------------------------
	// Response
	// --------------------------------------------------

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	result := OrderResponse{
		OrderID:       orderID,
		Product:       order.Product,
		Amount:        order.Amount,
		PaymentStatus: "successful",
		PaymentResult: json.RawMessage(paymentResponse),
	}

	json.NewEncoder(w).Encode(result)
}

// ==================================================
// PostgreSQL connection
// ==================================================

func connectDatabase() {

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		dsn = "host=postgres port=5432 user=admin password=password dbname=selfhealing sslmode=disable"
	}
	var err error

	db, err = sql.Open(
		"postgres",
		dsn,
	)

	if err != nil {

		log.Fatal(
			"Failed to open database:",
			err,
		)
	}

	if err := db.Ping(); err != nil {

		log.Fatal(
			"Failed to connect to PostgreSQL:",
			err,
		)
	}

	log.Println(
		"Connected to PostgreSQL",
	)

	if err := createOrdersTable(); err != nil {

		log.Fatal(
			"Failed to create orders table:",
			err,
		)
	}

	log.Println(
		"Orders table ready",
	)
}

// ==================================================
// Main - Graceful Shutdown
// ==================================================

func main() {

	connectDatabase()

	// --------------------------------------------------
	// HTTP routes
	// --------------------------------------------------

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/health",
		healthHandler,
	)

	mux.HandleFunc(
		"/health/live",
		livenessHandler,
	)

	mux.HandleFunc(
		"/health/ready",
		readinessHandler,
	)

	mux.HandleFunc(
		"/orders",
		createOrderHandler,
	)

	mux.Handle(
		"/metrics",
		promhttp.Handler(),
	)

	// --------------------------------------------------
	// HTTP Server
	// --------------------------------------------------

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,

		// Prevent connections from hanging forever.
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --------------------------------------------------
	// Start server
	// --------------------------------------------------

	go func() {

		log.Printf("Order Service listening on :%s", port)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			log.Fatalf(
				"HTTP server failed: %v",
				err,
			)
		}

	}()

	// --------------------------------------------------
	// Listen for shutdown signals
	// --------------------------------------------------

	stop := make(chan os.Signal, 1)

	signal.Notify(
		stop,
		os.Interrupt,
		syscall.SIGTERM,
	)

	// Wait until shutdown signal arrives.
	<-stop

	log.Println(
		"Shutdown signal received",
	)

	// --------------------------------------------------
	// Graceful shutdown
	// --------------------------------------------------

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	log.Println(
		"Stopping HTTP server gracefully...",
	)

	if err := server.Shutdown(
		shutdownContext,
	); err != nil {

		log.Printf(
			"Graceful shutdown failed: %v",
			err,
		)

	} else {

		log.Println(
			"HTTP server stopped gracefully",
		)
	}

	// --------------------------------------------------
	// Close database
	// --------------------------------------------------

	if db != nil {

		log.Println(
			"Closing PostgreSQL connection...",
		)

		if err := db.Close(); err != nil {

			log.Printf(
				"Failed to close database: %v",
				err,
			)

		} else {

			log.Println(
				"PostgreSQL connection closed",
			)
		}
	}

	log.Println(
		"Order Service shutdown complete",
	)
}
