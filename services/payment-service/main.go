package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ==================================================
// Global Database
// ==================================================

var db *sql.DB

// ==================================================
// Fault Injection
// ==================================================

var faultLatency time.Duration

// ==================================================
// Data Structures
// ==================================================

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type PaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type PaymentResponse struct {
	Status  string  `json:"status"`
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

// ==================================================
// Prometheus Metrics
// ==================================================

var (
	paymentRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_requests_total",
			Help: "Total number of payment requests received.",
		},
	)

	paymentFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_failures_total",
			Help: "Total number of failed payment requests.",
		},
	)

	paymentDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "payment_request_duration_seconds",
			Help: "Time spent processing payment requests.",
		},
	)
)

func init() {
	prometheus.MustRegister(paymentRequests)
	prometheus.MustRegister(paymentFailures)
	prometheus.MustRegister(paymentDuration)
}

// ==================================================
// Database Connection
// ==================================================

func connectDatabase() error {

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		dsn = "host=postgres port=5432 user=admin password=password dbname=selfhealing sslmode=disable"
	}
	var err error

	db, err = sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	log.Println("Connected to PostgreSQL")

	return nil
}

// ==================================================
// Payments Table
// ==================================================

func createPaymentsTable() error {

	query := `
	CREATE TABLE IF NOT EXISTS payments (
		id SERIAL PRIMARY KEY,
		order_id VARCHAR(100) UNIQUE NOT NULL,
		amount DECIMAL(10,2) NOT NULL,
		status VARCHAR(50) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`

	_, err := db.Exec(query)

	return err
}

// ==================================================
// Health
// ==================================================
var serviceUnavailable bool
var unknownFault bool

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

	status := "healthy"

	if serviceUnavailable {
		status = "unhealthy"
	}

	response := HealthResponse{
		Status:  status,
		Service: "payment-service",
	}

	json.NewEncoder(w).Encode(response)
}

func unavailableFaultHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	serviceUnavailable = true

	log.Printf(
		"FAULT INJECTION: Payment Service marked unavailable",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "fault_injected",
		"type":    "unavailable",
		"service": "payment-service",
	})
}
func unknownFaultHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	unknownFault = true

	log.Printf(
		"FAULT INJECTION: Payment Service unknown fault activated",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "fault_injected",
		"type":    "unknown",
		"service": "payment-service",
	})
}

func clearUnknownFaultHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	unknownFault = false

	log.Printf(
		"FAULT INJECTION: Payment Service unknown fault cleared",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "fault_cleared",
		"type":    "unknown",
		"service": "payment-service",
	})
}

func clearUnavailableFaultHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	serviceUnavailable = false

	log.Printf(
		"FAULT INJECTION: Payment Service availability restored",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "fault_cleared",
		"type":    "unavailable",
		"service": "payment-service",
	})
}

func restartHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	log.Printf(
		"🛠️ CONTROLLED RESTART | Payment Service restart requested",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]string{
		"status": "restart_requested",
	})

	// Give the HTTP response a moment to reach the caller
	// before terminating the process.
	go func() {
		time.Sleep(500 * time.Millisecond)

		log.Printf(
			"🔄 CONTROLLED RESTART | Payment Service shutting down",
		)

		os.Exit(0)
	}()
}

// ==================================================
// Fault Injection - Latency
// ==================================================

func latencyFaultHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {

		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	seconds := r.URL.Query().Get("seconds")

	if seconds == "" {

		http.Error(
			w,
			"seconds parameter is required",
			http.StatusBadRequest,
		)

		return
	}

	var duration int

	if _, err := fmt.Sscanf(seconds, "%d", &duration); err != nil {

		http.Error(
			w,
			"invalid seconds value",
			http.StatusBadRequest,
		)

		return
	}

	if duration < 0 || duration > 30 {

		http.Error(
			w,
			"seconds must be between 0 and 30",
			http.StatusBadRequest,
		)

		return
	}

	faultLatency = time.Duration(duration) * time.Second

	log.Printf(
		"FAULT INJECTION: Payment latency set to %d seconds",
		duration,
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "fault_injected",
		"type":            "latency",
		"latency_seconds": duration,
	})
}

// ==================================================
// Payment Handler
// ==================================================

