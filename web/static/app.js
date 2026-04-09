const RANGE_SPECS = {
  day: {
    label: "Last 24 hours",
    durationMs: 24 * 60 * 60 * 1000,
    requestLimit: 12,
    sessionLimit: 6,
  },
  week: {
    label: "Last 7 days",
    durationMs: 7 * 24 * 60 * 60 * 1000,
    requestLimit: 14,
    sessionLimit: 6,
  },
  month: {
    label: "Last 5 weeks",
    durationMs: 35 * 24 * 60 * 60 * 1000,
    requestLimit: 16,
    sessionLimit: 8,
  },
};

const state = {
  range: "day",
  metric: "tokens",
  contributor: "instances",
  data: null,
  requestToken: 0,
};

async function loadJSON(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return response.json();
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function number(value) {
  return new Intl.NumberFormat().format(value || 0);
}

function compactNumber(value) {
  return new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(value || 0);
}

function percent(value) {
  return `${(value || 0).toFixed(1)}%`;
}

function duration(value) {
  const ms = Number(value || 0);
  if (ms >= 10000) {
    return `${(ms / 1000).toFixed(1)} s`;
  }
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)} s`;
  }
  return `${number(Math.round(ms))} ms`;
}

function shortDateTime(value) {
  const date = new Date(value);
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function shortTime(value) {
  return new Date(value).toLocaleTimeString(undefined, {
    hour: "numeric",
    minute: "2-digit",
  });
}

function shortDate(value) {
  return new Date(value).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

function relativeTime(value) {
  const target = new Date(value).getTime();
  const deltaMinutes = Math.round((target - Date.now()) / 60000);
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

  const absMinutes = Math.abs(deltaMinutes);
  if (absMinutes < 60) {
    return formatter.format(deltaMinutes, "minute");
  }

  const deltaHours = Math.round(deltaMinutes / 60);
  if (Math.abs(deltaHours) < 48) {
    return formatter.format(deltaHours, "hour");
  }

  const deltaDays = Math.round(deltaHours / 24);
  return formatter.format(deltaDays, "day");
}

function rate(part, total) {
  if (!total) {
    return 0;
  }
  return (part / total) * 100;
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function currentWindow(range) {
  const spec = RANGE_SPECS[range] || RANGE_SPECS.day;
  const end = new Date();
  const start = new Date(end.getTime() - spec.durationMs);
  return { start, end, spec };
}

function previousWindow(window) {
  const span = window.end.getTime() - window.start.getTime();
  return {
    start: new Date(window.start.getTime() - span),
    end: new Date(window.start.getTime()),
  };
}

function buildParams(window, extra = {}) {
  const params = new URLSearchParams(extra);
  params.set("started_after", window.start.toISOString());
  params.set("started_before", window.end.toISOString());
  return params;
}

function metricValue(bucket, metric) {
  switch (metric) {
    case "requests":
      return bucket.request_count || 0;
    case "latency":
      return bucket.avg_request_duration_ms || 0;
    case "tokens":
    default:
      return bucket.total_tokens || 0;
  }
}

function metricLabel(metric) {
  switch (metric) {
    case "requests":
      return "Requests";
    case "latency":
      return "Average Latency";
    case "tokens":
    default:
      return "Total Tokens";
  }
}

function metricFormatter(metric, value) {
  switch (metric) {
    case "requests":
      return `${number(Math.round(value))}`;
    case "latency":
      return duration(value);
    case "tokens":
    default:
      return `${compactNumber(Math.round(value))}`;
  }
}

function bucketLabel(range, bucket) {
  const start = new Date(bucket.bucket_start);
  if (range === "day") {
    return start.toLocaleTimeString(undefined, { hour: "numeric" });
  }
  if (range === "week") {
    return start.toLocaleDateString(undefined, { weekday: "short" });
  }
  return start.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function formatTrend(current, previous, options = {}) {
  const {
    type = "number",
    invert = false,
    emptyLabel = "No prior window",
  } = options;

  const currentNumber = Number(current || 0);
  const previousNumber = Number(previous || 0);

  if (!previousNumber && !currentNumber) {
    return `<span class="trend-badge neutral">${escapeHtml(emptyLabel)}</span>`;
  }

  let delta;
  let text;
  if (type === "rate") {
    delta = currentNumber - previousNumber;
    text = `${delta >= 0 ? "+" : ""}${delta.toFixed(1)} pts`;
  } else {
    delta = currentNumber - previousNumber;
    const base = previousNumber === 0 ? 100 : (Math.abs(delta) / Math.abs(previousNumber)) * 100;
    text = `${delta >= 0 ? "+" : ""}${base.toFixed(1)}%`;
  }

  let tone = "neutral";
  if (Math.abs(delta) >= 0.05) {
    const isPositive = delta > 0;
    tone = invert ? (isPositive ? "down" : "up") : (isPositive ? "up" : "down");
  }

  return `<span class="trend-badge ${tone}">${escapeHtml(`${text} vs prior`)}</span>`;
}

function renderKpis(summary, previousSummary) {
  const container = document.getElementById("kpi-grid");
  const successRate = rate(summary.success_count, summary.request_count);
  const prevSuccessRate = rate(previousSummary?.success_count, previousSummary?.request_count);
  const abortedRate = rate(summary.aborted_count, summary.request_count);
  const prevAbortedRate = rate(previousSummary?.aborted_count, previousSummary?.request_count);

  const cards = [
    {
      tone: "",
      label: "Requests",
      value: number(summary.request_count),
      note: `${number(summary.success_count)} successful responses`,
      trend: formatTrend(summary.request_count, previousSummary?.request_count),
    },
    {
      tone: "alt",
      label: "Total Tokens",
      value: number(summary.total_tokens),
      note: `${number(summary.prompt_tokens)} prompt • ${number(summary.output_tokens)} output`,
      trend: formatTrend(summary.total_tokens, previousSummary?.total_tokens),
    },
    {
      tone: "",
      label: "Average Duration",
      value: duration(summary.avg_request_duration_ms),
      note: "Median-style steadiness is not shown yet",
      trend: formatTrend(summary.avg_request_duration_ms, previousSummary?.avg_request_duration_ms, { invert: true }),
    },
    {
      tone: "alt",
      label: "Success Rate",
      value: percent(successRate),
      note: `${number(summary.success_count)} successful requests`,
      trend: formatTrend(successRate, prevSuccessRate, { type: "rate" }),
    },
    {
      tone: "",
      label: "Aborted Rate",
      value: percent(abortedRate),
      note: `${number(summary.aborted_count)} aborted requests`,
      trend: formatTrend(abortedRate, prevAbortedRate, { type: "rate", invert: true }),
    },
    {
      tone: "alt",
      label: "Active Sessions",
      value: number(summary.active_session_count),
      note: "Distinct session ids seen in this window",
      trend: formatTrend(summary.active_session_count, previousSummary?.active_session_count),
    },
  ];

  container.innerHTML = cards.map((card) => `
    <article class="kpi-card ${card.tone}">
      <p class="kpi-label">${escapeHtml(card.label)}</p>
      <p class="kpi-value">${escapeHtml(card.value)}</p>
      <div class="kpi-foot">
        ${card.trend}
        <p class="kpi-note">${escapeHtml(card.note)}</p>
      </div>
    </article>
  `).join("");
}

function niceMax(maxValue) {
  if (!maxValue || maxValue <= 0) {
    return 1;
  }

  const exponent = 10 ** Math.floor(Math.log10(maxValue));
  const fraction = maxValue / exponent;
  if (fraction <= 1) {
    return exponent;
  }
  if (fraction <= 2) {
    return 2 * exponent;
  }
  if (fraction <= 5) {
    return 5 * exponent;
  }
  return 10 * exponent;
}

function renderTimeseries(range, metric, items) {
  const container = document.getElementById("timeseries-chart");
  const title = document.getElementById("timeseries-title");
  title.textContent = `${metricLabel(metric)} Across Time`;

  if (!items || items.length === 0) {
    container.innerHTML = `<div class="chart-empty">No traffic has been captured in this window yet.</div>`;
    return;
  }

  const values = items.map((item) => metricValue(item, metric));
  const total = values.reduce((sum, value) => sum + value, 0);
  const peak = Math.max(...values, 0);
  const peakIndex = values.findIndex((value) => value === peak);
  const peakLabel = peakIndex >= 0 ? bucketLabel(range, items[peakIndex]) : "n/a";
  const maxValue = niceMax(peak);
  const ticks = 4;
  const width = 820;
  const height = 290;
  const margin = { top: 16, right: 16, bottom: 34, left: 48 };
  const plotWidth = width - margin.left - margin.right;
  const plotHeight = height - margin.top - margin.bottom;
  const stepX = items.length > 1 ? plotWidth / (items.length - 1) : plotWidth;

  const points = items.map((item, index) => {
    const value = values[index];
    const x = margin.left + stepX * index;
    const y = margin.top + plotHeight - (value / maxValue) * plotHeight;
    return { x, y, value, label: bucketLabel(range, item) };
  });

  const linePath = points.map((point, index) => `${index === 0 ? "M" : "L"} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`).join(" ");
  const areaPath = [
    linePath,
    `L ${points[points.length - 1].x.toFixed(2)} ${(margin.top + plotHeight).toFixed(2)}`,
    `L ${points[0].x.toFixed(2)} ${(margin.top + plotHeight).toFixed(2)}`,
    "Z",
  ].join(" ");

  const gridLines = Array.from({ length: ticks + 1 }, (_, index) => {
    const value = (maxValue / ticks) * index;
    const y = margin.top + plotHeight - (value / maxValue) * plotHeight;
    return `
      <line class="grid-line" x1="${margin.left}" x2="${width - margin.right}" y1="${y}" y2="${y}"></line>
      <text class="axis-text" x="${margin.left - 10}" y="${y + 4}" text-anchor="end">${escapeHtml(metricFormatter(metric, value))}</text>
    `;
  }).join("");

  const xLabels = points.filter((_, index) => {
    if (points.length <= 7) {
      return true;
    }
    const step = Math.ceil(points.length / 6);
    return index % step === 0 || index === points.length - 1;
  }).map((point) => `
    <text class="axis-text" x="${point.x}" y="${height - 8}" text-anchor="middle">${escapeHtml(point.label)}</text>
  `).join("");

  const pointDots = points.map((point) => `
    <circle class="chart-point" cx="${point.x}" cy="${point.y}" r="4">
      <title>${escapeHtml(`${point.label}: ${metricFormatter(metric, point.value)}`)}</title>
    </circle>
  `).join("");

  container.innerHTML = `
    <div class="chart-meta">
      <div>
        <strong>Window Total</strong>
        <span>${escapeHtml(metricFormatter(metric, total))}</span>
      </div>
      <div>
        <strong>Peak Bucket</strong>
        <span>${escapeHtml(`${metricFormatter(metric, peak)} at ${peakLabel}`)}</span>
      </div>
    </div>
    <svg class="chart-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="${escapeHtml(metricLabel(metric))} chart">
      ${gridLines}
      <path class="chart-area" d="${areaPath}"></path>
      <path class="chart-line" d="${linePath}"></path>
      ${pointDots}
      ${xLabels}
    </svg>
  `;
}

function heatmapCellColor(intensity) {
  const clamped = clamp(intensity, 0, 1);
  const alpha = 0.08 + clamped * 0.86;
  return `rgba(245, 143, 98, ${alpha.toFixed(3)})`;
}

function renderHeatmap(items) {
  const container = document.getElementById("heatmap-panel");
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="empty-state">No usage cadence is available for this window yet.</div>`;
    return;
  }

  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const hours = Array.from({ length: 24 }, (_, hour) => hour);
  const maxCount = Math.max(...items.map((item) => item.request_count || 0), 0);

  const byKey = new Map(items.map((item) => [`${item.weekday}-${item.hour}`, item]));
  const cells = [];
  for (let weekday = 0; weekday < 7; weekday++) {
    for (let hour = 0; hour < 24; hour++) {
      const item = byKey.get(`${weekday}-${hour}`) || { request_count: 0, total_tokens: 0 };
      const intensity = maxCount === 0 ? 0 : item.request_count / maxCount;
      const startLabel = new Date(2000, 0, 2 + weekday, hour).toLocaleTimeString(undefined, { hour: "numeric" });
      cells.push(`
        <div
          class="heatmap-cell"
          style="background:${heatmapCellColor(intensity)}"
          title="${escapeHtml(`${weekdays[weekday]} ${startLabel}: ${number(item.request_count)} requests, ${number(item.total_tokens)} tokens`)}"
        ></div>
      `);
    }
  }

  const legend = Array.from({ length: 5 }, (_, index) => {
    const intensity = index / 4;
    return `<span style="background:${heatmapCellColor(intensity)}"></span>`;
  }).join("");

  container.innerHTML = `
    <div class="heatmap-hours">
      ${hours.map((hour) => `<span>${hour % 6 === 0 ? escapeHtml(new Date(2000, 0, 1, hour).toLocaleTimeString(undefined, { hour: "numeric" })) : ""}</span>`).join("")}
    </div>
    <div class="heatmap-layout">
      <div class="heatmap-days">
        ${weekdays.map((label) => `<span>${escapeHtml(label)}</span>`).join("")}
      </div>
      <div class="heatmap-grid">${cells.join("")}</div>
    </div>
    <div class="heatmap-legend">
      <span>Lower</span>
      <div class="heatmap-legend-bar">${legend}</div>
      <span>Higher activity</span>
    </div>
  `;
}

