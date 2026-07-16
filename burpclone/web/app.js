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
    if (btn.dataset.tab === 'rules') loadRules();
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

document.getElementById('export-json').addEventListener('click', () => {
  window.location.href = '/api/history/export?format=json';
});
document.getElementById('export-csv').addEventListener('click', () => {
  window.location.href = '/api/history/export?format=csv';
});

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
  renderInspectorParsed(e.URL, e.ReqHeaders);
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

// --- Inspector: encode/decode tools ---
function b64encodeUtf8(str) {
  return btoa(unescape(encodeURIComponent(str)));
}
function b64decodeUtf8(str) {
  return decodeURIComponent(escape(atob(str)));
}
function hexEncode(str) {
  return Array.from(new TextEncoder().encode(str)).map(b => b.toString(16).padStart(2, '0')).join(' ');
}
function hexDecode(str) {
  const bytes = str.trim().split(/\s+/).map(h => parseInt(h, 16));
  return new TextDecoder().decode(new Uint8Array(bytes));
}
function jwtDecode(token) {
  const parts = token.trim().split('.');
  if (parts.length < 2) throw new Error('Bukan format JWT yang valid (butuh minimal 2 bagian dipisah titik)');
  const header = JSON.parse(b64decodeUtf8(parts[0].replace(/-/g, '+').replace(/_/g, '/')));
  const payload = JSON.parse(b64decodeUtf8(parts[1].replace(/-/g, '+').replace(/_/g, '/')));
  return JSON.stringify({ header, payload }, null, 2) + '\n\n(signature tidak diverifikasi, cuma decode)';
}

const inspectorActions = {
  b64encode: (s) => b64encodeUtf8(s),
  b64decode: (s) => b64decodeUtf8(s),
  urlencode: (s) => encodeURIComponent(s),
  urldecode: (s) => decodeURIComponent(s),
  hexencode: (s) => hexEncode(s),
  hexdecode: (s) => hexDecode(s),
  jwtdecode: (s) => jwtDecode(s),
};

$all('.inspector-buttons button').forEach(btn => {
  btn.addEventListener('click', () => {
    const input = document.getElementById('insp-input').value;
    const output = document.getElementById('insp-output');
    try {
      output.textContent = inspectorActions[btn.dataset.action](input);
      output.classList.remove('error');
    } catch (e) {
      output.textContent = 'Error: ' + e.message;
      output.classList.add('error');
    }
  });
});

// --- Inspector: parsed request view (called from History detail) ---
function parseHeaderText(raw) {
  const lines = (raw || '').split('\r\n').filter(l => l.includes(':'));
  return lines.map(l => {
    const idx = l.indexOf(':');
    return { key: l.slice(0, idx).trim(), value: l.slice(idx + 1).trim() };
  });
}

function parseCookies(headers) {
  const cookieHeader = headers.find(h => h.key.toLowerCase() === 'cookie' || h.key.toLowerCase() === 'set-cookie');
  if (!cookieHeader) return [];
  return cookieHeader.value.split(';').map(pair => {
    const [k, ...rest] = pair.trim().split('=');
    return { key: k, value: rest.join('=') };
  }).filter(c => c.key);
}

function renderInspectorParsed(url, reqHeadersRaw) {
  const headers = parseHeaderText(reqHeadersRaw);
  const cookies = parseCookies(headers);
  let params = [];
  try {
    const u = new URL(url);
    params = Array.from(u.searchParams.entries()).map(([key, value]) => ({ key, value }));
  } catch (e) { /* invalid URL, skip */ }

  const table = (title, rows) => {
    if (!rows.length) return `<h4>${title}</h4><p class="hint">(kosong)</p>`;
    return `<h4>${title}</h4><table class="mini-table"><tbody>${rows.map(r => `<tr><td>${escapeHtml(r.key)}</td><td>${escapeHtml(r.value)}</td></tr>`).join('')}</tbody></table>`;
  };

  document.getElementById('insp-parsed').innerHTML =
    table('Headers', headers) + table('Query params', params) + table('Cookies', cookies);
}

// --- Inspector: encode/decode tools ---

// --- Match & Replace ---
async function loadRules() {
  const res = await fetch('/api/rules');
  const rules = await res.json();
  const tbody = document.getElementById('rules-body');
  tbody.innerHTML = '';
  (rules || []).forEach(rule => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td><input type="checkbox" ${rule.enabled ? 'checked' : ''} data-id="${rule.id}" class="rule-toggle"></td>
      <td>${escapeHtml(rule.target)}</td>
      <td><code>${escapeHtml(rule.match)}</code></td>
      <td><code>${escapeHtml(rule.replace)}</code></td>
      <td><button class="rule-delete" data-id="${rule.id}">Delete</button></td>
    `;
    tbody.appendChild(tr);
  });

  $all('.rule-toggle').forEach(cb => {
    cb.addEventListener('change', async () => {
      await fetch(`/api/rules/${cb.dataset.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: cb.checked }),
      });
    });
  });
  $all('.rule-delete').forEach(btn => {
    btn.addEventListener('click', async () => {
      await fetch(`/api/rules/${btn.dataset.id}`, { method: 'DELETE' });
      loadRules();
    });
  });
}

document.getElementById('rule-add').addEventListener('click', async () => {
  const target = document.getElementById('rule-target').value;
  const match = document.getElementById('rule-match').value;
  const replace = document.getElementById('rule-replace').value;
  if (!match) { alert('Match pattern wajib diisi'); return; }

  const res = await fetch('/api/rules', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target, match, replace }),
  });
  if (!res.ok) {
    alert('Gagal nambah rule: ' + await res.text());
    return;
  }
  document.getElementById('rule-match').value = '';
  document.getElementById('rule-replace').value = '';
  loadRules();
});

// --- Init ---
loadHistory();
connectWS();