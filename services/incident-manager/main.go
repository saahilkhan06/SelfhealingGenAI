package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const prometheusURL = "http://prometheus:9090"

const postgresURL = "postgres://admin:password@postgres:5432/selfhealing"

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type Incident struct {
	ID                  string    `json:"id"`
	Service             string    `json:"service"`
	Type                string    `json:"type"`
	Severity            string    `json:"severity"`
	Description         string    `json:"description"`
	Value               float64   `json:"value"`
	DetectedAt          time.Time `json:"detected_at"`
	Status              string    `json:"status"`
	AIRootCause         string    `json:"ai_root_cause"`
	AIConfidence        string    `json:"ai_confidence"`
	AIRecommendedAction string    `json:"ai_recommended_action"`
	AIReason            string    `json:"ai_reason"`
}
type ServiceHealth struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type IncidentEvidence struct {
	Incident                   Incident      `json:"incident"`
	OrderFailuresLastMinute    float64       `json:"order_failures_last_minute"`
	OrderServiceHealth         ServiceHealth `json:"order_service_health"`
	PaymentServiceHealth       ServiceHealth `json:"payment_service_health"`
	PaymentFault               FaultStatus   `json:"payment_fault"`
	OrderPaymentTimeoutSeconds float64       `json:"order_payment_timeout_seconds"`
	CollectedAt                time.Time     `json:"collected_at"`
}
type AIDiagnosis struct {
	RootCause         string `json:"root_cause"`
	Confidence        string `json:"confidence"`
	RecommendedAction string `json:"recommended_action"`
	Reason            string `json:"reason"`
}
type HealingAction struct {
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Approved bool   `json:"approved"`
}

