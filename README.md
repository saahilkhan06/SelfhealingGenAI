 # Self-Healing GenAI Infrastructure 🤖

**Self-Healing GenAI Infrastructure** is a fault-tolerant microservices project that combines monitoring, chaos engineering, and Generative AI to detect, diagnose, and help recover from system failures.

The system consists of multiple Go-based microservices, including User, Order, and Payment services. It uses fault injection to simulate real-world problems such as service failures, increased latency, and communication timeouts. An Incident Manager monitors failures, identifies incidents, and uses a locally running Large Language Model (LLM) through Ollama to analyze the problem and recommend corrective actions.

The project aims to demonstrate how AI-assisted incident management can improve the reliability, resilience, and recovery of distributed systems.

## 🚀 Key Features

- **Microservices Architecture:** Independent User, Order, and Payment services built with Go.
- **Fault Injection:** Simulate service failures, network timeouts, and high latency.
- **Real-Time Monitoring:** Track service health and failure metrics using Prometheus.
- **Automated Incident Detection:** Identify abnormal failure rates and generate incidents.
- **GenAI-Powered Diagnosis:** Use a local LLM through Ollama to analyze incidents and suggest solutions.
- **Circuit Breaker Pattern:** Prevent repeated calls to unhealthy services and support recovery testing.
- **Containerization:** Run services and supporting infrastructure using Docker Compose.
- **Database Integration:** Use PostgreSQL to store incident-related data.

## 🛠️ Tech Stack

- **Backend:** Go (Golang)
- **AI / LLM:** Ollama, Qwen 2.5
- **Architecture:** Microservices
- **Database:** PostgreSQL
- **Monitoring:** Prometheus
- **Containerization:** Docker, Docker Compose
- **Reliability Engineering:** Fault Injection, Circuit Breaker, Incident Detection

## ⚙️ How It Works

1. A user initiates an operation through the microservices system.
2. The Order Service communicates with the Payment Service to process the operation.
3. Fault injection introduces failures or delays to simulate real-world incidents.
4. The system monitors service health and tracks failures.
5. The Incident Manager detects abnormal behavior and records an incident.
6. A local LLM analyzes the incident and provides a possible root cause and recommended action.
7. Recovery mechanisms can be tested to evaluate whether the system returns to a healthy state.

## 🎯 Project Goal

The goal is to explore how Generative AI can support automated incident diagnosis and self-healing capabilities in distributed systems, reducing manual troubleshooting and improving service reliability.

**Project focus:** AI-assisted incident management, distributed systems reliability, fault tolerance, and automated recovery.

## 📌 Future Enhancements

- Implement automated recovery actions based on incident diagnosis.
- Add more failure scenarios and resilience tests.
- Build a dashboard for monitoring incidents and service health.
- Evaluate recovery time and system availability before and after remediation.
- Improve AI diagnosis accuracy using incident history and service metrics.
 
 
 
 # Self-Healing GenAI Infrastructure

A prototype autonomous reliability system for distributed microservices that combines observability, GenAI-based incident diagnosis, deterministic safety policies, automated recovery, and recovery verification.

---

## Overview

Modern distributed applications consist of multiple services that depend on each other. When one service becomes slow, unavailable, or fails unexpectedly, the resulting failures can propagate across the system.

Traditional monitoring systems can detect these failures, but investigation and recovery often still require human intervention.

This project explores a **Self-Healing GenAI Infrastructure** that automates the incident lifecycle:

Failure
   ↓
Monitoring
   ↓
Incident Detection
   ↓
Evidence Collection
   ↓
GenAI Diagnosis
   ↓
Deterministic Safety Policy
   ↓
Approved Healing Action
   ↓
Recovery Verification
   ↓
Resolved / Escalated
   ↓
Audit + Dashboard