function renderBreakdown(containerId, items, options = {}) {
  const {
    accent = "warm",
    empty = "No data yet.",
  } = options;

  const container = document.getElementById(containerId);
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="empty-state">${escapeHtml(empty)}</div>`;
    return;
  }

  const maxTokens = Math.max(...items.map((item) => item.total_tokens || 0), 0);
  const fillClass = accent === "cool" ? "bar-fill cool" : "bar-fill";

  container.innerHTML = items.slice(0, 6).map((item) => {
    const width = maxTokens === 0 ? 0 : ((item.total_tokens || 0) / maxTokens) * 100;
    return `
      <div class="breakdown-row">
        <div class="breakdown-head">
          <strong>${escapeHtml(item.key || "unknown")}</strong>
          <span class="muted">${escapeHtml(`${compactNumber(item.total_tokens || 0)} tokens`)}</span>
        </div>
        <div class="bar-track">
          <div class="${fillClass}" style="width:${width.toFixed(2)}%"></div>
        </div>
        <div class="breakdown-meta">
          <span>${escapeHtml(`${number(item.request_count || 0)} requests`)}</span>
          <span>${escapeHtml(duration(item.avg_request_duration_ms || 0))} avg</span>
        </div>
      </div>
    `;
  }).join("");
}

function contributorItems(summary, mode) {
  switch (mode) {
    case "agents":
      return summary.by_agent_name || [];
    case "clients":
      return summary.by_client_type || [];
    case "instances":
    default:
      return summary.by_client_instance || [];
  }
}

function renderSessions(items) {
  const container = document.getElementById("sessions-list");
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="empty-state">No sessions are active in this window yet.</div>`;
    return;
  }

  container.innerHTML = items.map((item) => {
    const identityParts = [];
    if (item.client_instance) {
      identityParts.push(item.client_instance);
    } else if (item.client_instances?.length > 1) {
      identityParts.push(`${item.client_instances.length} instances`);
    }
    if (item.agent_name) {
      identityParts.push(item.agent_name);
    } else if (item.agent_names?.length > 1) {
      identityParts.push(`${item.agent_names.length} agents`);
    }

    return `
      <div class="session-row">
        <div class="session-head">
          <strong class="mono">${escapeHtml(item.session_id)}</strong>
          <span class="session-pill">${escapeHtml(relativeTime(item.last_seen_at))}</span>
        </div>
        <div class="session-meta">
          <span>${escapeHtml(`${number(item.request_count)} requests`)}</span>
          <span>${escapeHtml(`${number(item.total_tokens)} tokens`)}</span>
          <span>${escapeHtml(identityParts.join(" • ") || "No attribution")}</span>
        </div>
      </div>
    `;
  }).join("");
}

