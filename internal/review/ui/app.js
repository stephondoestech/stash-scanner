const els = {
  dot: document.getElementById("dot"),
  statusText: document.getElementById("status-text"),
  refreshed: document.getElementById("refreshed"),
  itemCount: document.getElementById("item-count"),
  reviewCount: document.getElementById("review-count"),
  emptyCount: document.getElementById("empty-count"),
  queue: document.getElementById("queue"),
  queueEmpty: document.getElementById("queue-empty"),
  detail: document.getElementById("detail"),
  detailEmpty: document.getElementById("detail-empty"),
  detailTitle: document.getElementById("detail-title"),
  detailStatus: document.getElementById("detail-status"),
  detailType: document.getElementById("detail-type"),
  detailPath: document.getElementById("detail-path"),
  detailStudio: document.getElementById("detail-studio"),
  detailTags: document.getElementById("detail-tags"),
  detailBody: document.getElementById("detail-body"),
  candidates: document.getElementById("candidates"),
  refresh: document.getElementById("refresh"),
};

let state = { items: [] };
let selectedID = "";
let timer = null;

function fmt(value) {
  if (!value || value === "0001-01-01T00:00:00Z") return "Never";
  return new Date(value).toLocaleString();
}

function pickSelected(items) {
  if (!items.length) return null;
  return items.find((item) => item.id === selectedID) || items[0];
}

function render(status) {
  state = status;
  const selected = pickSelected(status.items || []);
  selectedID = selected ? selected.id : "";

  els.dot.className = "dot" + (status.running ? " live" : "");
  els.statusText.textContent = status.running ? "Refreshing queue" : (status.last_error ? status.last_error : "Queue ready");
  els.refreshed.textContent = `Last refresh: ${fmt(status.refreshed_at)}`;
  els.itemCount.textContent = String(status.item_count || 0);
  els.reviewCount.textContent = String(status.review_count || 0);
  els.emptyCount.textContent = String(status.empty_count || 0);
  renderQueue(status.items || []);
  renderDetail(selected);
}

function renderQueue(items) {
  els.queue.innerHTML = "";
  els.queueEmpty.style.display = items.length ? "none" : "block";
  for (const item of items) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "item" + (item.id === selectedID ? " active" : "");
    button.innerHTML = `
      <div style="display:flex;justify-content:space-between;gap:10px;align-items:center;">
        <strong>${escapeHTML(item.title || "(untitled)")}</strong>
        <span class="pill ${item.status === "no_candidate" ? "ok" : ""}">${escapeHTML(item.status.replace("_", " "))}</span>
      </div>
      <div class="sub">${escapeHTML(item.type)} • ${item.candidate_count} candidates • score ${item.best_score}</div>
      <div class="sub mono">${escapeHTML(item.path || "-")}</div>
    `;
    button.addEventListener("click", () => {
      selectedID = item.id;
      render(state);
    });
    els.queue.appendChild(button);
  }
}

function renderDetail(item) {
  const hasItem = !!item;
  els.detail.hidden = !hasItem;
  els.detailEmpty.style.display = hasItem ? "none" : "block";
  if (!hasItem) return;

  els.detailTitle.textContent = item.title || "(untitled)";
  els.detailStatus.textContent = `${item.status.replace("_", " ")} • ${item.candidate_count} candidates`;
  els.detailType.textContent = item.type;
  els.detailPath.textContent = item.path || "-";
  els.detailStudio.textContent = item.studio || "-";
  els.detailTags.textContent = (item.tags || []).join(", ") || "-";
  els.detailBody.textContent = item.details || "No details available.";

  els.candidates.innerHTML = "";
  if (!item.candidates || !item.candidates.length) {
    els.candidates.innerHTML = `<div class="empty">No likely performer candidates yet.</div>`;
    return;
  }

  for (const candidate of item.candidates) {
    const card = document.createElement("article");
    card.className = "candidate";
    const img = candidate.image_url ? `<img src="${escapeAttr(candidate.image_url)}" alt="${escapeAttr(candidate.name)}">` : `<img alt="" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 400 400'%3E%3Crect width='400' height='400' fill='%23ece3d7'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%238a7d6e' font-size='28' font-family='Arial'%3ENo image%3C/text%3E%3C/svg%3E">`;
    card.innerHTML = `
      ${img}
      <div class="candidate-body">
        <strong>${escapeHTML(candidate.name)}</strong>
        <div class="sub">Score ${candidate.score}</div>
        <div class="reason">${escapeHTML((candidate.reasons || []).join(", ") || "no reason recorded")}</div>
      </div>
    `;
    els.candidates.appendChild(card);
  }
}

async function loadStatus() {
  const res = await fetch("/api/status");
  if (!res.ok) throw new Error(await res.text());
  render(await res.json());
}

async function refreshQueue() {
  els.refresh.disabled = true;
  try {
    const res = await fetch("/api/refresh", { method: "POST" });
    if (!res.ok) throw new Error(await res.text());
    await loadStatus();
  } finally {
    els.refresh.disabled = false;
  }
}

function schedule() {
  clearTimeout(timer);
  if (document.hidden) return;
  const delay = state.running ? 5000 : 60000;
  timer = setTimeout(async () => {
    try {
      await loadStatus();
    } catch (err) {
      els.statusText.textContent = String(err);
    }
    schedule();
  }, delay);
}

function escapeHTML(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

function escapeAttr(value) {
  return escapeHTML(value).replaceAll("'", "&#39;");
}

els.refresh.addEventListener("click", async () => {
  await refreshQueue();
  schedule();
});

document.addEventListener("visibilitychange", async () => {
  if (!document.hidden) {
    try {
      await loadStatus();
    } catch (err) {
      els.statusText.textContent = String(err);
    }
  }
  schedule();
});

(async function boot() {
  try {
    await loadStatus();
  } catch (err) {
    els.statusText.textContent = String(err);
  }
  schedule();
})();
