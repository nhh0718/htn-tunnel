'use strict';

const ADMIN_API = '/_admin/api';
let adminToken = sessionStorage.getItem('htn_admin_token') || '';

function headers() {
  return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + adminToken };
}

async function api(method, path) {
  const res = await fetch(ADMIN_API + path, { method, headers: headers() });
  if (res.status === 401 || res.status === 403) { logout(); throw new Error('unauthorized'); }
  return res.json();
}

// --- Login ---
function handleLogin(e) {
  e.preventDefault();
  adminToken = document.getElementById('admin-key').value.trim();
  sessionStorage.setItem('htn_admin_token', adminToken);
  loadPanel();
  return false;
}

function logout() {
  if (adminSSE) { adminSSE.close(); adminSSE = null; }
  adminToken = '';
  sessionStorage.removeItem('htn_admin_token');
  document.getElementById('page-login').style.display = 'block';
  document.getElementById('page-panel').style.display = 'none';
}

// --- Panel ---
async function loadPanel() {
  try {
    const stats = await api('GET', '/stats');
    document.getElementById('page-login').style.display = 'none';
    document.getElementById('page-panel').style.display = 'block';
    renderStats(stats);
    loadKeys();
    loadTunnels();
    loadConfig();
    loadAdminTraffic();
    startAdminLogStream();
  } catch {
    document.getElementById('login-error').textContent = 'Invalid admin key';
  }
}

function renderStats(s) {
  document.getElementById('stat-keys').textContent = s.total_keys ?? 0;
  document.getElementById('stat-http').textContent = s.active_http ?? 0;
  document.getElementById('stat-tcp').textContent = s.active_tcp ?? 0;
  document.getElementById('stat-bw').textContent = fmtBytes((s.bytes_in || 0) + (s.bytes_out || 0));
}

// --- Tabs ---
function showTab(name) {
  document.querySelectorAll('.tab-content').forEach(el => el.style.display = 'none');
  document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
  document.getElementById('tab-' + name).style.display = 'block';
  event.target.classList.add('active');
}

// --- Keys ---
async function loadKeys() {
  try {
    const keys = await api('GET', '/keys');
    const tbody = document.getElementById('keys-tbody');
    if (!keys || !keys.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No registered users</td></tr>';
      return;
    }
    tbody.innerHTML = keys.map(k => `<tr>
      <td class="mono">${esc(k.key_preview)}</td>
      <td>${esc(k.name)}</td>
      <td class="subs">${(k.subdomains || []).join(', ') || '-'}</td>
      <td>${k.max_tunnels}</td>
      <td>${k.active
        ? '<span class="badge badge-active">Active</span>'
        : '<span class="badge badge-revoked">Revoked</span>'}</td>
      <td class="muted">${new Date(k.created_at).toLocaleDateString()}</td>
      <td>${k.active
        ? `<button class="btn-danger" onclick="revokeKey('${esc(k.key_preview)}')">Revoke</button>`
        : ''}</td>
    </tr>`).join('');
  } catch { /* retry */ }
}

async function revokeKey(preview) {
  if (!confirm('Revoke key ' + preview + '?')) return;
  try {
    await fetch(ADMIN_API + '/keys/' + encodeURIComponent(preview), {
      method: 'DELETE', headers: headers()
    });
    loadKeys();
    refresh();
  } catch (e) { alert(e.message); }
}

// --- Tunnels ---
async function loadTunnels() {
  try {
    const tunnels = await api('GET', '/tunnels');
    const tbody = document.getElementById('tunnels-tbody');
    if (!tunnels || !tunnels.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No active tunnels</td></tr>';
      return;
    }
    tbody.innerHTML = tunnels.map(t => {
      const badge = t.type === 'http'
        ? '<span class="badge badge-http">HTTP</span>'
        : '<span class="badge badge-tcp">TCP</span>';
      const endpoint = t.type === 'http' ? (t.subdomain || '-') : (':' + (t.port || '-'));
      return `<tr>
        <td>${badge}</td>
        <td class="mono">${esc(endpoint)}</td>
        <td>${t.local_port}</td>
        <td class="muted">${esc(t.token_prefix)}</td>
        <td class="muted">${esc(t.uptime)}</td>
        <td class="muted">${fmtBytes(t.bytes_in)} / ${fmtBytes(t.bytes_out)}</td>
        <td><button class="btn-kill" onclick="killTunnel('${esc(t.id)}')">Kill</button></td>
      </tr>`;
    }).join('');
  } catch { /* retry */ }
}