Key Features
Microservices-based distributed application
Docker containerization
Docker Compose orchestration
Prometheus-based observability
Automatic failure detection
Fault injection / chaos engineering
Retry mechanism
Circuit breaker
Request timeouts
Evidence-based incident analysis
Local GenAI diagnosis using Ollama
Deterministic safety policy
Allowlisted automated healing actions
Automated recovery verification
Incident escalation when no safe action exists
PostgreSQL incident and healing audit trail
Operational dashboard
Multiple validated failure scenarios
System Architecture
                    ┌─────────────────────┐
                    │        User         │
                    └──────────┬──────────┘
                               │
                               ↓
                    ┌─────────────────────┐
                    │    Order Service    │
                    │       :8082         │
                    └──────────┬──────────┘
                               │
                               ↓
                    ┌─────────────────────┐
                    │   Payment Service   │
                    │       :8083         │
                    └──────────┬──────────┘
                               │
                               ↓
                         PostgreSQL


              ┌─────────────────────────────┐
              │         Prometheus          │
              │            :9090            │
              └──────────────┬──────────────┘
                             │
                             ↓
              ┌─────────────────────────────┐
              │      Incident Manager       │
              │            :8090             │
              └──────────────┬──────────────┘
                             │
                 ┌───────────┴───────────┐
                 ↓                       ↓
        ┌────────────────┐      ┌─────────────────┐
        │ Evidence       │      │ PostgreSQL      │
        │ Collection     │      │ Audit Storage   │
        └───────┬────────┘      └─────────────────┘
                │
                ↓
        ┌────────────────────┐
        │ Ollama / Local LLM │
        │   qwen2.5:1.5b     │
        └─────────┬──────────┘
                  │
                  ↓
        ┌─────────────────────────┐
        │ Deterministic Safety    │
        │ Policy                  │
        └───────────┬─────────────┘
                    │
             ┌──────┴───────┐
             ↓              ↓
        Approved          NO_ACTION
          Action              │
             ↓                ↓
      Healing Executor    Escalation
             │
             ↓
       Recovery Check
             │
       ┌─────┴─────┐
       ↓           ↓
    Verified     Failed
       ↓           ↓
   Resolved    Escalated
Services
Component	Port	Responsibility
User Service	8081	User-related service
Order Service	8082	Creates orders and communicates with Payment Service
Payment Service	8083	Processes payments and exposes fault injection controls
PostgreSQL	5432	Persistent application and incident data
Prometheus	9090	Metrics collection and querying
Incident Manager	8090	Detection, diagnosis, policy, healing and verification
Dashboard	5173	Operational monitoring interface
Incident Lifecycle

The system follows a controlled incident lifecycle:

DETECTED
   ↓
DIAGNOSING
   ↓
HEALING
   ↓
EXECUTE
   ↓
VERIFYING
   ↓
RESOLVED

If diagnosis fails, the policy rejects the action, recovery fails, or verification fails:

                ┌──────────────┐
                │   INCIDENT   │
                └──────┬───────┘
                       ↓
                 Diagnosis
                       ↓
                Safety Policy
                       ↓
                ┌──────┴──────┐
                ↓             ↓
             Approved      Rejected
                ↓             ↓
             Healing       NO_ACTION
                ↓             ↓
           Verification    Escalated
                ↓
        ┌───────┴───────┐
        ↓               ↓
     Success          Failure
        ↓               ↓
    Resolved        Escalated
Observability

Prometheus monitors application metrics exposed by the services.

One of the primary failure indicators is:

increase(order_failures_total[1m])

When the failure rate crosses the configured threshold, the Incident Manager creates an incident.

The system also collects additional evidence such as:

Order Service health
Payment Service health
Payment latency
Order-to-Payment timeout
Failure rate
Incident metadata

This allows the GenAI model to reason from actual operational evidence instead of receiving only a generic error message.

GenAI Integration

The project uses a locally running Ollama model:

qwen2.5:1.5b

The Incident Manager sends structured incident evidence to the model.

The model produces:

{
  "root_cause": "short explanation",
  "confidence": "high|medium|low",
  "recommended_action": "safe recommended recovery action",
  "reason": "short explanation based on evidence"
}

The GenAI component is responsible for:

Root-cause diagnosis
Confidence estimation
Recovery recommendation
Explanation of the diagnosis

The LLM does not directly control infrastructure.

Safety Architecture