type HealingActionRecord struct {
	ID                 int       `json:"id"`
	IncidentID         string    `json:"incident_id"`
	Action             string    `json:"action"`
	Reason             string    `json:"reason"`
	Approved           bool      `json:"approved"`
	ExecutionStatus    string    `json:"execution_status"`
	VerificationStatus string    `json:"verification_status"`
	CreatedAt          time.Time `json:"created_at"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}
type FaultStatus struct {
	Type           string  `json:"type"`
	LatencySeconds float64 `json:"latency_seconds"`
}

var (
	incidents   []Incident
	incidentsMu sync.RWMutex
)
var failureAlertActive bool

func testDatabaseConnection() {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf("❌ DATABASE | connection failed: %v", err)
		return
	}

	defer conn.Close(ctx)

	var result int

	err = conn.QueryRow(
		ctx,
		"SELECT 1",
	).Scan(&result)

	if err != nil {
		log.Printf("❌ DATABASE | query failed: %v", err)
		return
	}

	log.Printf(
		"✅ DATABASE | PostgreSQL connected successfully | result=%d",
		result,
	)
}

func diagnoseIncident(evidence IncidentEvidence) (AIDiagnosis, error) {
	diagnosisHint := "unknown"

	if evidence.PaymentServiceHealth.Status != "healthy" {
		diagnosisHint = "payment_unavailable"
	} else if evidence.PaymentFault.LatencySeconds >
		evidence.OrderPaymentTimeoutSeconds {
		diagnosisHint = "payment_latency"
	}
	prompt := fmt.Sprintf(`
You are an SRE incident diagnosis assistant.

Analyze the production incident using ONLY the evidence provided below.

Incident:
Service: %s
Type: %s
Severity: %s
Description: %s

Evidence:
Order failures in last minute: %.2f
Order Service health: %s
Payment Service health: %s
Payment fault type: %s
Payment latency: %.2f seconds
Order-to-Payment timeout: %.2f seconds
Diagnosis classification: %s

Diagnosis classification is authoritative.

If the classification is "payment_unavailable", the root cause MUST identify
Payment Service unavailability or dependency failure. Do not describe it as
payment latency.

If the classification is "payment_latency", the root cause MUST identify
Payment Service latency exceeding the Order-to-Payment timeout.

If the classification is "unknown", determine the most likely root cause
using the available evidence.

Determine the most likely root cause based on the evidence and classification.
Return ONLY valid JSON in this exact format:

{
  "root_cause": "short explanation",
  "confidence": "high|medium|low",
  "recommended_action": "safe recommended recovery action",
  "reason": "short explanation based on evidence"
}
`, evidence.Incident.Service,
		evidence.Incident.Type,
		evidence.Incident.Severity,
		evidence.Incident.Description,
		evidence.OrderFailuresLastMinute,
		evidence.OrderServiceHealth.Status,
		evidence.PaymentServiceHealth.Status,
		evidence.PaymentFault.Type,
		evidence.PaymentFault.LatencySeconds,
		evidence.OrderPaymentTimeoutSeconds,
		diagnosisHint,
	)

	requestBody := map[string]interface{}{
		"model":  "qwen2.5:1.5b",
		"prompt": prompt,
		"stream": false,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return AIDiagnosis{}, err
	}

	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	resp, err := client.Post(
		"http://host.docker.internal:11434/api/generate",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return AIDiagnosis{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AIDiagnosis{}, fmt.Errorf(
			"ollama returned HTTP %d",
			resp.StatusCode,
		)
	}

	var ollamaResult OllamaResponse

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResult); err != nil {
		return AIDiagnosis{}, err
	}

	var diagnosis AIDiagnosis

	responseText := strings.TrimSpace(ollamaResult.Response)

	// Remove Markdown code fences if the model adds them.
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")

	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal(
		[]byte(responseText),
		&diagnosis,
	); err != nil {
		return AIDiagnosis{}, fmt.Errorf(
			"invalid AI diagnosis: %w; raw response: %s",
			err,
			ollamaResult.Response,
		)
	}

	return diagnosis, nil
}

func queryPrometheus(query string) (float64, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	apiURL := prometheusURL + "/api/v1/query?query=" + url.QueryEscape(query)

	resp, err := client.Get(apiURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned HTTP %d", resp.StatusCode)
	}

	var result PrometheusResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Status != "success" {
		return 0, fmt.Errorf("prometheus query failed")
	}

	if len(result.Data.Result) == 0 {
		return 0, nil
	}

	if len(result.Data.Result[0].Value) < 2 {
		return 0, nil
	}

	valueString, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected Prometheus value format")
	}

	value, err := strconv.ParseFloat(valueString, 64)
	if err != nil {
		return 0, err
	}

	return value, nil
}
func checkServiceHealth(serviceName, serviceURL string) ServiceHealth {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(serviceURL + "/health")

	if err != nil {
		return ServiceHealth{
			Service: serviceName,
			Status:  "unhealthy",
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServiceHealth{
			Service: serviceName,
			Status:  "unhealthy",
		}
	}

	var result ServiceHealth

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ServiceHealth{
			Service: serviceName,
			Status:  "unknown",
		}
	}

	return result
}

func collectEvidence(incident Incident) IncidentEvidence {
	incident.Status = "diagnosing"

	orderFailures, err := queryPrometheus(
		`increase(order_failures_total[1m])`,
	)

	if err != nil {
		log.Printf("Evidence metric query failed: %v", err)
	}

	orderHealth := checkServiceHealth(
		"order-service",
		"http://order-service:8082",
	)

	paymentHealth := checkServiceHealth(
		"payment-service",
		"http://payment-service:8083",
	)

	paymentFault := checkPaymentFault()

	evidence := IncidentEvidence{
		Incident:                   incident,
		OrderFailuresLastMinute:    orderFailures,
		OrderServiceHealth:         orderHealth,
		PaymentServiceHealth:       paymentHealth,
		CollectedAt:                time.Now(),
		OrderPaymentTimeoutSeconds: 2.0,
		PaymentFault:               paymentFault,
	}

	log.Printf(
		"Evidence collected | order_failures=%.2f | order_health=%s | payment_health=%s | payment_latency=%.2fs",
		orderFailures,
		orderHealth.Status,
		paymentHealth.Status,
		paymentFault.LatencySeconds,
	)

	// AI diagnosis
	diagnosis, err := diagnoseIncident(evidence)

	if err != nil {
		log.Printf("AI diagnosis failed: %v", err)

		evidence.Incident.Status = "escalated"

		return evidence
	}

	log.Printf(
		"🤖 AI DIAGNOSIS | root_cause=%s | confidence=%s | recommended_action=%s",
		diagnosis.RootCause,
		diagnosis.Confidence,
		diagnosis.RecommendedAction,
	)

	log.Printf(
		"🤖 AI REASON | %s",
		diagnosis.Reason,
	)

	// Store AI diagnosis inside the incident.
	evidence.Incident.AIRootCause = diagnosis.RootCause
	evidence.Incident.AIConfidence = diagnosis.Confidence
	evidence.Incident.AIRecommendedAction = diagnosis.RecommendedAction
	evidence.Incident.AIReason = diagnosis.Reason

	saveAIDiagnosis(
		evidence.Incident.ID,
		diagnosis,
	)

	// Healing policy
	evidence.Incident.Status = "healing"

	healingAction := determineHealingAction(
		evidence,
		diagnosis,
	)

	log.Printf(
		"🛠️ HEALING POLICY | action=%s | approved=%t | reason=%s",
		healingAction.Action,
		healingAction.Approved,
		healingAction.Reason,
	)

	saveHealingAction(
		incident.ID,
		healingAction,
		"not_executed",
		"not_started",
	)

	// Safety gate:
	// Never execute an action that was not approved
	// by the deterministic safety policy.
	if !healingAction.Approved {

		evidence.Incident.Status = "escalated"

		log.Printf(
			"🔴 INCIDENT | status=escalated | action=%s | reason=%s",
			healingAction.Action,
			healingAction.Reason,
		)

		updateHealingAction(
			incident.ID,
			healingAction.Action,
			"not_executed",
			"not_started",
		)

		return evidence
	}

	// Execute recovery only after policy approval.
	recoverySuccessful := executeHealingAction(
		healingAction,
	)

	if recoverySuccessful {

		log.Printf(
			"🔄 RECOVERY | action=%s | status=executed",
			healingAction.Action,
		)

		updateHealingAction(
			incident.ID,
			healingAction.Action,
			"executed",
			"not_started",
		)

		time.Sleep(2 * time.Second)

		// Verify recovery
		evidence.Incident.Status = "verifying"

		recoveryVerified := verifyRecovery(
			healingAction.Action,
		)

		if recoveryVerified {

			evidence.Incident.Status = "resolved"

			log.Printf(
				"🟢 SELF-HEALING | action=%s | status=verified",
				healingAction.Action,
			)

			updateHealingAction(
				incident.ID,
				healingAction.Action,
				"executed",
				"verified",
			)

		} else {

			evidence.Incident.Status = "escalated"

			log.Printf(
				"🔴 SELF-HEALING | action=%s | status=verification_failed",
				healingAction.Action,
			)

			updateHealingAction(
				incident.ID,
				healingAction.Action,
				"executed",
				"failed",
			)
		}

	} else {

		evidence.Incident.Status = "escalated"

		log.Printf(
			"🔴 INCIDENT | status=escalated | action=%s | recovery_failed",
			healingAction.Action,
		)

		updateHealingAction(
			incident.ID,
			healingAction.Action,
			"failed",
			"not_started",
		)
	}

	return evidence
}

func detectIncidents() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkPaymentFailures()
	}
}

func checkPaymentFailures() {
	query := `increase(order_failures_total[1m])`

	value, err := queryPrometheus(query)
	if err != nil {
		log.Printf("Prometheus query error: %v", err)
		return
	}

	log.Printf(
		"Monitoring order service: failures in last minute = %.0f",
		value,
	)

	paymentHealth := checkServiceHealth(
		"payment-service",
		"http://payment-service:8083",
	)

	if paymentHealth.Status != "healthy" {
		incidentsMu.Lock()

		paymentIncidentActive := false

		for _, existing := range incidents {
			if existing.Service == "payment-service" &&
				existing.Type == "service_unavailable" &&
				existing.Status != "resolved" &&
				existing.Status != "cancelled" {
				paymentIncidentActive = true
				break
			}
		}

		incidentsMu.Unlock()

		if !paymentIncidentActive {
			log.Printf(
				"PAYMENT SERVICE OUTAGE DETECTED | status=%s",
				paymentHealth.Status,
			)

			createIncident(Incident{
				ID: fmt.Sprintf(
					"INC-%d",
					time.Now().UnixNano(),
				),
				Service:     "payment-service",
				Type:        "service_unavailable",
				Severity:    "critical",
				Description: "Payment Service health check failed",
				Value:       1,
				DetectedAt:  time.Now(),
				Status:      "detected",
			})
		}

		return
	}

	const failureThreshold = 3.0

	// ------------------------------------------------------------
	// FAILURE CONDITION CLEARED
	// ------------------------------------------------------------
	//
	// Once the rolling Prometheus metric drops below the threshold,
	// allow the next genuine failure event to create a new incident.
	//
	if value < failureThreshold {
		incidentsMu.Lock()

		if failureAlertActive {
			log.Printf(
				"✅ FAILURE CONDITION CLEARED | failures=%.2f",
				value,
			)
		}

		failureAlertActive = false

		incidentsMu.Unlock()

		return
	}

	// ------------------------------------------------------------
	// FAILURE CONDITION ACTIVE
	// ------------------------------------------------------------

	incidentsMu.Lock()

	// The same unhealthy condition has already generated an incident.
	// Do not create another incident from the same rolling window.
	if failureAlertActive {
		incidentsMu.Unlock()

		log.Printf(
			"🛑 INCIDENT SUPPRESSION | failure condition already active | failures=%.2f",
			value,
		)

		return
	}

	// Lock the alert condition before creating the incident.
	// This prevents the monitoring loop from creating duplicates.
	failureAlertActive = true

	incidentsMu.Unlock()

	// ------------------------------------------------------------
	// CREATE NEW INCIDENT
	// ------------------------------------------------------------

	createIncident(Incident{
		ID: fmt.Sprintf(
			"INC-%d",
			time.Now().UnixNano(),
		),
		Service:     "order-service",
		Type:        "high_failure_rate",
		Severity:    "high",
		Description: "Order Service is experiencing repeated failures while communicating with Payment Service",
		Value:       value,
		DetectedAt:  time.Now(),
		Status:      "detected",
	})
}
func createIncident(incident Incident) {
	incidentsMu.Lock()

	// Prevent duplicate incidents every 30 seconds.
	if len(incidents) > 0 {
		last := incidents[len(incidents)-1]

		if last.Service == incident.Service &&
			last.Type == incident.Type &&
			time.Since(last.DetectedAt) < 30*time.Second {
			incidentsMu.Unlock()
			return
		}
	}

	incidents = append(incidents, incident)
	saveIncident(incident)

	incidentsMu.Unlock()

	log.Printf(
		"🚨 INCIDENT DETECTED | service=%s type=%s severity=%s value=%.0f",
		incident.Service,
		incident.Type,
		incident.Severity,
		incident.Value,
	)

	// Collect evidence and determine the final incident status.
	evidence := collectEvidence(incident)

	// Update the stored incident with the final status.
	incidentsMu.Lock()

	for i := range incidents {
		if incidents[i].ID == incident.ID {
			incidents[i].Status = evidence.Incident.Status
			break
		}
	}

	incidentsMu.Unlock()

	// Persist the final lifecycle status in PostgreSQL.
	updateIncidentStatus(
		incident.ID,
		evidence.Incident.Status,
	)

	log.Printf(
		"📋 INCIDENT LIFECYCLE | id=%s | final_status=%s",
		incident.ID,
		evidence.Incident.Status,
	)
}

func incidentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | incidents query connection failed: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"database connection failed"}`,
			http.StatusInternalServerError,
		)
		return
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(
		ctx,
		`SELECT
    id,
    service,
    type,
    severity,
    description,
    value,
    detected_at,
    status,
    COALESCE(ai_root_cause, '') AS ai_root_cause,
    COALESCE(ai_confidence, '') AS ai_confidence,
    COALESCE(ai_recommended_action, '') AS ai_recommended_action,
    COALESCE(ai_reason, '') AS ai_reason
FROM incidents
ORDER BY detected_at DESC`,
	)
	if err != nil {
		log.Printf(
			"❌ DATABASE | incidents query failed: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"failed to fetch incidents"}`,
			http.StatusInternalServerError,
		)
		return
	}
	defer rows.Close()

	type DatabaseIncident struct {
		ID                  string    `json:"id"`
		Service             string    `json:"service"`
		Type                string    `json:"type"`
		Severity            string    `json:"severity"`
		Description         string    `json:"description"`
		Value               float64   `json:"value"`
		DetectedAt          time.Time `json:"detected_at"`
		Status              string    `json:"status"`
		AIRootCause         string    `json:"ai_root_cause"`
		AIConfidence        string    `json:"ai_confidence"`
		AIRecommendedAction string    `json:"ai_recommended_action"`
		AIReason            string    `json:"ai_reason"`
	}

	var results []DatabaseIncident

	for rows.Next() {
		var incident DatabaseIncident

		err := rows.Scan(
			&incident.ID,
			&incident.Service,
			&incident.Type,
			&incident.Severity,
			&incident.Description,
			&incident.Value,
			&incident.DetectedAt,
			&incident.Status,
			&incident.AIRootCause,
			&incident.AIConfidence,
			&incident.AIRecommendedAction,
			&incident.AIReason,
		)

		if err != nil {
			log.Printf(
				"❌ DATABASE | incident row scan failed: %v",
				err,
			)

			http.Error(
				w,
				`{"error":"failed to read incidents"}`,
				http.StatusInternalServerError,
			)
			return
		}

		results = append(results, incident)
	}

	if err := rows.Err(); err != nil {
		log.Printf(
			"❌ DATABASE | incidents rows error: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"failed while reading incidents"}`,
			http.StatusInternalServerError,
		)
		return
	}

	if results == nil {
		results = []DatabaseIncident{}
	}

	json.NewEncoder(w).Encode(results)
}
func healingActionsHandler(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | healing actions query connection failed: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"database connection failed"}`,
			http.StatusInternalServerError,
		)
		return
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(
		ctx,
		`SELECT
			id,
			incident_id,
			action,
			reason,
			approved,
			execution_status,
			verification_status,
			created_at
		FROM healing_actions
		ORDER BY created_at DESC`,
	)

	if err != nil {
		log.Printf(
			"❌ DATABASE | healing actions query failed: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"failed to fetch healing actions"}`,
			http.StatusInternalServerError,
		)
		return
	}
	defer rows.Close()

	var results []HealingActionRecord

	for rows.Next() {
		var action HealingActionRecord

		err := rows.Scan(
			&action.ID,
			&action.IncidentID,
			&action.Action,
			&action.Reason,
			&action.Approved,
			&action.ExecutionStatus,
			&action.VerificationStatus,
			&action.CreatedAt,
		)

		if err != nil {
			log.Printf(
				"❌ DATABASE | healing action row scan failed: %v",
				err,
			)

			http.Error(
				w,
				`{"error":"failed to read healing actions"}`,
				http.StatusInternalServerError,
			)
			return
		}

		results = append(results, action)
	}

	if err := rows.Err(); err != nil {
		log.Printf(
			"❌ DATABASE | healing actions rows error: %v",
			err,
		)

		http.Error(
			w,
			`{"error":"failed while reading healing actions"}`,
			http.StatusInternalServerError,
		)
		return
	}

	if results == nil {
		results = []HealingActionRecord{}
	}

	json.NewEncoder(w).Encode(results)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"service": "incident-manager",
		"status":  "healthy",
	})
}
func checkPaymentFault() FaultStatus {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://payment-service:8083/fault")

	if err != nil {
		log.Printf("Payment fault status check failed: %v", err)

		return FaultStatus{
			Type:           "unknown",
			LatencySeconds: 0,
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FaultStatus{
			Type:           "unknown",
			LatencySeconds: 0,
		}
	}

	var result FaultStatus

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Payment fault status decode failed: %v", err)

		return FaultStatus{
			Type:           "unknown",
			LatencySeconds: 0,
		}
	}

	return result
}
func determineHealingAction(
	evidence IncidentEvidence,
	diagnosis AIDiagnosis,
) HealingAction {

	// Policy 1:
	// If Payment Service is healthy but its injected latency
	// exceeds the Order-to-Payment timeout, the fault can be
	// safely cleared automatically.
	if evidence.PaymentFault.LatencySeconds >
		evidence.OrderPaymentTimeoutSeconds &&
		evidence.PaymentServiceHealth.Status == "healthy" &&
		evidence.Incident.Type == "high_failure_rate" {

		return HealingAction{
			Action:   "CLEAR_PAYMENT_FAULT",
			Reason:   "Payment latency exceeds the Order-to-Payment timeout.",
			Approved: true,
		}
	}

	// Policy 2:
	// Approve a controlled restart when Payment Service is unhealthy
	// and either a high failure rate or a direct outage was detected.
	if evidence.PaymentServiceHealth.Status == "unhealthy" &&
		(evidence.Incident.Type == "high_failure_rate" ||
			(evidence.Incident.Service == "payment-service" &&
				evidence.Incident.Type == "service_unavailable")) {

		return HealingAction{
			Action:   "RESTART_PAYMENT_SERVICE",
			Reason:   "Payment Service is unhealthy; the deterministic policy approved a controlled restart.",
			Approved: true,
		}
	}

	return HealingAction{
		Action:   "NO_ACTION",
		Reason:   "No safe automated recovery policy matched the incident.",
		Approved: false,
	}
}


