const els = {
  dot: document.getElementById("dot"),
  statusText: document.getElementById("status-text"),
  refreshed: document.getElementById("refreshed"),
  itemCount: document.getElementById("item-count"),
  activeCount: document.getElementById("active-count"),
  skippedCount: document.getElementById("skipped-count"),
  resolvedCount: document.getElementById("resolved-count"),
  reviewCount: document.getElementById("review-count"),
  emptyCount: document.getElementById("empty-count"),
  queue: document.getElementById("queue"),
  queueEmpty: document.getElementById("queue-empty"),
  selectVisible: document.getElementById("select-visible"),
  clearSelection: document.getElementById("clear-selection"),
  bulkSkip: document.getElementById("bulk-skip"),
  bulkReopen: document.getElementById("bulk-reopen"),
  selectionCount: document.getElementById("selection-count"),
  detail: document.getElementById("detail"),
  detailEmpty: document.getElementById("detail-empty"),
  detailTitle: document.getElementById("detail-title"),
  detailStatus: document.getElementById("detail-status"),
  detailType: document.getElementById("detail-type"),
  detailPath: document.getElementById("detail-path"),
  detailStudio: document.getElementById("detail-studio"),
  detailTags: document.getElementById("detail-tags"),
  detailBody: document.getElementById("detail-body"),
  assignmentMeta: document.getElementById("assignment-meta"),
  manualSearchQuery: document.getElementById("manual-search-query"),
  manualSearchButton: document.getElementById("manual-search-button"),
  manualSearchStatus: document.getElementById("manual-search-status"),
  manualSearchResults: document.getElementById("manual-search-results"),
  candidates: document.getElementById("candidates"),
  refresh: document.getElementById("refresh"),
  filterActive: document.getElementById("filter-active"),
  filterAll: document.getElementById("filter-all"),
  filterSkipped: document.getElementById("filter-skipped"),
  filterResolved: document.getElementById("filter-resolved"),
  skipItem: document.getElementById("skip-item"),
  reopenItem: document.getElementById("reopen-item"),
};

let state = { items: [] };
let selectedID = "";
let timer = null;
let filter = "active";
let manualSearchItemID = "";
const selectedIDs = new Set();

function describeSuppression(reason) {
  switch (reason) {
    case "ambiguous_match": return "suppressed: ambiguous top match";
    case "weak_signal": return "suppressed: weak signal";
    default: return "";
  }
}

function fmt(value) {
  if (!value || value === "0001-01-01T00:00:00Z") return "Never";
  return new Date(value).toLocaleString();
}

function pickSelected(items) {
  const visible = filteredItems(items);
  if (!visible.length) return null;
  const fromVisible = visible.find((item) => item.id === selectedID);
  if (fromVisible) return fromVisible;
  if (items.find((item) => item.id === selectedID)) return items.find((item) => item.id === selectedID);
  return visible[0];
}

function filteredItems(items) {
  switch (filter) {
    case "all": return items;
    case "skipped": return items.filter((item) => item.review_state === "skipped");
    case "resolved": return items.filter((item) => item.review_state === "resolved");
    default: return items.filter((item) => item.review_state === "pending");
  }
}

function updateFilterButtons() {
  els.filterActive.classList.toggle("active", filter === "active");
  els.filterAll.classList.toggle("active", filter === "all");
  els.filterSkipped.classList.toggle("active", filter === "skipped");
  els.filterResolved.classList.toggle("active", filter === "resolved");
}

function selectedItemIDs() {
  return Array.from(selectedIDs);
}

function visibleItems() {
  return filteredItems(state.items || []);
}

function updateSelectionControls() {
  const count = selectedIDs.size;
  els.selectionCount.textContent = `${count} selected`;
  els.clearSelection.disabled = count === 0;
  els.bulkSkip.disabled = count === 0;
  els.bulkReopen.disabled = count === 0;
}

