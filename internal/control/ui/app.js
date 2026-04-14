const els = {
  version: document.getElementById("version"),
  mode: document.getElementById("mode"),
  lastRun: document.getElementById("last-run"),
  lastRunDetail: document.getElementById("last-run-detail"),
  pendingCount: document.getElementById("pending-count"),
  pendingDetail: document.getElementById("pending-detail"),
  lastOutcome: document.getElementById("last-outcome"),
  lastError: document.getElementById("last-error"),
  activity: document.getElementById("activity"),
  activityDetail: document.getElementById("activity-detail"),
  currentTrigger: document.getElementById("current-trigger"),
  currentStarted: document.getElementById("current-started"),
  currentUpdated: document.getElementById("current-updated"),
  currentIdentifySources: document.getElementById("current-identify-sources"),
  stashTaskTitle: document.getElementById("stash-task-title"),
  stashTaskDetail: document.getElementById("stash-task-detail"),
  stashTaskID: document.getElementById("stash-task-id"),
  stashTaskStatus: document.getElementById("stash-task-status"),
  stashTaskStarted: document.getElementById("stash-task-started"),
  stashTaskEnded: document.getElementById("stash-task-ended"),
  stashTaskProgress: document.getElementById("stash-task-progress"),
  pendingDebounce: document.getElementById("pending-debounce"),
  pendingDebounceCount: document.getElementById("pending-debounce-count"),
  pendingDebounceMeta: document.getElementById("pending-debounce-meta"),
  pendingDebounceEmpty: document.getElementById("pending-debounce-empty"),
  pendingRetry: document.getElementById("pending-retry"),
  pendingRetryCount: document.getElementById("pending-retry-count"),
  pendingRetryMeta: document.getElementById("pending-retry-meta"),
  pendingRetryEmpty: document.getElementById("pending-retry-empty"),
  watchRoots: document.getElementById("watch-roots"),
  watchRootsCount: document.getElementById("watch-roots-count"),
  watchRootsMeta: document.getElementById("watch-roots-meta"),
  watchRootsEmpty: document.getElementById("watch-roots-empty"),
  lastSummary: document.getElementById("last-summary"),
  lastSummaryEmpty: document.getElementById("last-summary-empty"),
  statusDot: document.getElementById("status-dot"),
  statusText: document.getElementById("status-text"),
  statusBadge: document.getElementById("status-badge"),
  pollState: document.getElementById("poll-state"),
  runNow: document.getElementById("run-now"),
  flushDebounce: document.getElementById("flush-debounce"),
  stopRun: document.getElementById("stop-run"),
};

let lastLoadedAt = null;
let lastStatus = null;
let refreshTimer = null;
let pollTimer = null;

function fmt(value) {
  if (!value) return "-";
  if (value === "0001-01-01T00:00:00Z") return "-";
  return new Date(value).toLocaleString();
}

function fmtRelative(date) {
  if (!date) return "never";
  const seconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000));
  if (seconds < 2) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  return `${hours}h ago`;
}