func restartPaymentContainer() error {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "tcp://docker-socket-proxy:2375"
	}

	dockerHost = strings.TrimPrefix(dockerHost, "tcp://")
	baseURL := "http://" + dockerHost

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Inspect the payment container.
	inspectURL := baseURL + "/containers/payment-service/json"

	resp, err := client.Get(inspectURL)
	if err != nil {
		return fmt.Errorf("inspect payment container: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("container inspection returned HTTP %d", resp.StatusCode)
	}

	var container struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}

	err = json.NewDecoder(resp.Body).Decode(&container)
	resp.Body.Close()

	if err != nil {
		return fmt.Errorf("decode container state: %w", err)
	}

	// Start a stopped container; restart a running one.
	endpoint := baseURL + "/containers/payment-service/start"
	if container.State.Running {
		endpoint = baseURL + "/containers/payment-service/restart?t=5"
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create recovery request: %w", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("Docker recovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("Docker recovery returned HTTP %d", resp.StatusCode)
	}

	log.Printf(
		"🔄 HEALING EXECUTOR | payment container recovery requested | running_before=%t",
		container.State.Running,
	)

	return nil
}


func executeHealingAction(action HealingAction) bool {
	if !action.Approved {
		log.Printf(
			"🛑 HEALING EXECUTOR | action=%s | skipped because action is not approved",
			action.Action,
		)
		return false
	}

	switch action.Action {

	case "CLEAR_PAYMENT_FAULT":

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		clearURL := "http://payment-service:8083/fault/latency?seconds=0"

		req, err := http.NewRequest(http.MethodPost, clearURL, nil)
		if err != nil {
			log.Printf(
				"❌ HEALING EXECUTOR | failed to create clear request: %v",
				err,
			)
			return false
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf(
				"❌ HEALING EXECUTOR | clear request failed: %v",
				err,
			)
			return false
		}

		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf(
				"❌ HEALING EXECUTOR | clear request failed | status=%d",
				resp.StatusCode,
			)
			return false
		}

		// Verify the fault state using the payment service's status endpoint.
		verifyResp, err := client.Get("http://payment-service:8083/fault")
		if err != nil {
			log.Printf(
				"❌ HEALING EXECUTOR | could not verify payment fault state: %v",
				err,
			)
			return false
		}
		defer verifyResp.Body.Close()

		if verifyResp.StatusCode != http.StatusOK {
			log.Printf(
				"❌ HEALING EXECUTOR | fault verification returned status=%d",
				verifyResp.StatusCode,
			)
			return false
		}

		var faultState struct {
			LatencySeconds float64 `json:"latency_seconds"`
		}

		if err := json.NewDecoder(verifyResp.Body).Decode(&faultState); err != nil {
			log.Printf(
				"❌ HEALING EXECUTOR | invalid fault verification response: %v",
				err,
			)
			return false
		}

		if faultState.LatencySeconds != 0 {
			log.Printf(
				"❌ HEALING EXECUTOR | latency remains active: %.2fs",
				faultState.LatencySeconds,
			)
			return false
		}

		log.Printf(
			"✅ HEALING EXECUTOR | latency cleared and verified",
		)

		return true
	case "RESTART_PAYMENT_SERVICE":

		if err := restartPaymentContainer(); err != nil {
			log.Printf(
				"❌ HEALING EXECUTOR | payment container recovery failed: %v",
				err,
			)
			return false
		}

		return true

	default:
		log.Printf(
			"🛑 HEALING EXECUTOR | unknown action=%s",
			action.Action,
		)
		return false
	}
}

