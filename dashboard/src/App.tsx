import {
  Activity,
  AlertTriangle,
  Brain,
  CheckCircle2,
  HeartPulse,
  Server,
  ShieldCheck,
  TrendingDown,
  TrendingUp,
  Wrench,
  Zap,
  XCircle,
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import {
  getFailureCount,
  getHealingActions,
  getIncidents,
  getOrderHealth,
  getPaymentHealth,
  getUserHealth,
  injectLatencyFailure,
  injectUnavailableFailure,
  type HealingAction,
  type Incident,
} from "./api";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

interface FailurePoint {
  time: string;
  failures: number;
}

function App() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [healingActions, setHealingActions] = useState<HealingAction[]>([]);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [paymentHealth, setPaymentHealth] = useState("unknown");
  const [userHealth, setUserHealth] = useState("unknown");
  const [orderHealth, setOrderHealth] = useState("unknown");

  const [failureCount, setFailureCount] = useState(0);
  const [failureHistory, setFailureHistory] = useState<FailurePoint[]>([]);
  const [failureType, setFailureType] = useState<"latency" | "unavailable">(
    "latency",
  );
  const [injectingFailure, setInjectingFailure] = useState(false);
  const [failureInjected, setFailureInjected] = useState(false);

  const [latencySeconds, setLatencySeconds] = useState(5);

  const healthyServices = [userHealth, orderHealth, paymentHealth].filter(
    (status) => status === "healthy",
  ).length;

  const systemHealth = Math.round((healthyServices / 3) * 100);

  const activeIncidents = incidents.filter(
    (incident) =>
      incident.status === "detected" ||
      incident.status === "diagnosing" ||
      incident.status === "healing" ||
      incident.status === "verifying",
  ).length;

  const escalatedIncidents = incidents.filter(
    (incident) => incident.status === "escalated",
  ).length;

  const verifiedRecoveries = healingActions.filter(
    (action) => action.approved && action.verification_status === "verified",
  ).length;

  const latestFailure =
    failureHistory.length > 0
      ? failureHistory[failureHistory.length - 1].failures
      : 0;

  const previousFailure =
    failureHistory.length > 1
      ? failureHistory[failureHistory.length - 2].failures
      : latestFailure;

  const failureTrendUp = latestFailure > previousFailure;
  const latestIncident =
    incidents.length > 0
      ? [...incidents].sort(
          (a, b) =>
            new Date(b.detected_at).getTime() -
            new Date(a.detected_at).getTime(),
        )[0]
      : null;
  const latestHealingAction = latestIncident
    ? (healingActions.find(
        (action) => action.incident_id === latestIncident.id,
      ) ?? null)
    : null;
  const labLifecycle = latestIncident
    ? {
        detected: true,

        evidence: [
          "diagnosing",
          "healing",
          "verifying",
          "resolved",
          "escalated",
        ].includes(latestIncident.status),

        diagnosis: ["healing", "verifying", "resolved", "escalated"].includes(
          latestIncident.status,
        ),

        policy: ["verifying", "resolved", "escalated"].includes(
          latestIncident.status,
        ),

        healing: ["verifying", "resolved", "escalated"].includes(
          latestIncident.status,
        ),

        verification: ["resolved", "escalated"].includes(latestIncident.status),
      }
    : {
        detected: false,
        evidence: false,
        diagnosis: false,
        policy: false,
        healing: false,
        verification: false,
      };
  const activeLabStage =
    latestIncident?.status === "detected"
      ? "detected"
      : latestIncident?.status === "diagnosing"
        ? "diagnosis"
        : latestIncident?.status === "healing"
          ? "healing"
          : latestIncident?.status === "verifying"
            ? "verification"
            : null;
  const pipelineStage =
    latestIncident?.status === "detected"
      ? "detect"
      : latestIncident?.status === "diagnosing"
        ? "diagnose"
        : latestIncident?.status === "healing"
          ? "heal"
          : latestIncident?.status === "verifying"
            ? "verify"
            : latestIncident?.status === "resolved"
              ? "resolved"
              : latestIncident?.status === "escalated"
                ? "escalated"
                : "idle";
  const totalIncidents = incidents.length;

  const resolvedIncidents = incidents.filter(
    (incident) => incident.status === "resolved",
  ).length;

  const successfulHealings = healingActions.filter(
    (action) =>
      action.approved &&
      action.execution_status === "executed" &&
      action.verification_status === "verified",
  ).length;

  const attemptedHealings = healingActions.filter(
    (action) =>
      action.approved &&
      (action.execution_status === "executed" ||
        action.execution_status === "failed"),
  ).length;

  const healingSuccessRate =
    attemptedHealings > 0
      ? Math.round((successfulHealings / attemptedHealings) * 100)
      : 0;

  useEffect(() => {
    let mounted = true;

    async function loadDashboard() {
      try {
        const [
          incidentData,
          paymentData,
          userData,
          orderData,
          failureData,
          healingData,
        ] = await Promise.all([
          getIncidents(),
          getPaymentHealth(),
          getUserHealth(),
          getOrderHealth(),
          getFailureCount(),
          getHealingActions(),
        ]);

        if (!mounted) return;

        setIncidents(incidentData);
        setPaymentHealth(paymentData.status);
        setUserHealth(userData.status);
        setOrderHealth(orderData.status);
        setFailureCount(failureData);
        setHealingActions(healingData);

        const now = new Date();

        const time = now.toLocaleTimeString([], {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        });

        setFailureHistory((previous) => {
          const updated = [
            ...previous,
            {
              time,
              failures: Number(failureData.toFixed(2)),
            },
          ];

          return updated.slice(-20);
        });

        setError(null);
      } catch (err) {
        if (!mounted) return;

        setError("Unable to connect to backend services");
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    }

    loadDashboard();

    const interval = setInterval(loadDashboard, 5000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  const handleInjectFailure = async () => {
    try {
      setInjectingFailure(true);
      setFailureInjected(false);

      if (failureType === "latency") {
        await injectLatencyFailure(latencySeconds);
      } else {
        await injectUnavailableFailure();
      }

      setFailureInjected(true);

      setTimeout(() => {
        setFailureInjected(false);
      }, 2000);
    } catch (error) {
      console.error("Failed to inject failure:", error);
    } finally {
      setInjectingFailure(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-white">
      {/* ========================================================= */}
      {/* HEADER */}
      {/* ========================================================= */}

      <header className="border-b border-slate-800 bg-slate-950/90">
        <div className="mx-auto flex max-w-7xl flex-col gap-4 px-6 py-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <ShieldCheck className="h-8 w-8 text-emerald-400" />

              <h1 className="text-xl font-bold">Self-Healing Infrastructure</h1>
            </div>

            <p className="mt-1 text-sm text-slate-400">
              GenAI-powered distributed system monitoring & recovery
            </p>
          </div>

          <div className="flex w-fit items-center gap-2 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-4 py-2 text-sm text-emerald-400">
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
            System Operational
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl space-y-8 px-6 py-8">
        {/* ========================================================= */}
        {/* SYSTEM OVERVIEW */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">System Overview</h2>

            <p className="text-sm text-slate-400">
              Current infrastructure health and recovery state
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <MetricCard
              icon={<Server />}
              label="Services"
              value={`${healthyServices} / 3`}
              description={
                healthyServices === 3
                  ? "All services healthy"
                  : `${3 - healthyServices} service(s) unhealthy`
              }
              status={healthyServices === 3 ? "healthy" : "warning"}
            />

            <MetricCard
              icon={<Activity />}
              label="Failures"
              value={failureCount.toFixed(2)}
              description="Failures in last 1 minute"
              status={failureCount >= 3 ? "danger" : "healthy"}
            />

            <MetricCard
              icon={<AlertTriangle />}
              label="Escalated Incidents"
              value={escalatedIncidents.toString()}
              description={
                escalatedIncidents === 0
                  ? "No incidents escalated"
                  : "Requires manual review"
              }
              status={escalatedIncidents === 0 ? "healthy" : "danger"}
            />

            <MetricCard
              icon={<HeartPulse />}
              label="System Health"
              value={`${systemHealth}%`}
              description="Overall service availability"
              status={systemHealth === 100 ? "healthy" : "warning"}
            />
          </div>
        </section>
        {/* ========================================================= */}
        {/* FAILURE TREND */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 className="text-lg font-semibold">Failure Trend</h2>

              <p className="text-sm text-slate-400">
                Live failure rate collected from Prometheus
              </p>
            </div>

            <div
              className={`flex items-center gap-2 text-sm ${
                failureHistory.length < 2
                  ? "text-slate-400"
                  : failureTrendUp
                    ? "text-red-400"
                    : latestFailure < previousFailure
                      ? "text-emerald-400"
                      : "text-slate-400"
              }`}
            >
              {failureHistory.length < 2 ? (
                <Activity className="h-4 w-4" />
              ) : failureTrendUp ? (
                <TrendingUp className="h-4 w-4" />
              ) : (
                <TrendingDown className="h-4 w-4" />
              )}

              {failureHistory.length < 2
                ? "Waiting for failure metrics"
                : failureTrendUp
                  ? "Failures increasing"
                  : latestFailure < previousFailure
                    ? "Failures decreasing"
                    : "Failures stable"}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5">
            {failureHistory.length < 2 ? (
              <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-slate-500">
                <Activity className="h-6 w-6" />
                <p>Waiting for failure metrics</p>
                <p className="text-xs">
                  The chart appears after two metric samples.
                </p>
              </div>
            ) : (
              <div className="h-64 w-full">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart
                    data={failureHistory}
                    margin={{
                      top: 10,
                      right: 10,
                      left: 0,
                      bottom: 0,
                    }}
                  >
                    <defs>
                      <linearGradient
                        id="failureGradient"
                        x1="0"
                        y1="0"
                        x2="0"
                        y2="1"
                      >
                        <stop
                          offset="0%"
                          stopColor="#34d399"
                          stopOpacity={0.25}
                        />

                        <stop
                          offset="100%"
                          stopColor="#34d399"
                          stopOpacity={0}
                        />
                      </linearGradient>
                    </defs>

                    <CartesianGrid
                      strokeDasharray="3 3"
                      stroke="#1e293b"
                      vertical={false}
                    />

                    <XAxis
                      dataKey="time"
                      tick={{
                        fill: "#64748b",
                        fontSize: 11,
                      }}
                      tickLine={false}
                      axisLine={false}
                      minTickGap={35}
                    />

                    <YAxis
                      allowDecimals
                      tick={{
                        fill: "#64748b",
                        fontSize: 11,
                      }}
                      tickLine={false}
                      axisLine={false}
                      width={32}
                    />

                    <Tooltip
                      cursor={{
                        stroke: "#475569",
                        strokeDasharray: "4 4",
                      }}
                      contentStyle={{
                        backgroundColor: "#0f172a",
                        border: "1px solid #334155",
                        borderRadius: "12px",
                        color: "#fff",
                        boxShadow: "0 10px 30px rgba(0,0,0,0.25)",
                      }}
                      labelStyle={{
                        color: "#94a3b8",
                        marginBottom: "4px",
                      }}
                      itemStyle={{
                        color: "#34d399",
                      }}
                      formatter={(value) => [
                        `${Number(value ?? 0).toFixed(2)} failures`,
                        "Failure rate",
                      ]}
                    />

                    <Area
                      type="monotone"
                      dataKey="failures"
                      stroke="#34d399"
                      fill="url(#failureGradient)"
                      strokeWidth={2}
                      dot={false}
                      activeDot={{
                        r: 4,
                      }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            )}
          </div>
        </section>
        {/* ========================================================= */}
        {/* SERVICE HEALTH */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Service Health</h2>

            <p className="text-sm text-slate-400">Live microservice status</p>
          </div>

          <div className="grid gap-4 md:grid-cols-3">
            <ServiceCard name="User Service" port="8081" status={userHealth} />

            <ServiceCard
              name="Order Service"
              port="8082"
              status={orderHealth}
            />

            <ServiceCard
              name="Payment Service"
              port="8083"
              status={paymentHealth}
            />
          </div>
        </section>
        {/* ========================================================= */}
        {/* CHAOS & RECOVERY LAB */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Chaos & Recovery Lab</h2>

            <p className="text-sm text-slate-400">
              Safely inject controlled failures and observe autonomous recovery
            </p>
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="grid gap-6 lg:grid-cols-[0.9fr_1.1fr]">
              {/* Failure Controls */}
              <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                <div className="mb-6 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-yellow-400/20 bg-yellow-400/10">
                      <Zap className="h-5 w-5 text-yellow-400" />
                    </div>

                    <div>
                      <h3 className="font-semibold text-white">
                        Failure Injection
                      </h3>

                      <p className="text-xs text-slate-500">
                        Controlled chaos testing
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/5 px-3 py-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />

                    <span className="text-[11px] font-medium text-emerald-400">
                      Safe Mode
                    </span>
                  </div>
                </div>

                {/* Target Service */}
                <div>
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                    Target Service
                  </label>

                  <select
                    className="mt-2 w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-sm text-slate-200 outline-none transition hover:border-slate-600 focus:border-yellow-400/50 focus:ring-1 focus:ring-yellow-400/20"
                    defaultValue="payment-service"
                  >
                    <option value="payment-service">Payment Service</option>
                  </select>
                </div>

                {/* Failure Type */}
                <div className="mt-5">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                      Failure Type
                    </label>

                    <span className="text-[11px] text-slate-600">
                      Select scenario
                    </span>
                  </div>

                  <div className="mt-2 grid grid-cols-2 gap-3">
                    <button
                      type="button"
                      onClick={() => setFailureType("latency")}
                      className={`rounded-xl border px-4 py-3 text-sm font-medium transition-all duration-200 ${
                        failureType === "latency"
                          ? "border-yellow-400/40 bg-yellow-400/10 text-yellow-300 shadow-[0_0_20px_rgba(250,204,21,0.05)]"
                          : "border-slate-700 bg-slate-950 text-slate-400 hover:border-yellow-400/30 hover:text-yellow-400"
                      }`}
                    >
                      High Latency
                    </button>

                    <button
                      type="button"
                      onClick={() => setFailureType("unavailable")}
                      className={`rounded-xl border px-4 py-3 text-sm font-medium transition-all duration-200 ${
                        failureType === "unavailable"
                          ? "border-red-400/40 bg-red-400/10 text-red-300 shadow-[0_0_20px_rgba(248,113,113,0.05)]"
                          : "border-slate-700 bg-slate-950 text-slate-400 hover:border-red-400/30 hover:text-red-400"
                      }`}
                    >
                      Service Unavailable
                    </button>
                  </div>
                </div>

                {/* Latency */}
                <div className="mt-5">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                      Latency
                    </label>

                    {failureType === "unavailable" && (
                      <span className="text-[11px] text-slate-600">
                        Not required
                      </span>
                    )}
                  </div>

                  <div className="mt-2 flex items-center gap-3">
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={latencySeconds}
                      disabled={failureType === "unavailable"}
                      onChange={(e) =>
                        setLatencySeconds(Number(e.target.value))
                      }
                      className="w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-sm text-slate-200 outline-none transition hover:border-slate-600 focus:border-yellow-400/50 focus:ring-1 focus:ring-yellow-400/20 disabled:cursor-not-allowed disabled:opacity-40"
                    />

                    <span className="shrink-0 text-sm text-slate-500">
                      seconds
                    </span>
                  </div>
                </div>

                {/* Inject Button */}
                <button
                  onClick={handleInjectFailure}
                  disabled={injectingFailure}
                  className={`mt-6 flex w-full cursor-pointer items-center justify-center gap-2 rounded-xl border px-5 py-3 text-sm font-semibold transition-all duration-200 disabled:cursor-not-allowed ${
                    failureInjected
                      ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-400"
                      : "border-yellow-400/30 bg-yellow-400/10 text-yellow-400 hover:border-yellow-400/50 hover:bg-yellow-400/15"
                  }`}
                >
                  {injectingFailure ? (
                    <>
                      <Activity className="h-4 w-4 animate-spin" />
                      Injecting Failure...
                    </>
                  ) : failureInjected ? (
                    <>
                      <CheckCircle2 className="h-4 w-4" />
                      Failure Injected
                    </>
                  ) : (
                    <>
                      <Zap className="h-4 w-4" />
                      Inject Failure
                    </>
                  )}
                </button>

                <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900/40 px-3 py-2.5">
                  <p className="text-[11px] leading-relaxed text-slate-500">
                    Failures are injected only into the selected test service.
                    Recovery remains controlled by the deterministic safety
                    policy.
                  </p>
                </div>
              </div>

              {/* Recovery Observation */}
              <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                <div className="mb-5 flex items-start justify-between gap-4">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                      Autonomous Recovery
                    </p>

                    <h3 className="mt-1 text-lg font-semibold text-white">
                      Recovery Lifecycle
                    </h3>

                    <p className="mt-1 text-sm text-slate-500">
                      Observe how the infrastructure responds to the injected
                      failure.
                    </p>
                  </div>

                  <div
                    className={`shrink-0 rounded-full border px-3 py-1.5 text-[11px] font-medium ${
                      latestIncident?.status === "resolved"
                        ? "border-emerald-400/20 bg-emerald-400/5 text-emerald-400"
                        : latestIncident?.status === "escalated"
                          ? "border-red-400/20 bg-red-400/5 text-red-400"
                          : "border-slate-700 bg-slate-900 text-slate-500"
                    }`}
                  >
                    {latestIncident?.status === "resolved"
                      ? "Recovered"
                      : latestIncident?.status === "escalated"
                        ? "Escalated"
                        : "Monitoring"}
                  </div>
                </div>

                <div className="space-y-3">
                  <LabStep
                    number="01"
                    title="Failure Detected"
                    description={
                      labLifecycle.detected
                        ? "Incident detected by monitoring"
                        : "Waiting for incident detection"
                    }
                    status={
                      activeLabStage === "detected"
                        ? "active"
                        : labLifecycle.detected
                          ? "completed"
                          : "pending"
                    }
                  />

                  <LabStep
                    number="02"
                    title="Evidence Collected"
                    description={
                      labLifecycle.evidence
                        ? "Service health and failure metrics collected"
                        : "Incident evidence will be gathered"
                    }
                    status={
                      labLifecycle.evidence
                        ? "completed"
                        : latestIncident?.status === "detected"
                          ? "active"
                          : "pending"
                    }
                  />

                  <LabStep
                    number="03"
                    title="AI Diagnosis"
                    description={
                      labLifecycle.diagnosis
                        ? latestIncident?.ai_root_cause ||
                          "Root cause identified"
                        : "GenAI will analyze the incident"
                    }
                    status={
                      activeLabStage === "diagnosis"
                        ? "active"
                        : labLifecycle.diagnosis
                          ? "completed"
                          : "pending"
                    }
                  />

                  <LabStep
                    number="04"
                    title="Policy Evaluation"
                    description={
                      labLifecycle.policy
                        ? latestHealingAction
                          ? `Policy approved: ${latestHealingAction.action.replaceAll("_", " ")}`
                          : "Deterministic safety policy evaluated"
                        : "Deterministic safety rules"
                    }
                    status={
                      latestIncident?.status === "healing" &&
                      !labLifecycle.policy
                        ? "active"
                        : labLifecycle.policy
                          ? "completed"
                          : "pending"
                    }
                  />

                  <LabStep
                    number="05"
                    title={
                      latestHealingAction
                        ? latestHealingAction.action.replaceAll("_", " ")
                        : "Healing"
                    }
                    description={
                      latestHealingAction
                        ? latestHealingAction.execution_status === "executed"
                          ? "Approved recovery action executed"
                          : latestHealingAction.execution_status === "failed"
                            ? "Recovery action failed"
                            : "Waiting for recovery execution"
                        : "Waiting for the deterministic healing policy"
                    }
                    status={
                      activeLabStage === "healing"
                        ? "active"
                        : labLifecycle.healing
                          ? "completed"
                          : "pending"
                    }
                  />

                  <LabStep
                    number="06"
                    title={
                      latestHealingAction?.verification_status === "verified"
                        ? "Recovery Verified"
                        : latestHealingAction?.verification_status === "failed"
                          ? "Verification Failed"
                          : "Verification"
                    }
                    description={
                      latestHealingAction?.verification_status === "verified"
                        ? "Recovery health check passed successfully"
                        : latestHealingAction?.verification_status === "failed"
                          ? "Recovery health check failed"
                          : "Waiting for recovery verification"
                    }
                    status={
                      activeLabStage === "verification"
                        ? "active"
                        : labLifecycle.verification
                          ? "completed"
                          : "pending"
                    }
                  />

                  {latestIncident?.status === "resolved" && (
                    <div className="mt-4 flex items-center gap-3 rounded-xl border border-emerald-400/20 bg-emerald-400/10 px-4 py-3">
                      <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-400" />

                      <div>
                        <p className="text-sm font-medium text-emerald-300">
                          Recovery completed successfully
                        </p>

                        <p className="mt-0.5 text-xs text-emerald-400/70">
                          The incident was automatically healed and verified.
                        </p>
                      </div>
                    </div>
                  )}

                  {latestIncident?.status === "escalated" && (
                    <div className="mt-4 flex items-center gap-3 rounded-xl border border-red-400/20 bg-red-400/10 px-4 py-3">
                      <XCircle className="h-5 w-5 shrink-0 text-red-400" />

                      <div>
                        <p className="text-sm font-medium text-red-300">
                          Recovery requires attention
                        </p>

                        <p className="mt-0.5 text-xs text-red-400/70">
                          Automated healing could not safely resolve the
                          incident.
                        </p>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </section>
        {/* ========================================================= */}
        {/* SELF-HEALING PIPELINE */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Self-Healing Pipeline</h2>

            <p className="text-sm text-slate-400">
              Automated incident detection, diagnosis and recovery
            </p>
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="grid gap-6 md:grid-cols-5">
              <PipelineStep
                icon={<Activity />}
                title="Detect"
                description="Prometheus"
                number="01"
                stage="detect"
                currentStage={pipelineStage}
              />

              <PipelineStep
                icon={<Brain />}
                title="Diagnose"
                description="GenAI / Ollama"
                number="02"
                stage="diagnose"
                currentStage={pipelineStage}
              />

              <PipelineStep
                icon={<ShieldCheck />}
                title="Policy"
                description="Safety rules"
                number="03"
                stage="policy"
                currentStage={pipelineStage}
              />

              <PipelineStep
                icon={<Wrench />}
                title="Heal"
                description="Automated action"
                number="04"
                stage="heal"
                currentStage={pipelineStage}
              />

              <PipelineStep
                icon={<CheckCircle2 />}
                title="Verify"
                description="Recovery check"
                number="05"
                stage="verify"
                currentStage={pipelineStage}
              />
            </div>
          </div>

          {/* AI Decision & Recovery */}
          {latestIncident && (
            <div className="mt-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
              <div className="mb-6 flex items-start justify-between gap-4">
                <div>
                  <h2 className="text-lg font-semibold">
                    AI Decision & Recovery
                  </h2>

                  <p className="mt-1 text-sm text-slate-400">
                    Evidence-driven diagnosis and controlled automated recovery
                  </p>
                </div>

                <div className="rounded-lg border border-slate-700 bg-slate-800/70 px-3 py-2 text-xs text-slate-400">
                  {latestIncident.id}
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                {/* Root Cause */}
                <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                  <div className="mb-3 flex items-center gap-2">
                    <AlertTriangle className="h-4 w-4 text-yellow-400" />

                    <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      Root Cause
                    </p>
                  </div>

                  <p className="text-sm leading-6 text-slate-200">
                    {latestIncident.ai_root_cause ||
                      "AI diagnosis not available for this incident."}
                  </p>
                </div>

                {/* AI Confidence */}
                <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                  <div className="mb-3 flex items-center gap-2">
                    <Brain className="h-4 w-4 text-violet-400" />

                    <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      AI Confidence
                    </p>
                  </div>

                  <span
                    className={`inline-flex rounded-full border px-3 py-1 text-xs font-semibold uppercase ${
                      latestIncident.ai_confidence.toLowerCase() === "high"
                        ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-400"
                        : latestIncident.ai_confidence.toLowerCase() ===
                            "medium"
                          ? "border-yellow-400/30 bg-yellow-400/10 text-yellow-400"
                          : "border-slate-600 bg-slate-800 text-slate-400"
                    }`}
                  >
                    {latestIncident.ai_confidence || "Unavailable"}
                  </span>
                </div>

                {/* Recommended Action */}
                <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                  <div className="mb-3 flex items-center gap-2">
                    <Wrench className="h-4 w-4 text-blue-400" />

                    <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      AI Recommendation
                    </p>
                  </div>

                  <p className="text-sm leading-6 text-slate-200">
                    {latestIncident.ai_recommended_action ||
                      "No recommendation available."}
                  </p>
                </div>

                {/* AI Reason */}
                <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                  <div className="mb-3 flex items-center gap-2">
                    <Activity className="h-4 w-4 text-cyan-400" />

                    <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      AI Reason
                    </p>
                  </div>

                  <p className="text-sm leading-6 text-slate-300">
                    {latestIncident.ai_reason ||
                      "No AI reasoning available for this incident."}
                  </p>
                </div>
              </div>

              {/* Deterministic Safety Policy */}
              <div className="mt-4 rounded-xl border border-slate-800 bg-slate-950/40 p-5">
                <div className="mb-4 flex items-center gap-2">
                  <ShieldCheck className="h-4 w-4 text-emerald-400" />

                  <div>
                    <p className="text-sm font-medium text-slate-300">
                      Deterministic Safety Policy
                    </p>

                    <p className="text-xs text-slate-500">
                      The LLM recommends; deterministic rules decide.
                    </p>
                  </div>
                </div>

                {(() => {
                  const healingAction = healingActions.find(
                    (action) => action.incident_id === latestIncident.id,
                  );

                  const approved = healingAction?.approved === true;

                  return (
                    <div className="grid gap-4 md:grid-cols-3">
                      {/* Policy Decision */}
                      <div className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
                        <p className="text-xs uppercase tracking-wider text-slate-500">
                          Policy Decision
                        </p>

                        <div
                          className={`mt-2 inline-flex rounded-lg border px-2.5 py-1 text-xs font-semibold ${
                            approved
                              ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-400"
                              : "border-red-400/30 bg-red-400/10 text-red-400"
                          }`}
                        >
                          {approved ? "APPROVED" : "NOT APPROVED"}
                        </div>

                        <p className="mt-2 text-xs leading-5 text-slate-500">
                          {approved
                            ? "A predefined safety rule matched this incident."
                            : "No safe automated recovery policy matched this incident."}
                        </p>
                      </div>

                      {/* Actual Action */}
                      <div className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
                        <p className="text-xs uppercase tracking-wider text-slate-500">
                          Actual Recovery Action
                        </p>

                        <p className="mt-2 text-sm font-semibold text-slate-200">
                          {healingAction
                            ? healingAction.action.replaceAll("_", " ")
                            : "NO ACTION"}
                        </p>

                        <p className="mt-2 text-xs leading-5 text-slate-500">
                          {healingAction?.reason ||
                            "No automated recovery action recorded."}
                        </p>
                      </div>

                      {/* Execution */}
                      <div className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
                        <p className="text-xs uppercase tracking-wider text-slate-500">
                          Recovery Status
                        </p>

                        <p
                          className={`mt-2 text-sm font-semibold ${
                            healingAction?.verification_status === "verified"
                              ? "text-emerald-400"
                              : healingAction?.execution_status === "executed"
                                ? "text-yellow-400"
                                : "text-slate-400"
                          }`}
                        >
                          {healingAction?.verification_status === "verified"
                            ? "VERIFIED"
                            : healingAction?.execution_status === "executed"
                              ? "EXECUTED"
                              : healingAction?.execution_status?.toUpperCase() ||
                                "PENDING"}
                        </p>

                        <p className="mt-2 text-xs leading-5 text-slate-500">
                          {healingAction?.verification_status === "verified"
                            ? "Recovery was successfully verified."
                            : healingAction?.execution_status === "executed"
                              ? "Recovery action executed; verification pending."
                              : "Waiting for recovery execution."}
                        </p>
                      </div>
                    </div>
                  );
                })()}
              </div>
            </div>
          )}
        </section>
        {/* Incident Details */}
        {latestIncident && (
          <div className="mt-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="mb-6">
              <h2 className="text-lg font-semibold">Incident Details</h2>

              <p className="mt-1 text-sm text-slate-400">
                Complete incident context, diagnosis and recovery record
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {" "}
              <DetailItem label="Incident ID" value={latestIncident.id} />
              <DetailItem
                label="Affected Service"
                value={latestIncident.service}
              />
              <DetailItem
                label="Suspected Root Cause"
                value={latestIncident.ai_root_cause || "Not yet determined"}
              />{" "}
              <DetailItem label="Incident Type" value={latestIncident.type} />
              <DetailItem
                label="Severity"
                value={latestIncident.severity.toUpperCase()}
              />
              <DetailItem
                label="Failure Rate"
                value={`${latestIncident.value.toFixed(2)} failures/min`}
              />
              <DetailItem
                label="Status"
                value={latestIncident.status.toUpperCase()}
              />
              <DetailItem
                label="Detected At"
                value={new Date(latestIncident.detected_at).toLocaleString()}
              />
              <DetailItem
                label="AI Confidence"
                value={latestIncident.ai_confidence.toUpperCase()}
              />
            </div>

            <div className="mt-4 rounded-xl border border-slate-800 bg-slate-950/40 p-4">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">
                Incident Description
              </p>

              <p className="text-sm leading-6 text-slate-300">
                {latestIncident.description}
              </p>
            </div>
          </div>
        )}
        {/* Incident Timeline */}
        {latestIncident && (
          <div className="mt-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="mb-6">
              <h2 className="text-lg font-semibold">Incident Timeline</h2>
              <p className="mt-1 text-sm text-slate-400">
                Automated incident lifecycle from detection to recovery
              </p>
            </div>

            {(() => {
              const healingAction = healingActions.find(
                (action) => action.incident_id === latestIncident.id,
              );

              const affectedService = latestIncident.service
                .replace(/-service/gi, " Service")
                .replace(/\b\w/g, (char) => char.toUpperCase());

              const isEscalated = latestIncident.status === "escalated";
              const isResolved = latestIncident.status === "resolved";
              const recoveryVerified =
                healingAction?.verification_status === "verified";

              return (
                <div className="space-y-6">
                  <TimelineItem
                    title="Incident Detected"
                    description={`The monitoring system detected a ${latestIncident.type.replace(/_/g, " ")} incident affecting ${affectedService}.`}
                    status="completed"
                    icon={<Activity className="h-4 w-4" />}
                  />

                  <TimelineItem
                    title="AI Diagnosis"
                    description={
                      latestIncident.ai_root_cause ||
                      "AI diagnosis has not been recorded for this incident."
                    }
                    status="completed"
                    icon={<Brain className="h-4 w-4" />}
                  />

                  <TimelineItem
                    title="Safety Policy"
                    description={
                      isEscalated
                        ? "The recovery workflow was escalated for human review."
                        : "The deterministic safety policy evaluated the proposed recovery action."
                    }
                    status="completed"
                    icon={<ShieldCheck className="h-4 w-4" />}
                  />

                  <TimelineItem
                    title="Healing Executed"
                    description={
                      isEscalated
                        ? "No automated recovery action was approved for this incident."
                        : healingAction?.action
                          ? `Recorded recovery action: ${healingAction.action.replace(/_/g, " ")}.`
                          : "No healing action is recorded for this incident."
                    }
                    status={
                      isEscalated
                        ? "escalated"
                        : healingAction?.execution_status === "executed"
                          ? "completed"
                          : "pending"
                    }
                    icon={<Wrench className="h-4 w-4" />}
                  />

                  <TimelineItem
                    title="Recovery Verified"
                    description={
                      recoveryVerified
                        ? `The recovery action was verified successfully for ${affectedService}.`
                        : isEscalated
                          ? "Automated recovery was not approved; human review is required."
                          : healingAction?.execution_status === "executed"
                            ? "The recovery action was executed, but successful verification has not been recorded."
                            : "Recovery verification is pending."
                    }
                    status={
                      recoveryVerified
                        ? "completed"
                        : isEscalated
                          ? "escalated"
                          : "pending"
                    }
                    icon={<CheckCircle2 className="h-4 w-4" />}
                  />

                  <TimelineItem
                    title={
                      isEscalated
                        ? "Incident Escalated"
                        : isResolved
                          ? "Incident Resolved"
                          : "Incident In Progress"
                    }
                    description={
                      isEscalated
                        ? "The incident requires human investigation or intervention."
                        : isResolved
                          ? "The incident is marked as resolved by the incident manager."
                          : "The incident is still being processed."
                    }
                    status={
                      isResolved
                        ? "completed"
                        : isEscalated
                          ? "escalated"
                          : "pending"
                    }
                    icon={
                      isEscalated ? (
                        <AlertTriangle className="h-4 w-4" />
                      ) : isResolved ? (
                        <CheckCircle2 className="h-4 w-4" />
                      ) : (
                        <Activity className="h-4 w-4" />
                      )
                    }
                  />
                </div>
              );
            })()}
          </div>
        )}
        {/* ========================================================= */}
        {/* RECOVERY STATISTICS */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Recovery Statistics</h2>

            <p className="text-sm text-slate-400">
              Incident outcomes and verified automated recovery performance
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              icon={<AlertTriangle className="h-5 w-5" />}
              label="Total Incidents"
              value={totalIncidents}
            />

            <StatCard
              icon={<CheckCircle2 className="h-5 w-5" />}
              label="Resolved Incidents"
              value={resolvedIncidents}
            />

            <StatCard
              icon={<Wrench className="h-5 w-5" />}
              label="Successful Healings"
              value={successfulHealings}
            />

            <StatCard
              icon={<TrendingUp className="h-5 w-5" />}
              label="Healing Success Rate"
              value={`${healingSuccessRate}%`}
            />
            <p className="text-xs text-slate-500">
              Percentage of executed recovery attempts verified successfully
            </p>
          </div>
        </section>
        {/* ========================================================= */}
        {/* RECENT INCIDENTS */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Recent Incidents</h2>

            <p className="text-sm text-slate-400">
              Latest events from Incident Manager
            </p>
          </div>

          {loading && (
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-sm text-slate-400">
              Loading incidents...
            </div>
          )}

          {error && (
            <div className="rounded-2xl border border-red-900 bg-red-950/30 p-6 text-red-400">
              {error}
            </div>
          )}

          {!loading && !error && (
            <div className="space-y-3">
              {incidents.slice(0, 5).map((incident) => {
                const isResolved = incident.status === "resolved";

                const isEscalated = incident.status === "escalated";

                return (
                  <div
                    key={incident.id}
                    className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 transition hover:border-slate-700 hover:bg-slate-900"
                  >
                    <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="font-semibold capitalize">
                            {incident.type.replaceAll("_", " ")}
                          </h3>

                          <span
                            className={`rounded-full px-2.5 py-1 text-xs font-medium ${
                              isResolved
                                ? "bg-emerald-500/10 text-emerald-400"
                                : isEscalated
                                  ? "bg-red-500/10 text-red-400"
                                  : "bg-yellow-500/10 text-yellow-400"
                            }`}
                          >
                            {incident.status}
                          </span>
                        </div>

                        <p className="mt-2 text-sm text-slate-400">
                          {incident.description}
                        </p>
                      </div>

                      <span className="shrink-0 rounded-full bg-slate-800 px-3 py-1 text-xs text-slate-400">
                        {incident.severity}
                      </span>
                    </div>

                    <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                      <InfoCard label="Service" value={incident.service} />

                      <InfoCard
                        label="Failures"
                        value={incident.value.toFixed(2)}
                      />

                      <InfoCard label="Incident ID" value={incident.id} />

                      <InfoCard
                        label="Detected"
                        value={new Date(incident.detected_at).toLocaleString()}
                      />
                    </div>

                    <div className="mt-4 flex items-center gap-2 text-xs">
                      <span
                        className={
                          isResolved
                            ? "text-emerald-400"
                            : isEscalated
                              ? "text-red-400"
                              : "text-yellow-400"
                        }
                      >
                        {isResolved
                          ? "✓ Recovery completed and verified"
                          : isEscalated
                            ? "⚠ Recovery could not be verified — requires manual review"
                            : "● Incident currently being processed"}
                      </span>
                    </div>
                  </div>
                );
              })}

              {incidents.length === 0 && (
                <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-sm text-slate-400">
                  No incidents recorded.
                </div>
              )}
            </div>
          )}
        </section>
        {/* ========================================================= */}
        {/* SELF-HEALING ACTIVITY */}
        {/* ========================================================= */}
        <section>
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Self-Healing Activity</h2>
            <p className="text-sm text-slate-400">
              Automated recovery actions and verification
            </p>
          </div>

          <div className="space-y-4">
            {healingActions.slice(0, 5).map((action) => {
              const isApproved = action.approved;
              const isExecuted = action.execution_status === "executed";
              const isVerified = action.verification_status === "verified";
              const executionFailed = action.execution_status === "failed";
              const verificationFailed =
                action.verification_status === "failed";
              const notExecuted = action.execution_status === "not_executed";

              // Determine the displayed state from the persisted action status.
              const isFailed = executionFailed || verificationFailed;
              const isEscalated = !isApproved || isFailed;
              const isInProgress =
                isApproved &&
                !isFailed &&
                !isVerified &&
                !notExecuted &&
                (isExecuted || action.execution_status === "executing");

              const statusLabel = isVerified
                ? "Recovered"
                : isFailed
                  ? "Failed"
                  : isEscalated
                    ? "Escalated"
                    : isInProgress
                      ? "In Progress"
                      : "Pending";

              const statusClass = isVerified
                ? "bg-emerald-500/10 text-emerald-400"
                : isFailed || isEscalated
                  ? "bg-red-500/10 text-red-400"
                  : isInProgress
                    ? "bg-yellow-500/10 text-yellow-400"
                    : "bg-slate-700/50 text-slate-300";

              const lifecycleLabel = isVerified
                ? "Recovery verified"
                : isFailed
                  ? "Recovery failed"
                  : isEscalated
                    ? "Escalated for review"
                    : isInProgress
                      ? "Recovery in progress"
                      : "Awaiting execution";

              const resultLabel = isVerified
                ? "Recovery verified successfully"
                : isFailed
                  ? "Recovery failed verification or execution"
                  : isEscalated
                    ? "Automated recovery was not approved"
                    : isInProgress
                      ? "Recovery is being processed"
                      : "Action has not been executed";

              const resultClass = isVerified
                ? "text-emerald-400"
                : isFailed || isEscalated
                  ? "text-red-400"
                  : isInProgress
                    ? "text-yellow-400"
                    : "text-slate-400";

              return (
                <div
                  key={action.id}
                  className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 transition hover:border-slate-700 hover:bg-slate-900"
                >
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="font-semibold">
                          {action.action.replaceAll("_", " ")}
                        </h3>

                        <span
                          className={`rounded-full px-2.5 py-1 text-xs font-medium ${statusClass}`}
                        >
                          {statusLabel}
                        </span>
                      </div>

                      <p className="mt-2 text-sm text-slate-400">
                        {action.reason}
                      </p>

                      <p className="mt-3 text-xs text-slate-500">
                        Incident: {action.incident_id}
                      </p>
                    </div>
                  </div>

                  <div className="mt-6 rounded-xl border border-slate-800 bg-slate-950/40 p-4">
                    <div className="mb-4 flex items-center justify-between gap-3">
                      <p className="text-xs font-medium uppercase tracking-wider text-slate-500">
                        Recovery Lifecycle
                      </p>

                      <p className={`text-xs font-medium ${resultClass}`}>
                        {lifecycleLabel}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <LifecycleStep
                        label="Approved"
                        active={isApproved}
                        completed={isApproved}
                      />

                      <div
                        className={`h-px flex-1 ${
                          isApproved ? "bg-emerald-500/40" : "bg-slate-800"
                        }`}
                      />

                      <LifecycleStep
                        label="Executed"
                        active={isExecuted}
                        completed={isExecuted}
                      />

                      <div
                        className={`h-px flex-1 ${
                          isExecuted ? "bg-emerald-500/40" : "bg-slate-800"
                        }`}
                      />

                      <LifecycleStep
                        label="Verified"
                        active={isVerified}
                        completed={isVerified}
                      />
                    </div>
                  </div>

                  <div className="mt-5 grid gap-3 sm:grid-cols-3">
                    <InfoCard
                      label="Root Cause / Reason"
                      value={action.reason}
                    />

                    <InfoCard
                      label="Recovery Action"
                      value={action.action.replaceAll("_", " ")}
                    />

                    <div className="rounded-xl bg-slate-800/50 p-4">
                      <p className="text-xs text-slate-500">Recovery Result</p>
                      <p className={`mt-2 text-sm font-medium ${resultClass}`}>
                        {resultLabel}
                      </p>
                    </div>
                  </div>

                  <p className="mt-4 text-xs text-slate-600">
                    {new Date(action.created_at).toLocaleString()}
                  </p>
                </div>
              );
            })}

            {healingActions.length === 0 && (
              <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-sm text-slate-400">
                No healing actions recorded.
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

/* ============================================================= */
/* METRIC CARD */
/* ============================================================= */

function MetricCard({
  icon,
  label,
  value,
  description,
  status,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  description: string;
  status: "healthy" | "warning" | "danger";
}) {
  const statusColor =
    status === "healthy"
      ? "bg-emerald-400"
      : status === "warning"
        ? "bg-yellow-400"
        : "bg-red-400";

  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 transition hover:border-slate-700">
      <div className="flex items-center justify-between">
        <div className="text-slate-400">{icon}</div>

        <span className={`h-2 w-2 rounded-full ${statusColor}`} />
      </div>

      <p className="mt-4 text-sm text-slate-400">{label}</p>

      <p className="mt-1 text-3xl font-bold">{value}</p>

      <p className="mt-1 text-xs text-slate-500">{description}</p>
    </div>
  );
}

/* ============================================================= */
/* SERVICE CARD */
/* ============================================================= */

function ServiceCard({
  name,
  port,
  status,
}: {
  name: string;
  port: string;
  status: string;
}) {
  const isHealthy = status === "healthy";

  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 transition hover:border-slate-700">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="rounded-xl bg-slate-800 p-3">
            <Server className="h-5 w-5 text-slate-300" />
          </div>

          <div>
            <h3 className="font-medium">{name}</h3>

            <p className="text-xs text-slate-500">Port {port}</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <span
            className={`h-2 w-2 rounded-full ${
              isHealthy ? "bg-emerald-400" : "bg-red-400"
            }`}
          />

          <p className={isHealthy ? "text-emerald-400" : "text-red-400"}>
            {isHealthy ? "Healthy" : "Unhealthy"}
          </p>
        </div>
      </div>
    </div>
  );
}
function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">
        {label}
      </p>

      <p className="mt-2 break-words text-sm font-medium text-slate-200">
        {value}
      </p>
    </div>
  );
}
function TimelineItem({
  title,
  description,
  status,
  icon,
}: {
  title: string;
  description: string;
  status: "completed" | "pending" | "escalated";
  icon: ReactNode;
}) {
  const completed = status === "completed";
  const escalated = status === "escalated";

  return (
    <div className="flex gap-4">
      <div className="relative flex flex-col items-center">
        <div
          className={`flex h-9 w-9 items-center justify-center rounded-full border ${
            completed
              ? "border-emerald-400/40 bg-emerald-400/10 text-emerald-400"
              : escalated
                ? "border-red-400/40 bg-red-400/10 text-red-400"
                : "border-slate-700 bg-slate-800 text-slate-500"
          }`}
        >
          {icon}
        </div>

        <div className="mt-2 h-full w-px bg-slate-800" />
      </div>

      <div className="pb-6">
        <div className="flex items-center gap-3">
          <h3
            className={`text-sm font-semibold ${
              completed
                ? "text-slate-200"
                : escalated
                  ? "text-red-400"
                  : "text-slate-200"
            }`}
          >
            {title}
          </h3>

          <span
            className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase ${
              completed
                ? "border-emerald-400/20 bg-emerald-400/10 text-emerald-400"
                : escalated
                  ? "border-red-400/20 bg-red-400/10 text-red-400"
                  : "border-slate-700 bg-slate-800 text-slate-500"
            }`}
          >
            {completed ? "Completed" : escalated ? "Escalated" : "Pending"}
          </span>
        </div>

        <p className="mt-1 text-sm leading-6 text-slate-500">{description}</p>
      </div>
    </div>
  );
}
/* ============================================================= */
/* PIPELINE STEP */
/* ============================================================= */

function PipelineStep({
  icon,
  title,
  description,
  number,
  stage,
  currentStage,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  number: string;
  stage: string;
  currentStage: string;
}) {
  const stageOrder = ["detect", "diagnose", "policy", "heal", "verify"];

  const currentIndex = stageOrder.indexOf(currentStage);
  const stageIndex = stageOrder.indexOf(stage);

  const isResolved = currentStage === "resolved";
  const isEscalated = currentStage === "escalated";

  const isCompleted =
    isResolved || (currentIndex >= 0 && stageIndex < currentIndex);

  const isActive = stage === currentStage;

  return (
    <div className="relative text-center">
      <div
        className={`mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl border transition-all duration-500 ${
          isEscalated
            ? "border-red-500/40 bg-red-500/10 text-red-400"
            : isCompleted
              ? "border-emerald-400/40 bg-emerald-400/10 text-emerald-400"
              : isActive
                ? "border-yellow-400/50 bg-yellow-400/10 text-yellow-400 shadow-lg shadow-yellow-400/10"
                : "border-slate-700 bg-slate-800 text-slate-500"
        }`}
      >
        {isCompleted ? (
          <CheckCircle2 className="h-5 w-5" />
        ) : isActive ? (
          <Activity className="h-5 w-5 animate-pulse" />
        ) : (
          icon
        )}
      </div>

      <p
        className={`text-[10px] font-semibold tracking-widest ${
          isCompleted
            ? "text-emerald-400"
            : isActive
              ? "text-yellow-400"
              : isEscalated
                ? "text-red-400"
                : "text-slate-600"
        }`}
      >
        {number}
      </p>

      <h3
        className={`mt-1 text-sm font-semibold ${
          isCompleted
            ? "text-emerald-400"
            : isActive
              ? "text-yellow-400"
              : isEscalated
                ? "text-red-400"
                : "text-white"
        }`}
      >
        {title}
      </h3>

      <p className="mt-1 text-xs text-slate-500">
        {isResolved
          ? "Completed"
          : isActive
            ? "Currently processing"
            : isCompleted
              ? "Completed"
              : description}
      </p>
    </div>
  );
} /* ============================================================= */
/* STAT CARD */
/* ============================================================= */

function StatCard({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string | number;
}) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5">
      <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-xl border border-slate-700 bg-slate-800 text-slate-300">
        {icon}
      </div>

      <p className="text-sm text-slate-500">{label}</p>

      <p className="mt-1 text-2xl font-semibold text-white">{value}</p>
    </div>
  );
}

/* ============================================================= */
/* INFO CARD */
/* ============================================================= */

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl bg-slate-800/50 p-4">
      <p className="text-xs text-slate-500">{label}</p>

      <p className="mt-2 break-words text-sm text-slate-300">{value}</p>
    </div>
  );
}

/* ============================================================= */
/* LIFECYCLE STEP */
/* ============================================================= */

function LifecycleStep({
  label,
  active,
  completed,
}: {
  label: string;
  active: boolean;
  completed: boolean;
}) {
  return (
    <div className="flex min-w-0 flex-col items-center gap-2">
      <div
        className={`flex h-8 w-8 items-center justify-center rounded-full border ${
          completed
            ? "border-emerald-400 bg-emerald-400/10 text-emerald-400"
            : active
              ? "border-yellow-400 bg-yellow-400/10 text-yellow-400"
              : "border-slate-700 bg-slate-800 text-slate-500"
        }`}
      >
        {completed ? "✓" : "•"}
      </div>

      <span
        className={`text-xs ${
          completed
            ? "text-emerald-400"
            : active
              ? "text-yellow-400"
              : "text-slate-500"
        }`}
      >
        {label}
      </span>
    </div>
  );
}
function LabStep({
  number,
  title,
  description,
  status,
}: {
  number: string;
  title: string;
  description: string;
  status: "pending" | "active" | "completed" | "failed";
}) {
  const styles =
    status === "completed"
      ? "border-emerald-400/30 bg-emerald-400/10"
      : status === "active"
        ? "border-yellow-400/30 bg-yellow-400/10"
        : status === "failed"
          ? "border-red-400/30 bg-red-400/10"
          : "border-slate-800 bg-slate-900/40";

  const numberStyles =
    status === "completed"
      ? "border-emerald-400/40 bg-emerald-400/10 text-emerald-400"
      : status === "active"
        ? "border-yellow-400/40 bg-yellow-400/10 text-yellow-400"
        : status === "failed"
          ? "border-red-400/40 bg-red-400/10 text-red-400"
          : "border-slate-700 bg-slate-800 text-slate-500";

  return (
    <div
      className={`flex items-center gap-4 rounded-xl border p-3 transition-all duration-300 ${styles}`}
    >
      <div
        className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full border text-xs font-semibold ${numberStyles}`}
      >
        {number}
      </div>

      <div className="min-w-0">
        <p className="text-sm font-medium text-slate-200">{title}</p>

        <p className="mt-0.5 text-xs text-slate-500">{description}</p>
      </div>

      <div className="ml-auto shrink-0">
        {status === "completed" && (
          <CheckCircle2 className="h-4 w-4 text-emerald-400" />
        )}

        {status === "active" && (
          <Activity className="h-4 w-4 animate-pulse text-yellow-400" />
        )}

        {status === "failed" && <XCircle className="h-4 w-4 text-red-400" />}
      </div>
    </div>
  );
}

export default App;