Allowing an LLM to directly execute infrastructure commands would create unnecessary operational risk.

Therefore, this project separates diagnosis from execution.

                    GenAI
                      │
                      ↓
              Diagnosis / Advice
                      │
                      ↓
          Deterministic Safety Policy
                      │
             ┌────────┴────────┐
             ↓                 ↓
          Approved          Rejected
             │                 │
             ↓                 ↓
     Allowlisted Action     NO_ACTION
             │                 │
             ↓                 ↓
        Execute             Escalate
             │
             ↓
        Verify Recovery

This provides a safety boundary around the GenAI component.

The important principle is:

The LLM recommends; deterministic rules decide.

Automated Healing Actions

The current system supports a controlled set of recovery actions.

1. Clear Payment Latency Fault

Used when:

Payment latency > Order Service timeout

Action:

CLEAR_PAYMENT_FAULT

The Incident Manager removes the injected latency and then verifies the Payment Service.

2. Restart Payment Service

Used when:

Payment Service = unhealthy

Action:

RESTART_PAYMENT_SERVICE

The Incident Manager requests a controlled process shutdown.

Docker's restart policy then restarts the container.

The Incident Manager waits for the service to become healthy before marking recovery as successful.

3. No Action

If the incident does not match a safe predefined recovery policy:

NO_ACTION

No infrastructure action is executed.

The incident is escalated.

This is an intentional safety behavior.

Chaos Engineering

The system contains controlled fault injection mechanisms for testing self-healing behavior.

Supported scenarios include:

High Latency
Payment Service
      ↓
5 second artificial latency
      ↓
Order timeout
      ↓
Repeated order failures
Service Unavailable
Payment Service
      ↓
Marked unhealthy
      ↓
Payment requests return failure
Unknown Fault

A failure condition is introduced that does not match a safe automated recovery policy.

Expected behavior:

NO_ACTION → Escalation
Resilience Mechanisms

The Order Service includes resilience mechanisms such as:

Retry

Failed Payment Service requests are retried before the request is considered failed.

Timeout

The Order Service does not wait indefinitely for the Payment Service.

Circuit Breaker

Repeated failures can trigger circuit-breaker behavior to prevent continuously sending requests to an unhealthy dependency.

These mechanisms help prevent cascading failures while the monitoring and self-healing system investigates the incident.

Database

PostgreSQL is used for persistent storage.

Important tables include:

orders
payments
incidents
healing_actions

The incidents table stores:

Incident ID
Service
Incident type
Severity
Description
Failure value
Detection time
Status
AI root cause
AI confidence
AI recommendation
AI reasoning

The healing_actions table stores:

Incident ID
Recovery action
Reason
Approval status
Execution status
Verification status
Creation time

This provides an audit trail of automated decisions.

Dashboard

The React + TypeScript dashboard provides operational visibility into the system.

The dashboard includes:

System Overview
Failure Trend
Service Health
Chaos & Recovery Lab
Self-Healing Pipeline
AI Decision & Recovery
Incident Details
Incident Timeline
Recovery Statistics
Recent Incidents
Self-Healing Activity

The dashboard allows an operator to see not only that a failure occurred, but also:

What failed?
     ↓
What evidence was collected?
     ↓
What did the AI diagnose?
     ↓
What did the safety policy decide?
     ↓
What recovery action was executed?
     ↓
Was recovery verified?
Experimental Validation

Three major scenarios have been validated.

Scenario	Policy	Action	Verification	Result
Payment High Latency	Approved	CLEAR_PAYMENT_FAULT	Passed	Resolved
Payment Unavailable	Approved	RESTART_PAYMENT_SERVICE	Passed	Resolved
Unknown Condition	Rejected	NO_ACTION	Not Executed	Escalated

These experiments demonstrate both:

Automated recovery
Safe refusal to recover when no approved policy exists

Detailed experiment results are available in:

docs/EXPERIMENTS.md
Technology Stack
Backend
Go
HTTP APIs
Microservices
Prometheus client
PostgreSQL
Infrastructure
Docker
Docker Compose
Prometheus
GenAI
Ollama
Qwen 2.5 1.5B
Frontend
React
TypeScript
Vite
Tailwind CSS
Recharts
Lucide React
Running the Project
Start the infrastructure

