// User dashboard SPA — tiếng Việt, reactive stats, fix copy.
const API = '/_dashboard/api';
let currentKey = localStorage.getItem('htn_key') || '';
let domain = '';

// --- CLI Callback Support ---
// When opened via "htn-tunnel login", the URL contains ?callback=http://127.0.0.1:PORT/cb
// After register/login, redirect the key back to the CLI callback server.

function isValidCallback(url) {
  try {
    const u = new URL(url);
    return u.hostname === '127.0.0.1';
  } catch { return false; }
}

function getCallbackURL() {
  // Check hash params: #register?callback=http://...
  const hash = window.location.hash;
  const hashMatch = hash.match(/callback=([^&]+)/);
  if (hashMatch) {
    const url = decodeURIComponent(hashMatch[1]);
    return isValidCallback(url) ? url : null;
  }
  // Check query string: ?callback=http://...
  const params = new URLSearchParams(window.location.search);
  const url = params.get('callback');
  if (url && isValidCallback(url)) return url;
  return null;
}

function redirectToCallback(key, name) {
  const cb = getCallbackURL();
  if (!cb) return false;
  const url = cb + '?key=' + encodeURIComponent(key) + '&name=' + encodeURIComponent(name || '');
  window.location.href = url;
  return true;
}

function headers() {
  return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + currentKey };
}

async function api(method, path, body) {
  const opts = { method, headers: headers() };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(API + path, opts);
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Yêu cầu thất bại');
  return data;
}

// --- Fetch domain on load ---
async function fetchDomain() {
  try {
    const info = await fetch(API + '/info').then(r => r.json());
    domain = info.domain || '';
    const suffix = document.getElementById('domain-suffix');
    if (suffix) suffix.textContent = '.' + domain;
  } catch { /* ignore */ }
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
    const res = await fetch(API + '/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: document.getElementById('reg-name').value,
        subdomain: document.getElementById('reg-subdomain').value.toLowerCase(),
      }),
    });
    const data = await res.json();
    if (data.error) { errEl.textContent = data.error; return false; }

    domain = data.domain || domain;

    // If opened from CLI login, redirect key back to callback server.
    if (redirectToCallback(data.key, document.getElementById('reg-name').value)) return false;

    document.getElementById('reg-key').textContent = data.key;
    const sub = (data.subdomains && data.subdomains[0]) || '';
    document.getElementById('reg-quickstart').textContent =
      `npm i -g htn-tunnel\nhtn-tunnel auth ${data.key} --server ${domain}:4443\nhtn-tunnel http 3000` +
      (sub ? `:${sub}` : '');
    document.getElementById('register-form').style.display = 'none';
    document.getElementById('reg-success').style.display = 'block';
  } catch (err) {
    errEl.textContent = err.message;
  }
  return false;
}

// --- Copy key (with fallback) ---
function copyKey() {
  const key = document.getElementById('reg-key').textContent;
  const btn = document.getElementById('copy-btn');
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(key).then(() => showCopied(btn)).catch(() => fallbackCopy(key, btn));
  } else {
    fallbackCopy(key, btn);
  }
}

function fallbackCopy(text, btn) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.cssText = 'position:fixed;opacity:0;left:-9999px';
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); showCopied(btn); }
  catch { btn.textContent = 'Lỗi'; }
  document.body.removeChild(ta);
}

function showCopied(btn) {
  btn.textContent = 'Đã sao chép!';
  btn.style.color = '#4ade80';
  setTimeout(() => { btn.textContent = 'Sao chép'; btn.style.color = ''; }, 2000);
}

