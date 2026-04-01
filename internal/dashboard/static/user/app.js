// User dashboard SPA — hash-based routing, API key stored in localStorage.
const API = '/_dashboard/api';
let currentKey = localStorage.getItem('htn_key') || '';
let domain = '';

function headers() {
  return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + currentKey };
}

async function api(method, path, body) {
  const opts = { method, headers: headers() };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(API + path, opts);
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Request failed');
  return data;
}

// --- Pages ---
function showPage(name) {
  document.querySelectorAll('.page').forEach(p => p.style.display = 'none');
  const page = document.getElementById('page-' + name);
  if (page) page.style.display = 'block';
  document.getElementById('user-nav').style.display = (name === 'panel') ? 'flex' : 'none';
  window.location.hash = name;
}

// --- Register ---
async function handleRegister(e) {
  e.preventDefault();
  const errEl = document.getElementById('reg-error');
  errEl.textContent = '';
  try {
    const data = await fetch(API + '/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: document.getElementById('reg-name').value,
        subdomain: document.getElementById('reg-subdomain').value.toLowerCase(),
      }),
    }).then(r => r.json());

    if (data.error) { errEl.textContent = data.error; return false; }

    domain = data.domain || domain;
    document.getElementById('reg-key').textContent = data.key;
    const sub = (data.subdomains && data.subdomains[0]) || '';
    document.getElementById('reg-quickstart').textContent =
      `npm i -g htn-tunnel\nhtn-tunnel auth ${data.key} --server ${domain}:4443\nhtn-tunnel http 3000` +
      (sub ? ` --subdomain ${sub}` : '');
    document.getElementById('register-form').style.display = 'none';
    document.getElementById('reg-success').style.display = 'block';
  } catch (err) {
    errEl.textContent = err.message;
  }
  return false;
}

function copyKey() {
  const key = document.getElementById('reg-key').textContent;
  navigator.clipboard.writeText(key);
}

// --- Login ---
async function handleLogin(e) {
  e.preventDefault();
  const key = document.getElementById('login-key').value.trim();
  const errEl = document.getElementById('login-error');
  errEl.textContent = '';
  try {
    const data = await fetch(API + '/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key }),
    }).then(r => r.json());

    if (data.error) { errEl.textContent = data.error; return false; }
    loginWithKey(key);
  } catch (err) {
    errEl.textContent = err.message;
  }
  return false;
}

function loginWithKey(key) {
  currentKey = key;
  localStorage.setItem('htn_key', key);
  loadPanel();
}

function logout() {
  currentKey = '';
  localStorage.removeItem('htn_key');
  showPage('landing');
}

// --- Panel ---
async function loadPanel() {
  showPage('panel');
  try {
    const me = await api('GET', '/me');
    domain = me.domain || domain;
    document.getElementById('user-name').textContent = me.name;
    document.getElementById('panel-key').textContent = currentKey;
    document.getElementById('panel-quickstart').textContent =
      `htn-tunnel auth ${currentKey} --server ${domain}:4443\nhtn-tunnel http 3000` +
      (me.subdomains.length ? ` --subdomain ${me.subdomains[0]}` : '');
    renderSubdomains(me.subdomains);
    loadTunnels();
  } catch {
    logout();
  }
}

function renderSubdomains(subs) {
  const el = document.getElementById('subdomains-list');
  if (!subs || !subs.length) {
    el.innerHTML = '<p class="empty">No subdomains claimed yet</p>';
    return;
  }
  el.innerHTML = subs.map(s => `
    <div class="subdomain-item">
      <span class="name">${s}.${domain}</span>
      <span class="status offline" id="status-${s}">offline</span>
      <button onclick="removeSubdomain('${s}')">Remove</button>
    </div>
  `).join('');
}

async function loadTunnels() {
  try {
    const tunnels = await api('GET', '/tunnels');
    const el = document.getElementById('tunnels-list');
    if (!tunnels.length) {
      el.innerHTML = '<p class="empty">No active tunnels</p>';
      return;
    }
    el.innerHTML = tunnels.map(t => {
      // Mark subdomain as online
      const statusEl = document.getElementById('status-' + t.subdomain);
      if (statusEl) { statusEl.textContent = 'online'; statusEl.className = 'status online'; }
      return `<div class="tunnel-item">
        <div class="route">${t.subdomain}.${domain} → localhost:${t.local_port}</div>
        <div class="meta">Uptime: ${t.uptime} | In: ${fmtBytes(t.bytes_in)} | Out: ${fmtBytes(t.bytes_out)}</div>
      </div>`;
    }).join('');
  } catch { /* ignore */ }
}

async function handleAddSubdomain(e) {
  e.preventDefault();
  const input = document.getElementById('add-sub-input');
  const errEl = document.getElementById('sub-error');
  errEl.textContent = '';
  try {
    const data = await api('POST', '/subdomains', { subdomain: input.value.toLowerCase() });
    input.value = '';
    renderSubdomains(data.subdomains);
    loadTunnels();
  } catch (err) {
    errEl.textContent = err.message;
  }
  return false;
}

async function removeSubdomain(name) {
  if (!confirm(`Remove ${name}.${domain}?`)) return;
  try {
    const data = await api('DELETE', '/subdomains/' + name);
    renderSubdomains(data.subdomains);
  } catch (err) {
    alert(err.message);
  }
}

function fmtBytes(b) {
  if (b < 1024) return b + 'B';
  if (b < 1048576) return (b / 1024).toFixed(1) + 'KB';
  return (b / 1048576).toFixed(1) + 'MB';
}

// --- Init ---
(async function init() {
  // Fetch domain from meta or register response
  const suffix = document.getElementById('domain-suffix');

  // Try to get domain from a quick login check
  if (currentKey) {
    try {
      const me = await api('GET', '/me');
      domain = me.domain || '';
      if (suffix) suffix.textContent = '.' + domain;
      loadPanel();
      return;
    } catch {
      localStorage.removeItem('htn_key');
      currentKey = '';
    }
  }

  // Not logged in — show landing or hash page
  const hash = window.location.hash.replace('#', '') || 'landing';
  if (['register', 'login'].includes(hash)) {
    showPage(hash);
  } else {
    showPage('landing');
  }

  // Auto-refresh tunnels every 5s when on panel
  setInterval(() => {
    if (document.getElementById('page-panel').style.display !== 'none') {
      loadTunnels();
    }
  }, 5000);
})();