async function killTunnel(id) {
  if (!confirm('Kill tunnel ' + id + '?')) return;
  try {
    await fetch(ADMIN_API + '/tunnels/' + encodeURIComponent(id) + '/kill', {
      method: 'POST', headers: headers()
    });
    loadTunnels();
    refresh();
  } catch (e) { alert(e.message); }
}

// --- Config ---
async function loadConfig() {
  try {
    const cfg = await api('GET', '/config');
    document.getElementById('cfg-domain').value = cfg.domain || '';
    document.getElementById('cfg-max-tunnels').value = cfg.max_tunnels_per_token || 10;
    document.getElementById('cfg-rate-limit').value = cfg.rate_limit || 100;
    document.getElementById('cfg-global-rate-limit').value = cfg.global_rate_limit || 1000;
    document.getElementById('cfg-allow-reg').checked = cfg.allow_registration !== false;
  } catch { /* ignore */ }
}

function handleSaveConfig(e) {
  e.preventDefault();
  const msgEl = document.getElementById('config-msg');
  msgEl.textContent = '';
  msgEl.className = 'config-msg';

  const updates = {
    domain: document.getElementById('cfg-domain').value,
    max_tunnels_per_token: parseInt(document.getElementById('cfg-max-tunnels').value) || 10,
    rate_limit: parseInt(document.getElementById('cfg-rate-limit').value) || 100,
    global_rate_limit: parseInt(document.getElementById('cfg-global-rate-limit').value) || 1000,
    allow_registration: document.getElementById('cfg-allow-reg').checked,
  };

  fetch(ADMIN_API + '/config', {
    method: 'PUT',
    headers: headers(),
    body: JSON.stringify(updates),
  }).then(r => r.json()).then(data => {
    if (data.error) {
      msgEl.textContent = data.error;
      msgEl.className = 'config-msg error';
    } else {
      msgEl.textContent = 'Config saved successfully';
      msgEl.className = 'config-msg success';
    }
  }).catch(err => {
    msgEl.textContent = err.message;
    msgEl.className = 'config-msg error';
  });
  return false;
}

// --- Helpers ---
function fmtBytes(n) {
  if (!n || n < 0) return '0B';
  if (n < 1024) return n + 'B';
  if (n < 1048576) return (n / 1024).toFixed(1) + 'KB';
  return (n / 1048576).toFixed(1) + 'MB';
}

function esc(s) {
  return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

async function refresh() {
  try {
    const stats = await api('GET', '/stats');
    renderStats(stats);
    document.getElementById('last-updated').textContent = 'Updated: ' + new Date().toLocaleTimeString();
  } catch { /* ignore */ }
}

// --- Admin Traffic Analytics ---
let adminSSE = null;
const MAX_ADMIN_LOG_ROWS = 300;

async function loadAdminTraffic() {
  try {
    const [traffic, topPaths] = await Promise.all([
      api('GET', '/stats/traffic'),
      api('GET', '/stats/top-paths'),
    ]);
    renderAdminChart(traffic);
    renderAdminStatus(traffic);
    renderAdminTopPaths(topPaths);
    updateAdminAnalytics(traffic);
  } catch {}
}

function renderAdminChart(buckets) {
  const el = document.getElementById('admin-traffic-chart');
  if (!buckets || !buckets.length) { el.innerHTML = '<span class="empty">No data</span>'; return; }
  const maxReqs = Math.max(...buckets.map(b => b.reqs), 1);
  el.innerHTML = buckets.map(b => {
    const h = Math.max(2, (b.reqs / maxReqs) * 140);
    const t = new Date(b.ts).toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});
    return `<div class="chart-bar" style="height:${h}px" data-tooltip="${t}: ${b.reqs} reqs"></div>`;
  }).join('');
}

