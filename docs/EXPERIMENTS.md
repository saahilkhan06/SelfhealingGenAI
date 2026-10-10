# Self-Healing GenAI Infrastructure
## Experiment & Validation Results

## 1. Objective

The objective of these experiments is to validate that the system can:

1. Detect service failures automatically.
2. Collect operational evidence.
3. Use GenAI to diagnose the probable root cause.
4. Apply deterministic safety policies.
5. Execute only approved recovery actions.
6. Verify whether recovery was successful.
7. Escalate incidents when no safe automated action exists.

---

## 2. Experiment Matrix

| Experiment | Injected Condition | AI Diagnosis | Policy Decision | Recovery Action | Verification | Final Result |
|---|---|---|---|---|---|---|
| 1 | Payment Service High Latency | Order-to-Payment timeout exceeded | Approved | CLEAR_PAYMENT_FAULT | Passed | Resolved |
| 2 | Payment Service Unavailable | Payment Service unavailable | Approved | RESTART_PAYMENT_SERVICE | Passed | Resolved |
| 3 | Unknown Failure Condition | Unknown / unsupported condition | Not Approved | NO_ACTION | Not Executed | Escalated |

---

## 3. Experiment 1 — High Latency

### Fault Injection

Payment Service latency was artificially increased to 5 seconds.

The Order Service has a payment request timeout of approximately 2 seconds.

### Expected Behavior

The system should detect repeated order failures and determine that the Payment Service latency exceeds the allowed timeout.

### Observed Flow

```text
Latency injected
      ↓
Order requests fail
      ↓
Prometheus records failures
      ↓
Incident detected
      ↓
Evidence collected
      ↓
GenAI diagnosis
      ↓
Deterministic policy
      ↓
CLEAR_PAYMENT_FAULT
      ↓
Recovery verification
      ↓
Incident resolved