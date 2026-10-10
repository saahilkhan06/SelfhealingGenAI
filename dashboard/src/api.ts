const INCIDENT_MANAGER_URL = "http://localhost:8090";

export interface Incident {
  id: string;
  service: string;
  type: string;
  severity: string;
  description: string;
  value: number;
  detected_at: string;
  status:
    | "detected"
    | "diagnosing"
    | "healing"
    | "verifying"
    | "resolved"
    | "escalated";

  ai_root_cause: string;
  ai_confidence: string;
  ai_recommended_action: string;
  ai_reason: string;
}

export async function getIncidents(): Promise<Incident[]> {
  const response = await fetch(`${INCIDENT_MANAGER_URL}/incidents`);

  if (!response.ok) {
    throw new Error("Failed to fetch incidents");
  }

  return response.json();
}
export interface ServiceHealth {
  status: string;
}

export async function getPaymentHealth(): Promise<ServiceHealth> {
  const response = await fetch("http://localhost:8083/health");

  if (!response.ok) {
    throw new Error("Payment Service is unhealthy");
  }

  return response.json();
}

export async function getUserHealth(): Promise<ServiceHealth> {
  const response = await fetch("http://localhost:8081/health");

  if (!response.ok) {
    throw new Error("User Service is unhealthy");
  }

  return response.json();
}

export async function getOrderHealth(): Promise<ServiceHealth> {
  const response = await fetch("http://localhost:8082/health");

  if (!response.ok) {
    throw new Error("Order Service is unhealthy");
  }

  return response.json();
}
const PROMETHEUS_URL = "http://localhost:9090";

export async function getFailureCount(): Promise<number> {
  const query = encodeURIComponent("increase(order_failures_total[1m])");

  const response = await fetch(`${PROMETHEUS_URL}/api/v1/query?query=${query}`);

  if (!response.ok) {
    throw new Error("Failed to fetch Prometheus metrics");
  }

  const data = await response.json();

  const result = data?.data?.result;

  if (!result || result.length === 0) {
    return 0;
  }

  return Number(result[0]?.value?.[1] ?? 0);
}
export interface HealingAction {
  id: number;
  incident_id: string;
  action: string;
  reason: string;
  approved: boolean;
  execution_status: string;
  verification_status: string;
  created_at: string;
}

export async function getHealingActions(): Promise<HealingAction[]> {
  const response = await fetch("http://localhost:8090/healing-actions");

  if (!response.ok) {
    throw new Error("Failed to fetch healing actions");
  }

  return response.json();
}
export async function injectLatencyFailure(
  seconds: number,
): Promise<void> {
  const response = await fetch(
    `http://localhost:8083/fault/latency?seconds=${seconds}`,
    {
      method: "POST",
    },
  );

  if (!response.ok) {
    throw new Error("Failed to inject latency fault");
  }
}

export async function injectUnavailableFailure(): Promise<void> {
  const response = await fetch(
    "http://localhost:8083/fault/unavailable",
    {
      method: "POST",
    },
  );

  if (!response.ok) {
    throw new Error("Failed to inject unavailable fault");
  }
}