// --- Login ---
async function handleLogin(e) {
  e.preventDefault();
  const key = document.getElementById('login-key').value.trim();
  const errEl = document.getElementById('login-error');
  errEl.textContent = '';
  try {
    const res = await fetch(API + '/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key }),
    });
    const data = await res.json();
    if (data.error) { errEl.textContent = data.error; return false; }
    // If opened from CLI login, redirect key back to callback server.
    if (redirectToCallback(key, data.name || '')) return false;
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
  if (logSSE) { logSSE.close(); logSSE = null; }
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
      (me.subdomains.length ? `:${me.subdomains[0]}` : '');
    renderSubdomains(me.subdomains);
    loadTunnels();
    loadTrafficStats();
    startLogStream();
  } catch {
    logout();
  }
}

function renderSubdomains(subs) {
  const el = document.getElementById('subdomains-list');
  if (!subs || !subs.length) {
    el.innerHTML = '<p class="empty">Chưa có subdomain nào</p>';
    return;
  }
  el.innerHTML = subs.map(s => `
    <div class="subdomain-item">
      <span class="name">${s}.${domain}</span>
      <span class="status offline" id="status-${s}">ngoại tuyến</span>
      <button onclick="removeSubdomain('${s}')">Xóa</button>
    </div>
  `).join('');
}

