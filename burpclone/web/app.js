const state = { activeTab: 'history' };

function $(sel) { return document.querySelector(sel); }
function $all(sel) { return document.querySelectorAll(sel); }

function escapeHtml(s) {
  return (s || '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

function atobSafe(b64) {
  if (!b64) return '';
  try { return atob(b64); } catch (e) { return '[binary data]'; }
}

// --- Tabs ---
$all('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    $all('.tab-btn').forEach(b => b.classList.remove('active'));
    $all('.tab').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
    state.activeTab = btn.dataset.tab;
    if (btn.dataset.tab === 'intercept') pollIntercept();
  });
});

// --- History ---
function renderHistoryRow(e, prepend) {
  const tbody = document.getElementById('history-body');
  const tr = document.createElement('tr');
  tr.innerHTML = `<td>${e.ID}</td><td>${escapeHtml(e.Method)}</td><td class="url">${escapeHtml(e.URL)}</td><td>${e.StatusCode}</td><td>${new Date(e.Timestamp).toLocaleTimeString()}</td>`;
  tr.addEventListener('click', () => showHistoryDetail(e.ID));
  if (prepend) tbody.prepend(tr); else tbody.appendChild(tr);
}

async function loadHistory() {
  const res = await fetch('/api/history?limit=50');
  if (!res.ok) return;
  const entries = await res.json();
  const tbody = document.getElementById('history-body');
  tbody.innerHTML = '';
  (entries || []).forEach(e => renderHistoryRow(e, false));
}

async function showHistoryDetail(id) {
  const res = await fetch('/api/history/' + id);
  if (!res.ok) return;
  const e = await res.json();
  document.getElementById('history-detail').innerHTML = `
    <h3>#${e.ID} ${escapeHtml(e.Method)} ${escapeHtml(e.URL)}</h3>
    <h4>Request headers</h4><pre>${escapeHtml(e.ReqHeaders)}</pre>
    <h4>Request body</h4><pre>${escapeHtml(atobSafe(e.ReqBody))}</pre>
    <h4>Response headers</h4><pre>${escapeHtml(e.RespHeaders)}</pre>
    <h4>Response body</h4><pre>${escapeHtml(atobSafe(e.RespBody))}</pre>
  `;
}

// --- WebSocket live feed ---
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onmessage = (ev) => {
    try {
      renderHistoryRow(JSON.parse(ev.data), true);
    } catch (e) { console.error('ws parse error', e); }
  };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

// --- Intercept ---
document.getElementById('intercept-toggle').addEventListener('click', async () => {
  const res = await fetch('/api/intercept/toggle', { method: 'POST' });
  const data = await res.json();
  document.getElementById('intercept-state').textContent = data.on ? 'ON' : 'OFF';
});

async function pollIntercept() {
  if (state.activeTab !== 'intercept') return;
  const res = await fetch('/api/intercept');
  const list = await res.json();
  const container = document.getElementById('intercept-list');
  container.innerHTML = '';
  (list || []).forEach(req => {
    const div = document.createElement('div');
    div.className = 'intercept-item';
    div.innerHTML = `
      <h4>#${req.id} ${escapeHtml(req.method)} ${escapeHtml(req.url)}</h4>
      <textarea class="int-headers" rows="4">${escapeHtml(req.headers)}</textarea>
      <textarea class="int-body" rows="4">${escapeHtml(req.body)}</textarea>
      <button class="int-forward">Forward</button>
      <button class="int-drop">Drop</button>
    `;
    div.querySelector('.int-forward').addEventListener('click', () => resolveIntercept(req.id, 'forward', div));
    div.querySelector('.int-drop').addEventListener('click', () => resolveIntercept(req.id, 'drop', div));
    container.appendChild(div);
  });
  setTimeout(pollIntercept, 1000);
}

async function resolveIntercept(id, action, div) {
  const headers = div.querySelector('.int-headers').value;
  const body = div.querySelector('.int-body').value;
  await fetch(`/api/intercept/${id}/${action}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ headers, body }),
  });
}

// --- Repeater ---
document.getElementById('rep-send').addEventListener('click', async () => {
  const method = document.getElementById('rep-method').value;
  const url = document.getElementById('rep-url').value;
  const headers = document.getElementById('rep-headers').value;
  const body = document.getElementById('rep-body').value;
  const res = await fetch('/api/repeater/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method, url, headers, body }),
  });
  const data = await res.json();
  document.getElementById('rep-response').innerHTML = `
    <h4>Status: ${data.status_code} (${data.duration_ms}ms)</h4>
    ${data.error ? `<p class="error">${escapeHtml(data.error)}</p>` : ''}
    <h4>Headers</h4><pre>${escapeHtml(data.headers)}</pre>
    <h4>Body</h4><pre>${escapeHtml(data.body)}</pre>
  `;
});

// --- Init ---
loadHistory();
connectWS();