From the project root:

docker compose up -d --build

Check running containers:

docker compose ps
Start the dashboard
cd dashboard
npm install
npm run dev

The dashboard is available at:

http://localhost:5173
Important Services

Prometheus:

http://localhost:9090

Incident Manager:

http://localhost:8090

Payment Service:

http://localhost:8083

Order Service:

http://localhost:8082
Example Incident Flow

Consider a Payment Service latency failure.

1. Payment latency increases
        ↓
2. Order requests begin timing out
        ↓
3. Order Service records failures
        ↓
4. Prometheus collects the metric
        ↓
5. Incident Manager detects the threshold
        ↓
6. Incident is created in PostgreSQL
        ↓
7. Operational evidence is collected
        ↓
8. Ollama diagnoses the incident
        ↓
9. Deterministic policy evaluates the evidence
        ↓
10. CLEAR_PAYMENT_FAULT is approved
        ↓
11. Recovery action is executed
        ↓
12. Payment Service health is verified
        ↓
13. Incident becomes RESOLVED
        ↓
14. Healing result is stored in PostgreSQL
        ↓
15. Dashboard displays the complete lifecycle
Why This Project Is Different

Traditional monitoring typically follows:

Failure
   ↓
Metric
   ↓
Alert
   ↓
Human Investigation
   ↓
Human Recovery

This project explores:

Failure
   ↓
Monitoring
   ↓
Incident Detection
   ↓
Evidence Collection
   ↓
GenAI Diagnosis
   ↓
Deterministic Safety Policy
   ↓
Automated Recovery
   ↓
Verification
   ↓
Audit

The goal is not to replace established infrastructure platforms.

Instead, the project explores how GenAI can be integrated into an existing observability and reliability workflow while maintaining deterministic safety boundaries around automated infrastructure actions.

Limitations

This is a prototype rather than a production infrastructure platform.

Current limitations include:

Small local LLM model
Limited number of predefined healing actions
Prototype-level anomaly detection
Single-node Docker environment
Limited service topology
No production Kubernetes deployment
No distributed tracing
Limited authentication and authorization
Fault scenarios are intentionally controlled

These limitations provide opportunities for future development.

Future Scope

Possible future improvements include:

Kubernetes-based deployment
Kubernetes-native healing
Distributed tracing
OpenTelemetry integration
More sophisticated anomaly detection
Larger or specialized local LLMs
Incident severity classification
More recovery policies
Multi-service dependency graphs
Long-term incident learning
Human approval workflows for high-risk actions
Secure authentication and authorization
Multi-node deployment
Production-grade alerting integrations
Academic Significance

This project combines concepts from several areas of computer science:

Distributed Systems
Microservices
Cloud Computing
DevOps
Observability
Fault Tolerance
Chaos Engineering
Artificial Intelligence
Generative AI
Database Systems
Software Reliability
Automated Recovery

It demonstrates how these concepts can be combined into a single reliability-oriented system.

Core Design Principle

The central principle of this project is:

              ┌─────────────────┐
              │      GenAI      │
              │    Diagnoses    │
              └────────┬────────┘
                       ↓
              ┌─────────────────┐
              │    Deterministic│
              │      Policy     │
              │     Decides     │
              └────────┬────────┘
                       ↓
              ┌─────────────────┐
              │   Allowlisted   │
              │     Executor    │
              │      Acts       │
              └────────┬────────┘
                       ↓
              ┌─────────────────┐
              │    Verification │
              │      Proves     │
              │    Recovery     │
              └─────────────────┘

LLM diagnoses. Deterministic policy decides. Allowlisted executor acts. Verification proves recovery.

Project Status

Core prototype: Completed

Validated capabilities:

Failure detection
Evidence collection
GenAI diagnosis
Deterministic policy evaluation
Automated healing
Recovery verification
Incident escalation
Persistent audit trail
Operational dashboard
Multiple chaos scenarios