function render(status) {
  state = status;
  const selected = pickSelected(status.items || []);
  selectedID = selected ? selected.id : "";

  els.dot.className = "dot" + (status.running ? " live" : "");
  els.statusText.textContent = status.running ? "Refreshing queue" : (status.last_error ? status.last_error : "Queue ready");
  els.refreshed.textContent = `Last refresh: ${fmt(status.refreshed_at)}`;
  els.itemCount.textContent = String(status.item_count || 0);
  els.activeCount.textContent = String(status.active_count || 0);
  els.skippedCount.textContent = String(status.skipped_count || 0);
  els.resolvedCount.textContent = String(status.resolved_count || 0);
  els.reviewCount.textContent = String(status.review_count || 0);
  els.emptyCount.textContent = String(status.empty_count || 0);
  updateFilterButtons();
  updateSelectionControls();
  renderQueue(filteredItems(status.items || []));
  renderDetail(selected);
}

function renderQueue(items) {
  els.queue.innerHTML = "";
  els.queueEmpty.style.display = items.length ? "none" : "block";
  for (const item of items) {
    const card = document.createElement("div");
    card.className = "item" + (item.id === selectedID ? " active" : "");
    card.innerHTML = `
      <div class="item-head">
        <input class="selector" type="checkbox" aria-label="Select item" ${selectedIDs.has(item.id) ? "checked" : ""}>
        <strong>${escapeHTML(item.title || "(untitled)")}</strong>
        <span class="pill ${item.review_state === "skipped" ? "ok" : ""}">${escapeHTML(item.review_state || "pending")}</span>
      </div>
      <div class="sub">${escapeHTML(item.type)} • ${escapeHTML(item.status.replace("_", " "))} • ${item.candidate_count} candidates • score ${item.best_score}</div>
      ${item.suppression_reason ? `<div class="sub">${escapeHTML(describeSuppression(item.suppression_reason))}</div>` : ""}
      <div class="sub mono">${escapeHTML(item.path || "-")}</div>
    `;
    const checkbox = card.querySelector(".selector");
    checkbox.addEventListener("click", (event) => {
      event.stopPropagation();
    });
    checkbox.addEventListener("change", () => {
      if (checkbox.checked) {
        selectedIDs.add(item.id);
      } else {
        selectedIDs.delete(item.id);
      }
      updateSelectionControls();
      renderDetail(pickSelected(state.items || []));
    });
    card.addEventListener("click", () => {
      selectedID = item.id;
      render(state);
    });
    els.queue.appendChild(card);
  }
}

function renderDetail(item) {
  const hasItem = !!item;
  els.detail.hidden = !hasItem;
  els.detailEmpty.style.display = hasItem ? "none" : "block";
  if (!hasItem) {
    resetManualSearch("");
    return;
  }

  if (manualSearchItemID !== item.id) {
    resetManualSearch(item.id);
  }

  els.detailTitle.textContent = item.title || "(untitled)";
  const suppression = item.suppression_reason ? ` • ${describeSuppression(item.suppression_reason)}` : "";
  els.detailStatus.textContent = `${item.review_state || "pending"} • ${item.status.replace("_", " ")} • ${item.candidate_count} candidates${suppression}`;
  els.detailType.textContent = item.type;
  els.detailPath.textContent = item.path || "-";
  els.detailStudio.textContent = item.studio || "-";
  els.detailTags.textContent = (item.tags || []).join(", ") || "-";
  els.detailBody.textContent = item.details || "No details available.";
  els.skipItem.hidden = item.review_state === "skipped" || item.review_state === "resolved";
  els.reopenItem.hidden = item.review_state === "pending";
  els.assignmentMeta.textContent = item.review_state === "resolved"
    ? `Resolved ${fmt(item.resolved_at)}${item.assigned_performer_ids?.length ? ` • performer ids: ${item.assigned_performer_ids.join(", ")}` : ""}`
    : "";

  els.candidates.innerHTML = "";
  if (!item.candidates || !item.candidates.length) {
    els.candidates.innerHTML = `<div class="empty">${escapeHTML(item.suppression_reason ? describeSuppression(item.suppression_reason) : "No likely performer candidates yet.")}</div>`;
  } else {
    for (const candidate of item.candidates) {
      const card = document.createElement("article");
      card.className = "candidate";
      const img = candidate.image_url
        ? `<img src="${escapeAttr(candidateImageURL(item.id, candidate.performer_id))}" alt="${escapeAttr(candidate.name)}" loading="lazy" referrerpolicy="no-referrer">`
        : `<img alt="" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 400 400'%3E%3Crect width='400' height='400' fill='%23ece3d7'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%238a7d6e' font-size='28' font-family='Arial'%3ENo image%3C/text%3E%3C/svg%3E">`;
      card.innerHTML = `
        ${img}
        <div class="candidate-body">
          <strong>${escapeHTML(candidate.name)}</strong>
          <div class="sub">Score ${candidate.score}</div>
        <div class="reason">${escapeHTML((candidate.reasons || []).join(", ") || "no reason recorded")}</div>
        <div style="margin-top:12px;">
          <button type="button" class="assign-candidate" data-item-id="${escapeAttr(item.id)}" data-performer-id="${escapeAttr(candidate.performer_id)}" ${item.review_state === "resolved" ? "disabled" : ""}>Assign</button>
          <button type="button" class="assign-selected" data-performer-id="${escapeAttr(candidate.performer_id)}" ${item.review_state === "resolved" || selectedIDs.size === 0 ? "disabled" : ""}>Assign Selected</button>
        </div>
      </div>
      `;
      els.candidates.appendChild(card);
    }
  }
  bindAssignButtons(els.candidates);
}