func paymentHandler(w http.ResponseWriter, r *http.Request) {

	start := time.Now()

	paymentRequests.Inc()

	defer paymentDuration.Observe(
		time.Since(start).Seconds(),
	)

	// --------------------------------------------------
	// HTTP Method
	// --------------------------------------------------

	if r.Method != http.MethodPost {

		paymentFailures.Inc()

		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}
	if serviceUnavailable {

		paymentFailures.Inc()

		log.Printf(
			"FAULT INJECTION ACTIVE: Payment Service unavailable",
		)

		http.Error(
			w,
			"payment service unavailable",
			http.StatusServiceUnavailable,
		)

		return
	}
	if unknownFault {

		paymentFailures.Inc()

		log.Printf(
			"FAULT INJECTION ACTIVE: Unknown Payment Service failure",
		)

		http.Error(
			w,
			"unexpected payment processing failure",
			http.StatusInternalServerError,
		)

		return
	}

	// --------------------------------------------------
	// Parse Request
	// --------------------------------------------------

	var request PaymentRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {

		paymentFailures.Inc()

		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)

		return
	}

	// --------------------------------------------------
	// Validate Order ID
	// --------------------------------------------------

	if request.OrderID == "" {

		paymentFailures.Inc()

		http.Error(
			w,
			"order_id is required",
			http.StatusBadRequest,
		)

		return
	}

	// --------------------------------------------------
	// Injected Latency
	// --------------------------------------------------

	if faultLatency > 0 {

		log.Printf(
			"FAULT INJECTION ACTIVE: sleeping for %v",
			faultLatency,
		)

		time.Sleep(faultLatency)
	}

	// --------------------------------------------------
	// Check Existing Payment
	// --------------------------------------------------

	var existingStatus string
	var existingAmount float64

	err := db.QueryRow(
		`
		SELECT status, amount
		FROM payments
		WHERE order_id = $1
		`,
		request.OrderID,
	).Scan(
		&existingStatus,
		&existingAmount,
	)

	if err == nil {

		log.Printf(
			"Idempotent payment request: order_id=%s already processed",
			request.OrderID,
		)

		response := PaymentResponse{
			Status:  existingStatus,
			OrderID: request.OrderID,
			Amount:  existingAmount,
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		json.NewEncoder(w).Encode(response)

		return
	}

	if err != sql.ErrNoRows {

		paymentFailures.Inc()

		log.Printf(
			"Failed checking existing payment: %v",
			err,
		)

		http.Error(
			w,
			"database error",
			http.StatusInternalServerError,
		)

		return
	}

	// --------------------------------------------------
	// Process Payment
	// --------------------------------------------------

	log.Printf(
		"Processing new payment: order_id=%s amount=%.2f",
		request.OrderID,
		request.Amount,
	)

	paymentStatus := "payment_success"

	// --------------------------------------------------
	// Store Payment
	// --------------------------------------------------

	_, err = db.Exec(
		`
		INSERT INTO payments
		(order_id, amount, status)
		VALUES ($1, $2, $3)
		`,
		request.OrderID,
		request.Amount,
		paymentStatus,
	)

	if err != nil {

		paymentFailures.Inc()

		log.Printf(
			"Failed to store payment: %v",
			err,
		)

		http.Error(
			w,
			"failed to process payment",
			http.StatusInternalServerError,
		)

		return
	}

	// --------------------------------------------------
	// Return Success
	// --------------------------------------------------

	response := PaymentResponse{
		Status:  paymentStatus,
		OrderID: request.OrderID,
		Amount:  request.Amount,
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(response)

	log.Printf(
		"Payment successfully processed: order_id=%s",
		request.OrderID,
	)
}
func faultStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":            "latency",
		"latency_seconds": faultLatency.Seconds(),
	})
}

// ==================================================
// Main
// ==================================================

func main() {

	// --------------------------------------------------
	// Database
	// --------------------------------------------------

	if err := connectDatabase(); err != nil {

		log.Fatalf(
			"Failed to connect to PostgreSQL: %v",
			err,
		)
	}

	defer func() {

		log.Println("Closing PostgreSQL connection...")

		if err := db.Close(); err != nil {
			log.Printf(
				"Error closing PostgreSQL: %v",
				err,
			)
		}

		log.Println("PostgreSQL connection closed")
	}()

	// --------------------------------------------------
	// Create Table
	// --------------------------------------------------

	if err := createPaymentsTable(); err != nil {

		log.Fatalf(
			"Failed to create payments table: %v",
			err,
		)
	}

	log.Println("Payments table ready")

	// --------------------------------------------------
	// Routes
	// --------------------------------------------------

	http.HandleFunc(
		"/health",
		healthHandler,
	)

	http.HandleFunc(
		"/payments",
		paymentHandler,
	)

	http.HandleFunc(
		"/fault/latency",
		latencyFaultHandler,
	)

	http.HandleFunc(
		"/fault",
		faultStatusHandler,
	)

	http.HandleFunc(
		"/fault/unavailable",
		unavailableFaultHandler,
	)

	http.HandleFunc(
		"/fault/unavailable/clear",
		clearUnavailableFaultHandler,
	)
	http.HandleFunc(
		"/fault/unknown",
		unknownFaultHandler,
	)
	http.HandleFunc(
		"/fault/unknown/clear",
		clearUnknownFaultHandler,
	)

	// Controlled self-healing endpoint
	http.HandleFunc(
		"/admin/restart",
		restartHandler,
	)

	// Prometheus metrics
	http.Handle(
		"/metrics",
		promhttp.Handler(),
	)
	// --------------------------------------------------
	// Start Server
	// --------------------------------------------------

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	addr := ":" + port

	log.Printf("Payment Service listening on :%s", port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