func verifyRecovery(action string) bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	if action == "RESTART_PAYMENT_SERVICE" {
		log.Printf("🔍 RECOVERY VERIFICATION | waiting for Payment Service")

		const maxAttempts = 10

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			resp, err := client.Get("http://payment-service:8083/health")

			if err == nil {
				healthy := resp.StatusCode == http.StatusOK
				resp.Body.Close()

				if healthy {
					log.Printf(
						"✅ RECOVERY VERIFICATION | payment service healthy | attempt=%d",
						attempt,
					)
					return true
				}
			}

			log.Printf(
				"⏳ RECOVERY VERIFICATION | payment service not ready | attempt=%d/%d",
				attempt,
				maxAttempts,
			)

			time.Sleep(time.Second)
		}

		log.Printf("❌ RECOVERY VERIFICATION FAILED | reason=payment_service_not_recovered")
		return false
	}

	if action != "CLEAR_PAYMENT_FAULT" {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=unsupported_action | action=%s",
			action,
		)
		return false
	}

	// Check whether injected latency has been cleared.
	resp, err := client.Get("http://payment-service:8083/fault")
	if err != nil {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=fault_endpoint_unreachable | error=%v",
			err,
		)
		return false
	}

	if resp.StatusCode != http.StatusOK {
		status := resp.StatusCode
		resp.Body.Close()

		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=fault_endpoint_http_error | status=%d",
			status,
		)
		return false
	}

	var fault FaultStatus
	decodeErr := json.NewDecoder(resp.Body).Decode(&fault)
	resp.Body.Close()

	if decodeErr != nil {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=invalid_fault_response | error=%v",
			decodeErr,
		)
		return false
	}

	if fault.LatencySeconds > 0 {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=latency_still_active | latency=%.2fs",
			fault.LatencySeconds,
		)
		return false
	}

	// Check that Payment Service is healthy.
	healthResp, err := client.Get("http://payment-service:8083/health")
	if err != nil {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=health_endpoint_unreachable | error=%v",
			err,
		)
		return false
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode != http.StatusOK {
		log.Printf(
			"❌ RECOVERY VERIFICATION FAILED | reason=payment_service_unhealthy | status=%d",
			healthResp.StatusCode,
		)
		return false
	}

	log.Printf(
		"✅ RECOVERY VERIFICATION PASSED | action=CLEAR_PAYMENT_FAULT | latency=%.2fs | payment_service=healthy",
		fault.LatencySeconds,
	)

	return true
}
func saveAIDiagnosis(
	incidentID string,
	diagnosis AIDiagnosis,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | AI diagnosis connection failed: %v",
			err,
		)
		return
	}

	defer conn.Close(ctx)

	_, err = conn.Exec(
		ctx,
		`UPDATE incidents
		 SET ai_root_cause = $1,
		     ai_confidence = $2,
		     ai_recommended_action = $3,
		     ai_reason = $4
		 WHERE id = $5`,
		diagnosis.RootCause,
		diagnosis.Confidence,
		diagnosis.RecommendedAction,
		diagnosis.Reason,
		incidentID,
	)

	if err != nil {
		log.Printf(
			"❌ DATABASE | AI diagnosis update failed | incident=%s | error=%v",
			incidentID,
			err,
		)
		return
	}

	log.Printf(
		"💾 DATABASE | AI diagnosis saved | incident=%s | confidence=%s",
		incidentID,
		diagnosis.Confidence,
	)
}