function candidateImageURL(itemID, performerID) {
  const params = new URLSearchParams({
    item_id: itemID,
    performer_id: performerID,
  });
  return `api/candidate-image?${params.toString()}`;
}

async function loadStatus() {
  const res = await fetch("api/status");
  if (!res.ok) throw new Error(await res.text());
  render(await res.json());
}

async function refreshQueue() {
  els.refresh.disabled = true;
  try {
    const res = await fetch("api/refresh", { method: "POST" });
    if (!res.ok) throw new Error(await res.text());
    await loadStatus();
  } finally {
    els.refresh.disabled = false;
  }
}

async function updateItemState(itemID, reviewState) {
  const res = await fetch("api/items/state", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ item_id: itemID, review_state: reviewState }),
  });
  if (!res.ok) throw new Error(await res.text());
  await loadStatus();
}

async function updateItemStateBulk(itemIDs, reviewState) {
  const res = await fetch("api/items/state-bulk", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ item_ids: itemIDs, review_state: reviewState }),
  });
  if (!res.ok) throw new Error(await res.text());
  await loadStatus();
}

async function assignCandidate(itemID, performerID) {
  const res = await fetch("api/items/assign", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ item_id: itemID, performer_id: performerID }),
  });
  if (!res.ok) throw new Error(await res.text());
  await loadStatus();
}

async function assignCandidateBulk(itemIDs, performerID) {
  const res = await fetch("api/items/assign-bulk", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ item_ids: itemIDs, performer_id: performerID }),
  });
  if (!res.ok) throw new Error(await res.text());
  await loadStatus();
}

async function searchPerformers(query) {
  const params = new URLSearchParams({ q: query });
  const res = await fetch(`api/performers/search?${params.toString()}`);
  if (!res.ok) throw new Error(await res.text());
  return await res.json();
}

function bindAssignButtons(root) {
  for (const button of root.querySelectorAll(".assign-candidate")) {
    button.addEventListener("click", async () => {
      button.disabled = true;
      try {
        await assignCandidate(button.dataset.itemId, button.dataset.performerId);
      } finally {
        button.disabled = false;
      }
    });
  }
  for (const button of root.querySelectorAll(".assign-selected")) {
    button.addEventListener("click", async () => {
      const itemIDs = selectedItemIDs();
      if (!itemIDs.length) return;
      button.disabled = true;
      try {
        await assignCandidateBulk(itemIDs, button.dataset.performerId);
        selectedIDs.clear();
        updateSelectionControls();
      } finally {
        button.disabled = false;
      }
    });
  }
}

function resetManualSearch(itemID) {
  manualSearchItemID = itemID;
  els.manualSearchQuery.value = "";
  els.manualSearchStatus.textContent = "";
  els.manualSearchResults.innerHTML = "";
}