async function loadTunnels() {
  try {
    const tunnels = await api('GET', '/tunnels');
    const el = document.getElementById('tunnels-list');

    // Update stats
    let totalIn = 0, totalOut = 0;
    tunnels.forEach(t => { totalIn += t.bytes_in || 0; totalOut += t.bytes_out || 0; });
    document.getElementById('stat-tunnels').textContent = tunnels.length;
    document.getElementById('stat-in').textContent = fmtBytes(totalIn);
    document.getElementById('stat-out').textContent = fmtBytes(totalOut);

    // Reset all statuses to offline
    document.querySelectorAll('.status').forEach(s => {
      s.textContent = 'ngoại tuyến';
      s.className = 'status offline';
    });

    if (!tunnels.length) {
      el.innerHTML = '<p class="empty">Chưa có tunnel nào</p>';
      return;
    }

    el.innerHTML = tunnels.map(t => {
      // Mark subdomain as online
      const statusEl = document.getElementById('status-' + t.subdomain);
      if (statusEl) { statusEl.textContent = 'trực tuyến'; statusEl.className = 'status online'; }
      return `<div class="tunnel-item">
        <div class="route">${t.subdomain || ''}.${domain} → localhost:${t.local_port}</div>
        <div class="meta">Uptime: ${t.uptime} | ↓ ${fmtBytes(t.bytes_in)} ↑ ${fmtBytes(t.bytes_out)}</div>
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
  if (!confirm(`Xóa ${name}.${domain}?`)) return;
  try {
    const data = await api('DELETE', '/subdomains/' + name);
    renderSubdomains(data.subdomains);
  } catch (err) {
    alert(err.message);
  }
}

function fmtBytes(b) {
  if (!b || b < 0) return '0B';
  if (b < 1024) return b + 'B';
  if (b < 1048576) return (b / 1024).toFixed(1) + 'KB';
  return (b / 1048576).toFixed(1) + 'MB';
}

// --- Traffic Analytics ---
let logSSE = null;
const MAX_LOG_ROWS = 200;

async function loadTrafficStats() {
  try {
    const [traffic, topPaths] = await Promise.all([
      api('GET', '/stats/traffic'),
      api('GET', '/stats/top-paths'),
    ]);
    renderTrafficChart(traffic);
    renderStatusBreakdown(traffic);
    renderTopPaths(topPaths);
    updateAnalyticsStats(traffic);
  } catch { /* ignore */ }
}

function renderTrafficChart(buckets) {
  const el = document.getElementById('traffic-chart');
  if (!buckets || !buckets.length) { el.innerHTML = '<span class="empty">No data</span>'; return; }
  const maxReqs = Math.max(...buckets.map(b => b.reqs), 1);
  el.innerHTML = buckets.map(b => {
    const h = Math.max(2, (b.reqs / maxReqs) * 140);
    const t = new Date(b.ts).toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});
    return `<div class="chart-bar" style="height:${h}px" data-tooltip="${t}: ${b.reqs} reqs"></div>`;
  }).join('');
}

function renderStatusBreakdown(buckets) {
  const el = document.getElementById('status-breakdown');
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
      <span class="status-dot" style="background:${i.color}"></span>
      <span>${i.label}</span>
      <div class="status-bar"><div class="status-bar-fill" style="width:${(i.count/total*100)}%;background:${i.color}"></div></div>
      <span class="status-count">${i.count}</span>
    </div>
  `).join('');
}

function renderTopPaths(paths) {
  const el = document.getElementById('top-paths');
  if (!paths || !paths.length) { el.innerHTML = '<p class="empty">No data</p>'; return; }
  el.innerHTML = paths.map(p =>
    `<div class="path-row"><span class="path-name" title="${p.path}">${p.path}</span><span class="path-count">${p.count}</span></div>`
  ).join('');
}

function updateAnalyticsStats(buckets) {
  let totalReqs = 0, totalLatency = 0, reqsWithLatency = 0;
  buckets.forEach(b => {
    totalReqs += b.reqs;
    if (b.avg_ms > 0) { totalLatency += b.avg_ms * b.reqs; reqsWithLatency += b.reqs; }
  });
  document.getElementById('stat-reqs').textContent = totalReqs;
  document.getElementById('stat-latency').textContent =
    reqsWithLatency > 0 ? Math.round(totalLatency / reqsWithLatency) + 'ms' : '0ms';
}

function startLogStream() {
  if (logSSE) logSSE.close();
  logSSE = new EventSource(API + '/logs/stream?key=' + encodeURIComponent(currentKey));
  logSSE.onmessage = (e) => {
    try { addLogRow(JSON.parse(e.data)); } catch {}
  };
  logSSE.onerror = () => {
    setTimeout(() => { if (currentKey) startLogStream(); }, 5000);
  };
}

function addLogRow(log) {
  const tbody = document.getElementById('log-tbody');
  const empty = tbody.querySelector('.empty');
  if (empty) empty.parentElement.remove();
  const tr = document.createElement('tr');
  const t = new Date(log.ts).toLocaleTimeString();
  const sc = log.s >= 500 ? 'log-s5' : log.s >= 400 ? 'log-s4' : log.s >= 300 ? 'log-s3' : 'log-s2';
  const path = log.p.length > 35 ? log.p.slice(0, 32) + '...' : log.p;
  tr.innerHTML = `<td>${t}</td><td>${log.m}</td><td title="${log.p}">${path}</td><td class="${sc}">${log.s}</td><td>${log.d}ms</td><td>${fmtBytes(log.z)}</td>`;
  tbody.insertBefore(tr, tbody.firstChild);
  while (tbody.children.length > MAX_LOG_ROWS) tbody.removeChild(tbody.lastChild);
}

// --- Init ---
(async function init() {
  await fetchDomain();

  const callbackURL = getCallbackURL();

  if (currentKey) {
    try {
      const me = await api('GET', '/me');
      domain = me.domain || domain;
      // If callback present and already logged in, redirect key immediately.
      if (callbackURL) {
        redirectToCallback(currentKey, me.name || '');
        return;
      }
      loadPanel();
      return;
    } catch {
      localStorage.removeItem('htn_key');
      currentKey = '';
    }
  }

  // If callback present, show register form for CLI login flow.
  if (callbackURL) {
    showPage('register');
    return;
  }

  const hash = window.location.hash.replace('#', '') || 'landing';
  if (['register', 'login'].includes(hash)) showPage(hash);
  else showPage('landing');

  // Reactive: refresh tunnels every 3s when on panel
  setInterval(() => {
    if (document.getElementById('page-panel').style.display !== 'none') {
      loadTunnels();
    }
  }, 3000);

  setInterval(() => {
    if (document.getElementById('page-panel').style.display !== 'none') {
      loadTrafficStats();
    }
  }, 10000);
})();