func saveIncident(incident Incident) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf("❌ DATABASE | incident insert connection failed: %v", err)
		return
	}

	defer conn.Close(ctx)

	_, err = conn.Exec(
		ctx,
		`INSERT INTO incidents
			(id, service, type, severity, description, value, detected_at, status)
		 VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)`,
		incident.ID,
		incident.Service,
		incident.Type,
		incident.Severity,
		incident.Description,
		incident.Value,
		incident.DetectedAt,
		incident.Status,
	)

	if err != nil {
		log.Printf("❌ DATABASE | incident insert failed: %v", err)
		return
	}

	log.Printf(
		"💾 DATABASE | incident saved | id=%s | status=%s",
		incident.ID,
		incident.Status,
	)
}

func saveHealingAction(
	incidentID string,
	action HealingAction,
	executionStatus string,
	verificationStatus string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | healing action connection failed: %v",
			err,
		)
		return
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(
		ctx,
		`INSERT INTO healing_actions
			(incident_id, action, reason, approved, execution_status, verification_status)
		 VALUES
			($1, $2, $3, $4, $5, $6)`,
		incidentID,
		action.Action,
		action.Reason,
		action.Approved,
		executionStatus,
		verificationStatus,
	)

	if err != nil {
		log.Printf(
			"❌ DATABASE | healing action insert failed | incident=%s | action=%s | error=%v",
			incidentID,
			action.Action,
			err,
		)
		return
	}

	log.Printf(
		"💾 DATABASE | healing action saved | incident=%s | action=%s | execution=%s | verification=%s",
		incidentID,
		action.Action,
		executionStatus,
		verificationStatus,
	)
}