function renderRequests(items) {
  const body = document.getElementById("requests-body");
  if (!items || items.length === 0) {
    body.innerHTML = `<tr><td colspan="7" class="muted">No requests captured in this window yet.</td></tr>`;
    return;
  }

  body.innerHTML = items.map((item) => {
    const statusClass = item.success ? "ok" : "warn";
    const clientParts = [item.client_type || "unknown", item.client_instance || "default"];
    return `
      <tr>
        <td>${escapeHtml(shortDateTime(item.started_at))}</td>
        <td><span class="status ${statusClass}">${escapeHtml(String(item.http_status))}</span></td>
        <td>${escapeHtml(item.model || "unknown")}</td>
        <td>${escapeHtml(number(item.total_tokens))}</td>
        <td>${escapeHtml(duration(item.request_duration_ms))}</td>
        <td>
          <div class="request-client">
            <span>${escapeHtml(clientParts.join(" / "))}</span>
            <span class="muted">${escapeHtml(item.agent_name || "no agent")}</span>
          </div>
        </td>
        <td class="mono">${escapeHtml(item.session_id || "-")}</td>
      </tr>
    `;
  }).join("");
}

function setSelectedButtons(groupSelector, attribute, value) {
  document.querySelectorAll(groupSelector).forEach((button) => {
    button.classList.toggle("active", button.dataset[attribute] === value);
  });
}

