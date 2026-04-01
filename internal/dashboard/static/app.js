'use strict';

// Admin token is stored in sessionStorage (cleared on tab close).
// Never sent as a URL param — always in Authorization header.
let adminToken = sessionStorage.getItem('htn_admin_token') || '';

// ── Fetch helpers ─────────────────────────────────────────────────────────────

function apiFetch(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  if (adminToken) headers['Authorization'] = 'Bearer ' + adminToken;
  return fetch(path, { ...opts, headers });
}

async function fetchStats() {
  try {
    const res = await apiFetch('/_dashboard/api/stats');
    if (!res.ok) return;
    const data = await res.json();
    document.getElementById('stat-total').textContent = data.total_tunnels ?? 0;
    document.getElementById('stat-http').textContent  = data.active_http  ?? 0;
    document.getElementById('stat-tcp').textContent   = data.active_tcp   ?? 0;
    document.getElementById('stat-bw').textContent    = formatBytes(data.bytes_in + data.bytes_out);
  } catch (_) { /* network error; will retry */ }
}

async function fetchTunnels() {
  try {
    const res = await apiFetch('/_dashboard/api/tunnels');
    if (!res.ok) return;
    const tunnels = await res.json();
    renderTunnels(Array.isArray(tunnels) ? tunnels : []);
    document.getElementById('last-updated').textContent =
      'Last updated: ' + new Date().toLocaleTimeString();
  } catch (_) { /* network error; will retry */ }
}

// ── Render ────────────────────────────────────────────────────────────────────

function renderTunnels(tunnels) {
  const tbody = document.getElementById('tunnels-tbody');
  const killCol = document.getElementById('kill-col');

  if (adminToken) killCol.style.display = '';

  if (!tunnels.length) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty">No active tunnels</td></tr>';
    return;
  }

  tbody.innerHTML = tunnels.map(t => {
    const endpoint = t.type === 'http'
      ? `<span class="endpoint">${t.subdomain || '—'}</span>`
      : `<span class="endpoint">:${t.port || '—'}</span>`;
    const badge = t.type === 'http'
      ? '<span class="badge badge-http">HTTP</span>'
      : '<span class="badge badge-tcp">TCP</span>';
    const killBtn = adminToken
      ? `<button class="kill-btn" onclick="killTunnel('${escHtml(t.id)}')">Kill</button>`
      : '';
    return `<tr>
      <td>${badge}</td>
      <td>${endpoint}</td>
      <td class="muted">${t.local_port}</td>
      <td class="muted">${escHtml(t.token_prefix)}</td>
      <td class="muted">${escHtml(t.uptime)}</td>
      <td class="muted">${formatBytes(t.bytes_in)} / ${formatBytes(t.bytes_out)}</td>
      <td>${killBtn}</td>
    </tr>`;
  }).join('');
}

// ── Admin ─────────────────────────────────────────────────────────────────────

function promptAdminLogin() {
  const token = window.prompt('Enter admin token:');
  if (!token) return;
  adminToken = token.trim();
  sessionStorage.setItem('htn_admin_token', adminToken);
  document.getElementById('admin-status').textContent = 'Admin';
  document.getElementById('admin-login-btn').textContent = 'Logout';
  document.getElementById('admin-login-btn').onclick = adminLogout;
  refresh();
}

function adminLogout() {
  adminToken = '';
  sessionStorage.removeItem('htn_admin_token');
  document.getElementById('admin-status').textContent = '';
  document.getElementById('admin-login-btn').textContent = 'Admin Login';
  document.getElementById('admin-login-btn').onclick = promptAdminLogin;
  document.getElementById('kill-col').style.display = 'none';
  refresh();
}

async function killTunnel(id) {
  if (!window.confirm('Kill tunnel ' + id + '?')) return;
  try {
    const res = await apiFetch('/_dashboard/api/tunnels/' + encodeURIComponent(id) + '/kill',
      { method: 'POST' });
    if (!res.ok) {
      const msg = await res.text();
      alert('Failed: ' + msg);
      return;
    }
    refresh();
  } catch (e) {
    alert('Error: ' + e.message);
  }
}

// ── Utilities ─────────────────────────────────────────────────────────────────

function formatBytes(n) {
  if (!n || n < 0) return '0 B';
  if (n < 1024)       return n + ' B';
  if (n < 1048576)    return (n / 1024).toFixed(1) + ' KB';
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
  return (n / 1073741824).toFixed(2) + ' GB';
}

function escHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// ── Init ──────────────────────────────────────────────────────────────────────

function refresh() {
  fetchStats();
  fetchTunnels();
}

// Restore admin status indicator if token was in sessionStorage.
if (adminToken) {
  document.getElementById('admin-status').textContent = 'Admin';
  document.getElementById('admin-login-btn').textContent = 'Logout';
  document.getElementById('admin-login-btn').onclick = adminLogout;
}

refresh();
setInterval(refresh, 5000);