function renderManualSearchResults(item, results) {
  els.manualSearchResults.innerHTML = "";
  if (!results.length) {
    els.manualSearchResults.innerHTML = `<div class="empty">No performer matches found.</div>`;
    return;
  }

  for (const candidate of results) {
    const card = document.createElement("article");
    card.className = "candidate";
    const img = candidate.image_url
      ? `<img src="${escapeAttr(candidateImageURL(item.id, candidate.performer_id))}" alt="${escapeAttr(candidate.name)}" loading="lazy" referrerpolicy="no-referrer">`
      : `<img alt="" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 400 400'%3E%3Crect width='400' height='400' fill='%23ece3d7'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%238a7d6e' font-size='28' font-family='Arial'%3ENo image%3C/text%3E%3C/svg%3E">`;
    const aliases = candidate.aliases?.length ? `Aliases: ${candidate.aliases.join(", ")}` : "No aliases";
    card.innerHTML = `
      ${img}
      <div class="candidate-body">
        <strong>${escapeHTML(candidate.name)}</strong>
        <div class="sub">Score ${candidate.score}</div>
        <div class="reason">${escapeHTML((candidate.reasons || []).join(", ") || aliases)}</div>
        <div class="sub">${escapeHTML(aliases)}</div>
        <div style="margin-top:12px;">
          <button type="button" class="assign-candidate" data-item-id="${escapeAttr(item.id)}" data-performer-id="${escapeAttr(candidate.performer_id)}" ${item.review_state === "resolved" ? "disabled" : ""}>Assign</button>
          <button type="button" class="assign-selected" data-performer-id="${escapeAttr(candidate.performer_id)}" ${item.review_state === "resolved" || selectedIDs.size === 0 ? "disabled" : ""}>Assign Selected</button>
        </div>
      </div>
    `;
    els.manualSearchResults.appendChild(card);
  }
  bindAssignButtons(els.manualSearchResults);
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

els.filterActive.addEventListener("click", () => {
  filter = "active";
  render(state);
});
els.filterAll.addEventListener("click", () => {
  filter = "all";
  render(state);
});
els.filterSkipped.addEventListener("click", () => {
  filter = "skipped";
  render(state);
});
els.filterResolved.addEventListener("click", () => {
  filter = "resolved";
  render(state);
});
els.selectVisible.addEventListener("click", () => {
  for (const item of visibleItems()) {
    selectedIDs.add(item.id);
  }
  render(state);
});
els.clearSelection.addEventListener("click", () => {
  selectedIDs.clear();
  render(state);
});
els.skipItem.addEventListener("click", async () => {
  if (!selectedID) return;
  await updateItemState(selectedID, "skipped");
});
els.reopenItem.addEventListener("click", async () => {
  if (!selectedID) return;
  await updateItemState(selectedID, "pending");
});
els.bulkSkip.addEventListener("click", async () => {
  const itemIDs = selectedItemIDs();
  if (!itemIDs.length) return;
  await updateItemStateBulk(itemIDs, "skipped");
  selectedIDs.clear();
  render(state);
});
els.bulkReopen.addEventListener("click", async () => {
  const itemIDs = selectedItemIDs();
  if (!itemIDs.length) return;
  await updateItemStateBulk(itemIDs, "pending");
  selectedIDs.clear();
  render(state);
});
els.manualSearchButton.addEventListener("click", async () => {
  const item = state.items?.find((entry) => entry.id === selectedID);
  if (!item) return;

  const query = els.manualSearchQuery.value.trim();
  if (!query) {
    els.manualSearchStatus.textContent = "Enter a performer name or alias.";
    els.manualSearchResults.innerHTML = "";
    return;
  }

  els.manualSearchButton.disabled = true;
  els.manualSearchStatus.textContent = "Searching performers...";
  try {
    const results = await searchPerformers(query);
    els.manualSearchStatus.textContent = `${results.length} performer match${results.length === 1 ? "" : "es"}`;
    renderManualSearchResults(item, results);
  } catch (err) {
    els.manualSearchStatus.textContent = String(err);
    els.manualSearchResults.innerHTML = "";
  } finally {
    els.manualSearchButton.disabled = false;
  }
});
els.manualSearchQuery.addEventListener("keydown", async (event) => {
  if (event.key !== "Enter") return;
  event.preventDefault();
  els.manualSearchButton.click();
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