function setStatus(stateLabel, kind) {
  const pill = document.getElementById("status-pill");
  pill.textContent = stateLabel;
  pill.classList.remove("ready", "error");
  if (kind) {
    pill.classList.add(kind);
  }
}

function renderDashboard() {
  if (!state.data) {
    return;
  }

  const { window, summary, previousSummary, timeseries, heatmap, sessions, requests, lastRefresh } = state.data;
  document.getElementById("window-summary").textContent = window.spec.label;
  document.getElementById("last-refresh").textContent = `Updated ${shortTime(lastRefresh)}`;

  setSelectedButtons("#range-control .segment", "range", state.range);
  setSelectedButtons("#metric-control .segment", "metric", state.metric);
  setSelectedButtons("#contributor-control .segment", "contributor", state.contributor);

  renderKpis(summary, previousSummary);
  renderTimeseries(state.range, state.metric, timeseries.items);
  renderHeatmap(heatmap.items);
  renderBreakdown("models-breakdown", summary.by_model, {
    accent: "warm",
    empty: "No models have traffic in this window yet.",
  });
  renderBreakdown("contributors-breakdown", contributorItems(summary, state.contributor), {
    accent: "cool",
    empty: "No contributor breakdown is available for this window yet.",
  });
  renderSessions(sessions.items);
  renderRequests(requests.items);
}