function render(status) {
  const running = status.running;
  const last = status.last_run || {};
  const pending = status.pending_scan || {};
  const debounce = status.pending_debounce || {};
  const current = status.current_run || {};
  const task = pickTask(current.stash_task, last.stash_task);
  const debouncePaths = debounce.paths || [];
  const retryPaths = pending.paths || [];
  const watchRoots = status.watch_roots || [];
  const pendingTotal = retryPaths.length + debouncePaths.length;

  els.version.textContent = status.version || "dev";
  els.mode.textContent = status.dry_run ? "Dry Run Mode" : "Live Mode";
  els.lastRun.textContent = fmt(status.last_run_at);
  els.lastRunDetail.textContent = last.finished_at ? `Finished ${fmt(last.finished_at)}` : "No completed runs yet";
  els.pendingCount.textContent = String(pendingTotal);
  els.pendingDetail.textContent = pendingTotal ? `${debouncePaths.length} debounced, ${retryPaths.length} retrying` : "No queued work";
  els.lastOutcome.textContent = last.stopped ? "Stopped" : (last.scan_succeeded ? "Success" : (last.last_error ? "Failed" : "Idle"));
  els.lastError.textContent = last.last_error || "No recent error";
  els.activity.textContent = running ? humanPhase(current.phase) : "Idle";
  els.activityDetail.textContent = running ? formatCurrentRun(current) : "No active run";
  els.currentTrigger.textContent = current.trigger || "-";
  els.currentStarted.textContent = fmt(current.started_at);
  els.currentUpdated.textContent = fmt(current.updated_at);
  els.currentIdentifySources.textContent = (current.identify_sources || []).join(", ") || (last.identify_sources || []).join(", ") || "-";
  renderTask(task);
  renderPathList(els.pendingDebounce, debouncePaths, els.pendingDebounceEmpty);
  renderPathList(els.pendingRetry, retryPaths, els.pendingRetryEmpty);
  renderPathList(els.watchRoots, watchRoots, els.watchRootsEmpty);
  renderSummary(last);
  els.pendingDebounceCount.textContent = String(debouncePaths.length);
  els.pendingDebounceMeta.textContent = debouncePaths.length ? `Ready ${fmt(debounce.ready_at)}` : "Nothing waiting to mature";
  els.pendingRetryCount.textContent = String(retryPaths.length);
  els.pendingRetryMeta.textContent = retryPaths.length ? `Attempt ${pending.attempt_count || 0}, next try ${fmt(pending.next_attempt_at)}` : "No retry backlog";
  els.watchRootsCount.textContent = String(watchRoots.length);
  els.watchRootsMeta.textContent = status.watch_roots_from_stash ? "Loaded from Stash configuration" : "Using configured watch roots";
  els.statusText.textContent = running ? (current.detail || "Run in progress") : (last.last_error ? "Waiting for retry" : "Ready");
  els.statusBadge.textContent = running ? "Running" : (last.last_error ? "Attention" : "Idle");
  els.statusBadge.className = "pill" + (last.last_error && !running ? " warn" : "");
  els.statusDot.className = "dot" + (running ? " live" : (last.last_error ? " warn" : ""));
  els.runNow.disabled = running;
  els.flushDebounce.disabled = !running && debouncePaths.length === 0;
  els.stopRun.disabled = !running;
}

function renderTask(task) {
  if (!hasTask(task)) {
    els.stashTaskTitle.textContent = "No Active Task";
    els.stashTaskDetail.textContent = "Waiting for scanner work";
    els.stashTaskID.textContent = "-";
    els.stashTaskStatus.textContent = "-";
    els.stashTaskStarted.textContent = "-";
    els.stashTaskEnded.textContent = "-";
    els.stashTaskProgress.style.width = "0%";
    return;
  }

  els.stashTaskTitle.textContent = task.description || "Stash task";
  els.stashTaskDetail.textContent = task.error || formatTask(task);
  els.stashTaskID.textContent = task.id || "-";
  els.stashTaskStatus.textContent = task.status || "-";
  els.stashTaskStarted.textContent = fmt(task.started_at || task.added_at);
  els.stashTaskEnded.textContent = fmt(task.ended_at);
  els.stashTaskProgress.style.width = `${Math.max(0, Math.min(100, Math.round((task.progress || 0) * 100)))}%`;
}

function renderSummary(last) {
  const items = [];
  if (last.trigger) items.push(`Trigger: ${last.trigger}`);
  if (last.started_at) items.push(`Started: ${fmt(last.started_at)}`);
  if (last.finished_at) items.push(`Finished: ${fmt(last.finished_at)}`);
  if (typeof last.tracked_files === "number") items.push(`Tracked files: ${last.tracked_files}`);
  if (typeof last.detected_targets === "number") items.push(`Detected targets: ${last.detected_targets}`);
  if (typeof last.scan_targets === "number") items.push(`Scan targets: ${last.scan_targets}`);
  if (typeof last.pending_after === "number") items.push(`Pending after run: ${last.pending_after}`);
  if (typeof last.retry_attempt === "number" && last.retry_attempt > 0) items.push(`Retry attempt: ${last.retry_attempt}`);
  if (Array.isArray(last.post_scan_tasks) && last.post_scan_tasks.length) items.push(`Post-scan: ${last.post_scan_tasks.join(", ")}`);
  if (Array.isArray(last.identify_sources) && last.identify_sources.length) items.push(`Identify sources: ${last.identify_sources.join(", ")}`);
  if (last.last_error) items.push(`Error: ${last.last_error}`);
  renderSimpleList(els.lastSummary, items, els.lastSummaryEmpty);
}