func updateHealingAction(
	incidentID string,
	action string,
	executionStatus string,
	verificationStatus string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | healing action update connection failed: %v",
			err,
		)
		return
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(
		ctx,
		`UPDATE healing_actions
		 SET execution_status = $1,
		     verification_status = $2
		 WHERE incident_id = $3
		   AND action = $4`,
		executionStatus,
		verificationStatus,
		incidentID,
		action,
	)

	if err != nil {
		log.Printf(
			"❌ DATABASE | healing action update failed | incident=%s | action=%s | error=%v",
			incidentID,
			action,
			err,
		)
		return
	}

	log.Printf(
		"💾 DATABASE | healing action updated | incident=%s | action=%s | execution=%s | verification=%s",
		incidentID,
		action,
		executionStatus,
		verificationStatus,
	)
}

func updateIncidentStatus(id string, status string) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		log.Printf(
			"❌ DATABASE | status update connection failed: %v",
			err,
		)
		return
	}
	defer conn.Close(ctx)

	tag, err := conn.Exec(
		ctx,
		`UPDATE incidents
		 SET status = $1
		 WHERE id = $2`,
		status,
		id,
	)
	if err != nil {
		log.Printf(
			"❌ DATABASE | incident status update failed | id=%s | status=%s | error=%v",
			id,
			status,
			err,
		)
		return
	}

	if tag.RowsAffected() == 0 {
		log.Printf(
			"⚠️ DATABASE | no incident row updated | id=%s | requested_status=%s",
			id,
			status,
		)
		return
	}

	log.Printf(
		"💾 DATABASE | incident status updated | id=%s | status=%s | rows=%d",
		id,
		status,
		tag.RowsAffected(),
	)
}
func main() {
	testDatabaseConnection()
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/incidents", incidentsHandler)
	http.HandleFunc("/healing-actions", healingActionsHandler)

	go detectIncidents()

	log.Println("Incident Manager running on http://localhost:8090")

	if err := http.ListenAndServe(":8090", nil); err != nil {
		log.Fatal(err)
	}
}