function renderAdminStatus(buckets) {
  const el = document.getElementById('admin-status-breakdown');
  let s2=0, s3=0, s4=0, s5=0;
  buckets.forEach(b => { s2+=b.s2xx; s3+=b.s3xx; s4+=b.s4xx; s5+=b.s5xx; });
  const total = s2+s3+s4+s5 || 1;
  const items = [
    { label: '2xx', count: s2, color: '#4ade80' },
    { label: '3xx', count: s3, color: '#38bdf8' },
    { label: '4xx', count: s4, color: '#fbbf24' },
    { label: '5xx', count: s5, color: '#f87171' },
  ];
  el.innerHTML = items.map(i => `
    <div class="status-row">
      <span class="status-dot" style="background:${i.color}"></span><span>${i.label}</span>
      <div class="status-bar"><div class="status-bar-fill" style="width:${(i.count/total*100)}%;background:${i.color}"></div></div>
      <span class="status-count">${i.count}</span>
    </div>
  `).join('');
}

function renderAdminTopPaths(paths) {
  const el = document.getElementById('admin-top-paths');
  if (!paths || !paths.length) { el.innerHTML = '<p class="empty">No data</p>'; return; }
  el.innerHTML = paths.map(p =>
    `<div class="path-row"><span class="path-name" title="${esc(p.path)}">${esc(p.path)}</span><span class="path-count">${p.count}</span></div>`
  ).join('');
}

function updateAdminAnalytics(buckets) {
  let totalReqs = 0, totalLatency = 0, reqsWithLatency = 0;
  buckets.forEach(b => {
    totalReqs += b.reqs;
    if (b.avg_ms > 0) { totalLatency += b.avg_ms * b.reqs; reqsWithLatency += b.reqs; }
  });
  document.getElementById('stat-reqs').textContent = totalReqs;
  document.getElementById('stat-latency').textContent =
    reqsWithLatency > 0 ? Math.round(totalLatency / reqsWithLatency) + 'ms' : '0ms';
}

function startAdminLogStream() {
  if (adminSSE) adminSSE.close();
  adminSSE = new EventSource(ADMIN_API + '/logs/stream?key=' + encodeURIComponent(adminToken));
  adminSSE.onmessage = (e) => {
    try { addAdminLogRow(JSON.parse(e.data)); } catch {}
  };
  adminSSE.onerror = () => {
    setTimeout(() => { if (adminToken) startAdminLogStream(); }, 5000);
  };
}

function addAdminLogRow(log) {
  const tbody = document.getElementById('admin-log-tbody');
  const empty = tbody.querySelector('.empty');
  if (empty) empty.parentElement.remove();
  const tr = document.createElement('tr');
  const t = new Date(log.ts).toLocaleTimeString();
  const sc = log.s >= 500 ? 'log-s5' : log.s >= 400 ? 'log-s4' : log.s >= 300 ? 'log-s3' : 'log-s2';
  const path = log.p.length > 30 ? log.p.slice(0, 27) + '...' : log.p;
  tr.innerHTML = `<td>${t}</td><td>${esc(log.sub||'-')}</td><td>${esc(log.m)}</td><td title="${esc(log.p)}">${esc(path)}</td><td class="${sc}">${log.s}</td><td>${log.d}ms</td><td>${fmtBytes(log.z)}</td><td class="muted">${esc(log.tok||'-')}</td>`;
  tbody.insertBefore(tr, tbody.firstChild);
  while (tbody.children.length > MAX_ADMIN_LOG_ROWS) tbody.removeChild(tbody.lastChild);
}

// --- Init ---
if (adminToken) {
  loadPanel();
} else {
  document.getElementById('page-login').style.display = 'block';
}
setInterval(() => {
  if (adminToken && document.getElementById('page-panel').style.display !== 'none') {
    refresh();
    loadTunnels();
    loadAdminTraffic();
  }
}, 5000);