function renderPathList(el, items, emptyEl) {
  const max = 8;
  const lines = items.slice(0, max);
  if (items.length > max) {
    lines.push(`+ ${items.length - max} more`);
  }
  renderSimpleList(el, lines, emptyEl);
}

function renderSimpleList(el, items, emptyEl) {
  el.innerHTML = "";
  const hasItems = items.length > 0;
  emptyEl.style.display = hasItems ? "none" : "block";
  if (!hasItems) return;
  for (const item of items) {
    const li = document.createElement("li");
    li.textContent = item;
    el.appendChild(li);
  }
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
  if (!hasTask(task)) return "No active Stash task";
  const lines = [];
  if (task.status) lines.push(`Status: ${task.status}`);
  if (typeof task.progress === "number") {
    lines.push(`Progress: ${Math.round(task.progress * 100)}%`);
  }
  return lines.join("\n");
}

function hasTask(task) {
  return !!(task && (task.id || task.status || task.description || task.error));
}

function pickTask(currentTask, lastTask) {
  if (hasTask(currentTask)) return currentTask;
  if (hasTask(lastTask)) return lastTask;
  return null;
}

async function loadStatus() {
  const res = await fetch("/api/status");
  if (!res.ok) throw new Error(await res.text());
  const status = await res.json();
  lastStatus = status;
  lastLoadedAt = new Date();
  render(status);
  updatePollState();
}

function handleLoadError(err) {
  els.statusText.textContent = String(err);
  els.statusDot.className = "dot warn";
}

function refreshIntervalMs() {
  if (document.hidden) return 0;
  if (lastStatus && lastStatus.running) return 5000;
  return 60000;
}

function clearRefreshTimer() {
  if (refreshTimer) {
    clearTimeout(refreshTimer);
    refreshTimer = null;
  }
}

function scheduleRefresh(delay = refreshIntervalMs()) {
  clearRefreshTimer();
  if (delay <= 0) return;
  refreshTimer = setTimeout(async () => {
    try {
      await loadStatus();
    } catch (err) {
      handleLoadError(err);
    }
    scheduleRefresh();
  }, delay);
}

els.runNow.addEventListener("click", async () => {
  els.runNow.disabled = true;
  const res = await fetch("/api/run-now", { method: "POST" });
  if (!res.ok && res.status !== 409) {
    throw new Error(await res.text());
  }
  await loadStatus();
});

els.flushDebounce.addEventListener("click", async () => {
  els.flushDebounce.disabled = true;
  const res = await fetch("/api/flush-debounce", { method: "POST" });
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

function updatePollState() {
  if (document.hidden) {
    els.pollState.textContent = lastLoadedAt ? `Paused while tab is hidden, last refreshed ${fmtRelative(lastLoadedAt)}` : "Paused while tab is hidden";
    return;
  }
  els.pollState.textContent = lastLoadedAt ? `Last refreshed ${fmtRelative(lastLoadedAt)}` : "Waiting for first refresh";
}

async function boot() {
  try {
    await loadStatus();
    scheduleRefresh();
    pollTimer = setInterval(updatePollState, 1000);
  } catch (err) {
    handleLoadError(err);
  }
}

document.addEventListener("visibilitychange", async () => {
  updatePollState();
  if (document.hidden) {
    clearRefreshTimer();
    return;
  }
  try {
    await loadStatus();
  } catch (err) {
    handleLoadError(err);
  }
  scheduleRefresh();
});

boot();
