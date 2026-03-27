const els = {
  version: document.getElementById("version"),
  mode: document.getElementById("mode"),
  lastRun: document.getElementById("last-run"),
  pendingCount: document.getElementById("pending-count"),
  lastOutcome: document.getElementById("last-outcome"),
  activity: document.getElementById("activity"),
  activityDetail: document.getElementById("activity-detail"),
  stashTask: document.getElementById("stash-task"),
  currentRun: document.getElementById("current-run"),
  pendingDebounce: document.getElementById("pending-debounce"),
  pendingRetry: document.getElementById("pending-retry"),
  watchRoots: document.getElementById("watch-roots"),
  lastSummary: document.getElementById("last-summary"),
  statusDot: document.getElementById("status-dot"),
  statusText: document.getElementById("status-text"),
  runNow: document.getElementById("run-now"),
  stopRun: document.getElementById("stop-run"),
};

function fmt(value) {
  if (!value) return "-";
  if (value === "0001-01-01T00:00:00Z") return "-";
  return new Date(value).toLocaleString();
}

function render(status) {
  const running = status.running;
  const last = status.last_run || {};
  const pending = status.pending_scan || {};
  const debounce = status.pending_debounce || {};
  const current = status.current_run || {};
  const task = current.stash_task || last.stash_task || {};

  els.version.textContent = status.version || "dev";
  els.mode.textContent = status.dry_run ? "Dry Run" : "Live";
  els.lastRun.textContent = fmt(status.last_run_at);
  els.pendingCount.textContent = String((pending.paths || []).length + (debounce.paths || []).length);
  els.lastOutcome.textContent = last.stopped ? "Stopped" : (last.scan_succeeded ? "Success" : (last.last_error ? "Failed" : "Idle"));
  els.activity.textContent = running ? humanPhase(current.phase) : "Idle";
  els.activityDetail.textContent = running ? formatCurrentRun(current) : "No active run";
  els.stashTask.textContent = formatTask(task);
  els.currentRun.textContent = running ? JSON.stringify(status.current_run, null, 2) : "No active run";
  els.pendingDebounce.textContent = JSON.stringify(debounce, null, 2);
  els.pendingRetry.textContent = JSON.stringify(pending, null, 2);
  els.watchRoots.textContent = (status.watch_roots || []).join("\n") || "-";
  els.lastSummary.textContent = JSON.stringify(last, null, 2);
  els.statusText.textContent = running ? (current.detail || "Run in progress") : (last.last_error ? "Waiting for retry" : "Ready");
  els.statusDot.className = "dot" + (running ? " live" : (last.last_error ? " warn" : ""));
  els.runNow.disabled = running;
  els.stopRun.disabled = !running;
}

function humanPhase(phase) {
  switch (phase) {
    case "starting": return "Starting";
    case "resolving_roots": return "Resolving Roots";
    case "scanning_files": return "Scanning Files";
    case "triggering_scan": return "Triggering Stash";
    case "post_scan_task": return "Running Post-Scan Task";
    case "waiting_retry": return "Waiting To Retry";
    case "debouncing_changes": return "Debouncing Changes";
    case "saving_state": return "Saving State";
    case "waiting_for_stash": return "Waiting For Stash";
    case "stopping": return "Stopping";
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

function formatTask(task) {
  if (!task || !task.id) return "No active Stash task";
  const lines = [];
  lines.push(`ID: ${task.id}`);
  if (task.description) lines.push(`Task: ${task.description}`);
  if (task.status) lines.push(`Status: ${task.status}`);
  if (typeof task.progress === "number" && task.progress > 0) {
    lines.push(`Progress: ${Math.round(task.progress * 100)}%`);
  }
  if (task.started_at) lines.push(`Started: ${fmt(task.started_at)}`);
  if (task.ended_at) lines.push(`Ended: ${fmt(task.ended_at)}`);
  if (task.error) lines.push(`Error: ${task.error}`);
  return lines.join("\n");
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

els.stopRun.addEventListener("click", async () => {
  els.stopRun.disabled = true;
  const res = await fetch("/api/stop", { method: "POST" });
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
