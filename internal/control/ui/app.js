const els = {
  version: document.getElementById("version"),
  mode: document.getElementById("mode"),
  lastRun: document.getElementById("last-run"),
  pendingCount: document.getElementById("pending-count"),
  lastOutcome: document.getElementById("last-outcome"),
  activity: document.getElementById("activity"),
  activityDetail: document.getElementById("activity-detail"),
  currentRun: document.getElementById("current-run"),
  pendingRetry: document.getElementById("pending-retry"),
  watchRoots: document.getElementById("watch-roots"),
  lastSummary: document.getElementById("last-summary"),
  statusDot: document.getElementById("status-dot"),
  statusText: document.getElementById("status-text"),
  runNow: document.getElementById("run-now"),
};

function fmt(value) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}

function render(status) {
  const running = status.running;
  const last = status.last_run || {};
  const pending = status.pending_scan || {};
  const current = status.current_run || {};

  els.version.textContent = status.version || "dev";
  els.mode.textContent = status.dry_run ? "Dry Run" : "Live";
  els.lastRun.textContent = fmt(status.last_run_at);
  els.pendingCount.textContent = String((pending.paths || []).length);
  els.lastOutcome.textContent = last.scan_succeeded ? "Success" : (last.last_error ? "Failed" : "Idle");
  els.activity.textContent = running ? humanPhase(current.phase) : "Idle";
  els.activityDetail.textContent = running ? formatCurrentRun(current) : "No active run";
  els.currentRun.textContent = running ? JSON.stringify(status.current_run, null, 2) : "No active run";
  els.pendingRetry.textContent = JSON.stringify(pending, null, 2);
  els.watchRoots.textContent = (status.watch_roots || []).join("\n") || "-";
  els.lastSummary.textContent = JSON.stringify(last, null, 2);
  els.statusText.textContent = running ? (current.detail || "Run in progress") : (last.last_error ? "Waiting for retry" : "Ready");
  els.statusDot.className = "dot" + (running ? " live" : (last.last_error ? " warn" : ""));
  els.runNow.disabled = running;
}

function humanPhase(phase) {
  switch (phase) {
    case "starting": return "Starting";
    case "resolving_roots": return "Resolving Roots";
    case "scanning_files": return "Scanning Files";
    case "triggering_scan": return "Triggering Stash";
    case "waiting_retry": return "Waiting To Retry";
    case "saving_state": return "Saving State";
    case "completed": return "Completed";
    case "idle": return "Idle";
    default: return "Working";
  }
}

function formatCurrentRun(current) {
  const lines = [];
  if (current.trigger) lines.push(`Trigger: ${current.trigger}`);
  if (current.phase) lines.push(`Phase: ${humanPhase(current.phase)}`);
  if (current.detail) lines.push(current.detail);
  if (current.started_at) lines.push(`Started: ${fmt(current.started_at)}`);
  if (current.updated_at) lines.push(`Updated: ${fmt(current.updated_at)}`);
  return lines.join("\n") || "Run in progress";
}

async function loadStatus() {
  const res = await fetch("/api/status");
  if (!res.ok) throw new Error(await res.text());
  const status = await res.json();
  render(status);
}

els.runNow.addEventListener("click", async () => {
  els.runNow.disabled = true;
  const res = await fetch("/api/run-now", { method: "POST" });
  if (!res.ok && res.status !== 409) {
    throw new Error(await res.text());
  }
  await loadStatus();
});

async function boot() {
  try {
    await loadStatus();
    setInterval(loadStatus, 5000);
  } catch (err) {
    els.statusText.textContent = String(err);
    els.statusDot.className = "dot warn";
  }
}

boot();