function renderError(message) {
  setStatus("API Error", "error");
  document.getElementById("last-refresh").textContent = message;
  document.getElementById("kpi-grid").innerHTML = `<div class="empty-state">${escapeHtml(message)}</div>`;
  document.getElementById("timeseries-chart").innerHTML = `<div class="chart-empty">The dashboard could not reach the local API.</div>`;
  document.getElementById("heatmap-panel").innerHTML = `<div class="empty-state">The heatmap will appear once the local API responds.</div>`;
  document.getElementById("models-breakdown").innerHTML = `<div class="empty-state">Waiting for the API.</div>`;
  document.getElementById("contributors-breakdown").innerHTML = `<div class="empty-state">Waiting for the API.</div>`;
  document.getElementById("sessions-list").innerHTML = `<div class="empty-state">Waiting for the API.</div>`;
  document.getElementById("requests-body").innerHTML = `<tr><td colspan="7" class="muted">Waiting for the API.</td></tr>`;
}

async function loadDashboard() {
  const token = ++state.requestToken;
  setStatus("Loading", "");

  try {
    const window = currentWindow(state.range);
    const previous = previousWindow(window);
    const timezoneOffset = new Date().getTimezoneOffset();
    const currentParams = buildParams(window);
    const previousParams = buildParams(previous);

    const [summary, previousSummary, timeseries, heatmap, sessions, requests] = await Promise.all([
      loadJSON(`/api/usage/summary?${currentParams}`),
      loadJSON(`/api/usage/summary?${previousParams}`),
      loadJSON(`/api/usage/timeseries?${buildParams(window, { range: state.range })}`),
      loadJSON(`/api/usage/heatmap?${buildParams(window, { range: state.range, tz_offset_minutes: String(timezoneOffset) })}`),
      loadJSON(`/api/sessions?${buildParams(window, { limit: String(window.spec.sessionLimit) })}`),
      loadJSON(`/api/requests?${buildParams(window, { limit: String(window.spec.requestLimit) })}`),
    ]);

    if (token !== state.requestToken) {
      return;
    }

    state.data = {
      window,
      summary,
      previousSummary,
      timeseries,
      heatmap,
      sessions,
      requests,
      lastRefresh: new Date(),
    };

    setStatus("Ready", "ready");
    renderDashboard();
  } catch (error) {
    console.error(error);
    if (token !== state.requestToken) {
      return;
    }
    renderError("The dashboard could not reach the local API yet.");
  }
}

function bindControls() {
  document.querySelectorAll("#range-control .segment").forEach((button) => {
    button.addEventListener("click", () => {
      if (button.dataset.range === state.range) {
        return;
      }
      state.range = button.dataset.range;
      loadDashboard();
    });
  });

  document.querySelectorAll("#metric-control .segment").forEach((button) => {
    button.addEventListener("click", () => {
      state.metric = button.dataset.metric;
      if (state.data) {
        renderDashboard();
      }
    });
  });

  document.querySelectorAll("#contributor-control .segment").forEach((button) => {
    button.addEventListener("click", () => {
      state.contributor = button.dataset.contributor;
      if (state.data) {
        renderDashboard();
      }
    });
  });
}

function boot() {
  bindControls();
  loadDashboard();
  window.setInterval(loadDashboard, 30000);
}

boot();
