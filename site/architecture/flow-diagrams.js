/* ============================================================
   htn-tunnel Architecture — Animated Flow Diagrams
   Self-contained module. No dependencies.
   Call initFlowDiagrams() on DOMContentLoaded.

   Animation model:
   - Each diagram has a cycleDuration (ms). Time wraps around.
   - Each packet has a start time t and traverses its path nodes.
   - packetDuration ms per full path (split equally per segment).
   - requestAnimationFrame loop; IntersectionObserver pauses off-screen.
   - Default speed: 0.75x (slower, more readable).
   ============================================================ */

/* ── SVG Icon Library ────────────────────────────────────── */
/* All icons: viewBox 36×36 centered at (0,0).
   Stroke-based line icons, 2px stroke, round caps/joins.
   color: currentColor (inherited from node context). */

const ICONS = {

  /* Browser window: rounded rect + 3 traffic-light dots + URL bar */
  browser: {
    paths: [
      '<rect x="-16" y="-12" width="32" height="24" rx="3" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      '<line x1="-16" y1="-4" x2="16" y2="-4" stroke="currentColor" stroke-width="1.5" opacity="0.5"/>',
      '<circle cx="-11" cy="-8" r="1.5" fill="#ff5f57"/>',
      '<circle cx="-6"  cy="-8" r="1.5" fill="#febc2e"/>',
      '<circle cx="-1"  cy="-8" r="1.5" fill="#28c840"/>',
      '<rect x="-4" y="-9.5" width="14" height="3" rx="1.5" fill="none" stroke="currentColor" stroke-width="1" opacity="0.35"/>',
      '<rect x="-12" y="-1" width="24" height="3" rx="1" fill="none" stroke="currentColor" stroke-width="1" opacity="0.25"/>',
      '<rect x="-12" y="4"  width="16" height="3" rx="1" fill="none" stroke="currentColor" stroke-width="1" opacity="0.25"/>',
    ],
    accent: '#007AFF',
  },

  /* Server rack: 3 stacked slabs with LED dot on each */
  server: {
    paths: [
      '<rect x="-13" y="-12" width="26" height="7" rx="2" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      '<rect x="-13" y="-3"  width="26" height="7" rx="2" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      '<rect x="-13" y="6"   width="26" height="7" rx="2" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      '<circle cx="9" cy="-8.5" r="1.5" fill="#34C759"/>',
      '<circle cx="9" cy="0.5"  r="1.5" fill="#34C759"/>',
      '<circle cx="9" cy="9.5"  r="1.5" fill="currentColor" opacity="0.4"/>',
      '<rect x="-10" y="-10" width="12" height="3" rx="1" fill="none" stroke="currentColor" stroke-width="1" opacity="0.3"/>',
      '<rect x="-10" y="-1"  width="12" height="3" rx="1" fill="none" stroke="currentColor" stroke-width="1" opacity="0.3"/>',
    ],
    accent: '#5AC8FA',
  },

  /* Tunnel-server: cloud outline with bidirectional tunnel arrow */
  'tunnel-server': {
    paths: [
      /* cloud body */
      '<path d="M-8,8 H-11 A6,6 0 0,1 -13,-2 A6,6 0 0,1 -4,-8 A7,7 0 0,1 9,-10 A8,8 0 0,1 14,2 A5,5 0 0,1 10,8 Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>',
      /* tunnel arrow right */
      '<line x1="-6" y1="2" x2="6" y2="2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>',
      '<polyline points="3,-1 6,2 3,5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>',
    ],
    accent: '#5AC8FA',
  },

  /* Client / laptop: screen + keyboard base */
  client: {
    paths: [
      /* screen */
      '<rect x="-11" y="-12" width="22" height="15" rx="2" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      /* screen glare */
      '<rect x="-9" y="-10" width="18" height="11" rx="1" fill="none" stroke="currentColor" stroke-width="0.75" opacity="0.3"/>',
      /* hinge */
      '<line x1="-11" y1="3" x2="11" y2="3" stroke="currentColor" stroke-width="1.5" opacity="0.5"/>',
      /* base left */
      '<path d="M-14,3 L-11,10 H11 L14,3" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>',
      /* touchpad */
      '<rect x="-5" y="5" width="10" height="3" rx="1" fill="none" stroke="currentColor" stroke-width="1" opacity="0.4"/>',
    ],
    accent: '#007AFF',
  },

  /* App / code: <  /> brackets */
  app: {
    paths: [
      '<polyline points="-8,-7 -14,0 -8,7" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>',
      '<polyline points="8,-7 14,0 8,7"   fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>',
      '<line x1="4" y1="-9" x2="-4" y2="9" stroke="currentColor" stroke-width="2" stroke-linecap="round" opacity="0.85"/>',
    ],
    accent: '#34C759',
  },

  /* Localhost / terminal: window with ">_" prompt */
  localhost: {
    paths: [
      '<rect x="-14" y="-11" width="28" height="22" rx="3" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      '<line x1="-14" y1="-4" x2="14" y2="-4" stroke="currentColor" stroke-width="1.5" opacity="0.45"/>',
      '<circle cx="-10" cy="-7.5" r="1.2" fill="currentColor" opacity="0.5"/>',
      '<circle cx="-6"  cy="-7.5" r="1.2" fill="currentColor" opacity="0.5"/>',
      /* > prompt */
      '<polyline points="-10,-1 -6,2 -10,5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>',
      /* _ cursor */
      '<line x1="-4" y1="5" x2="2" y2="5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>',
    ],
    accent: '#34C759',
  },

  /* Yamux multiplexer: 3 thin lines merging into 1 thick */
  yamux: {
    paths: [
      /* 3 input lines */
      '<line x1="-14" y1="-7" x2="-2" y2="-7" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" opacity="0.9"/>',
      '<line x1="-14" y1="0"  x2="-2" y2="0"  stroke="currentColor" stroke-width="1.5" stroke-linecap="round" opacity="0.9"/>',
      '<line x1="-14" y1="7"  x2="-2" y2="7"  stroke="currentColor" stroke-width="1.5" stroke-linecap="round" opacity="0.9"/>',
      /* funnel shape */
      '<path d="M-2,-8 Q2,-4 2,-1 M-2,8 Q2,4 2,1" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      /* merged output */
      '<line x1="2" y1="0" x2="14" y2="0" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>',
    ],
    accent: '#a78bfa',
  },

  /* SNI Router: diamond with 4 outward arrows */
  'sni-router': {
    paths: [
      '<polygon points="0,-12 11,0 0,12 -11,0" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      /* 4 directional tick arrows */
      '<line x1="0" y1="-12" x2="0"  y2="-16" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      '<line x1="11" y1="0"  x2="15" y2="0"   stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      '<line x1="0"  y1="12" x2="0"  y2="16"  stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      '<line x1="-11" y1="0" x2="-15" y2="0"  stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      /* center dot */
      '<circle cx="0" cy="0" r="2.5" fill="currentColor"/>',
    ],
    accent: '#fbbf24',
  },

  /* Let's Encrypt: padlock with keyhole */
  'lets-encrypt': {
    paths: [
      /* shackle */
      '<path d="M-6,-4 V-8 A6,6 0 0,1 6,-8 V-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>',
      /* lock body */
      '<rect x="-9" y="-4" width="18" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>',
      /* keyhole */
      '<circle cx="0" cy="3" r="2.5" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      '<line x1="0" y1="5.5" x2="0" y2="9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
      /* checkmark badge */
      '<circle cx="7" cy="-7" r="4" fill="#34C759" stroke="#0d1520" stroke-width="1"/>',
      '<polyline points="5,-7 7,-5 10,-9" fill="none" stroke="#fff" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>',
    ],
    accent: '#34C759',
  },

  /* Cloudflare DNS: cloud with dot-dot-dot below */
  'cloudflare-dns': {
    paths: [
      /* cloud */
      '<path d="M-9,2 H-11 A5,5 0 0,1 -12,-5 A5,5 0 0,1 -4,-9 A6,6 0 0,1 8,-7 A7,7 0 0,1 12,1 A4,4 0 0,1 8,5 Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>',
      /* DNS dots */
      '<circle cx="-6" cy="9" r="1.8" fill="currentColor"/>',
      '<circle cx="0"  cy="9" r="1.8" fill="currentColor"/>',
      '<circle cx="6"  cy="9" r="1.8" fill="currentColor"/>',
      /* lightning bolt (Cloudflare brand hint) */
      '<polyline points="1,-5 -2,0 1,0 -1,5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7"/>',
    ],
    accent: '#f6821f',
  },

  /* Firewall: brick wall pattern */
  firewall: {
    paths: [
      /* row 1 bricks */
      '<rect x="-13" y="-11" width="11" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      '<rect x="1"   y="-11" width="11" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      /* row 2 bricks (offset) */
      '<rect x="-13" y="-3"  width="5"  height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      '<rect x="-6"  y="-3"  width="11" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      '<rect x="7"   y="-3"  width="5"  height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      /* row 3 bricks */
      '<rect x="-13" y="5"   width="11" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
      '<rect x="1"   y="5"   width="11" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/>',
    ],
    accent: '#ff6b6b',
  },

  /* NAT: bidirectional crossing arrows */
  nat: {
    paths: [
      /* right arrow (top-left to bottom-right) */
      '<line x1="-12" y1="-6" x2="12" y2="6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>',
      '<polyline points="6,2 12,6 8,11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>',
      /* left arrow (bottom-left to top-right) */
      '<line x1="-12" y1="6" x2="12" y2="-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" opacity="0.65"/>',
      '<polyline points="-6,-2 -12,-6 -8,-11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" opacity="0.65"/>',
    ],
    accent: '#fbbf24',
  },
};

/* Render icon SVG markup string for a given icon key */
function renderIcon(iconName) {
  const icon = ICONS[iconName] || ICONS.server;
  const accentStyle = icon.accent ? ` style="color:${icon.accent}"` : '';
  return `<g class="node-icon-svg"${accentStyle} aria-hidden="true">${icon.paths.join('')}</g>`;
}

/* ── Diagram data ────────────────────────────────────────── */
/* Timings: packetDuration doubled (1400ms base), cycleDurations +~80%,
   default speed 0.75x, 1s breath gap appended to each cycle. */

const DIAGRAMS = {

  /* ── 1. Toàn cảnh tunnel ─────────────────────────────── */
  'tunnel-overview': {
    title: 'Luồng 1: HTTPS request qua tunnel',
    description: 'Gói tin đi từ Browser → VPS → Máy dev → LocalApp → ngược lại.',
    viewBox: { w: 920, h: 360 },
    nodes: [
      { id: 'browser', icon: 'browser',       label: 'Browser',       x: 100,  y: 180 },
      { id: 'server',  icon: 'tunnel-server', label: 'Tunnel Server', x: 340,  y: 180 },
      { id: 'client',  icon: 'client',        label: 'Tunnel Client', x: 580,  y: 180 },
      { id: 'app',     icon: 'app',           label: 'LocalApp:3000', x: 820,  y: 180 },
    ],
    edges: [
      { from: 'browser', to: 'server', curve: 0 },
      { from: 'server',  to: 'client', curve: 0 },
      { from: 'client',  to: 'app',    curve: 0 },
    ],
    legend: [
      { color: '#007AFF', label: 'HTTPS Request' },
      { color: '#34C759', label: 'Response' },
    ],
    /* t values: ~500ms gap after each previous packet finishes (1400ms dur).
       Previous ends at t + 1400; next starts at t + 1400 + 500. */
    packets: [
      { t: 0,    path: ['browser', 'server'], color: '#007AFF', label: 'HTTPS req' },
      { t: 1900, path: ['server',  'client'], color: '#007AFF', label: 'yamux fwd' },
      { t: 3800, path: ['client',  'app'],    color: '#007AFF', label: 'HTTP req'  },
      { t: 5700, path: ['app',     'client'], color: '#34C759', label: 'Response'  },
      { t: 7600, path: ['client',  'server'], color: '#34C759' },
      { t: 9500, path: ['server',  'browser'],color: '#34C759', label: 'HTTPS resp'},
    ],
    /* cycle = last packet end + 1000ms breath: 9500+1400+1000 = 11900 */
    cycleDuration: 11900,
    packetDuration: 1400,
  },

  /* ── 2. Client auth handshake ────────────────────────── */
  'handshake': {
    title: 'Luồng 2: Client kết nối server lần đầu',
    description: 'TLS handshake → Auth → yamux upgrade → control stream.',
    viewBox: { w: 820, h: 360 },
    nodes: [
      { id: 'client', icon: 'client',        label: 'Tunnel Client', x: 180, y: 180 },
      { id: 'server', icon: 'tunnel-server', label: 'Tunnel Server', x: 640, y: 180 },
    ],
    edges: [
      { from: 'client', to: 'server', curve: -50 },
      { from: 'server', to: 'client', curve:  50 },
    ],
    legend: [
      { color: '#fbbf24', label: 'TLS/Control' },
      { color: '#007AFF', label: 'Auth / yamux' },
      { color: '#34C759', label: 'Accepted' },
    ],
    /* gap ~500ms between consecutive packets */
    packets: [
      { t: 0,    path: ['client', 'server'], color: '#fbbf24', label: 'ClientHello' },
      { t: 1900, path: ['server', 'client'], color: '#fbbf24', label: 'ServerHello+Cert' },
      { t: 3800, path: ['client', 'server'], color: '#fbbf24', label: 'TLS Finished' },
      { t: 5700, path: ['client', 'server'], color: '#007AFF', label: 'Auth token' },
      { t: 7600, path: ['server', 'client'], color: '#34C759', label: 'AuthOK' },
      { t: 9500, path: ['client', 'server'], color: '#007AFF', label: 'yamux open' },
      { t: 11400,path: ['server', 'client'], color: '#007AFF', label: 'yamux ack' },
      { t: 13300,path: ['client', 'server'], color: '#fbbf24', label: 'ctrl stream' },
    ],
    /* 13300 + 1400 + 1000 = 15700 */
    cycleDuration: 15700,
    packetDuration: 1400,
  },

  /* ── 3. HTTP request qua yamux ───────────────────────── */
  'http-request': {
    title: 'Luồng 3: HTTP request qua yamux',
    description: 'Browser → SNI Router → Yamux Session → Client → LocalApp.',
    viewBox: { w: 920, h: 380 },
    nodes: [
      { id: 'browser', icon: 'browser',    label: 'Browser',       x: 80,  y: 190 },
      { id: 'sni',     icon: 'sni-router', label: 'SNI Router',    x: 270, y: 190 },
      { id: 'yamux',   icon: 'yamux',      label: 'Yamux Session', x: 460, y: 190 },
      { id: 'client',  icon: 'client',     label: 'Client',        x: 650, y: 190 },
      { id: 'app',     icon: 'app',        label: 'LocalApp',      x: 840, y: 190 },
    ],
    edges: [
      { from: 'browser', to: 'sni',    curve: 0 },
      { from: 'sni',     to: 'yamux',  curve: 0 },
      { from: 'yamux',   to: 'client', curve: 0 },
      { from: 'client',  to: 'app',    curve: 0 },
    ],
    legend: [
      { color: '#007AFF', label: 'Request' },
      { color: '#34C759', label: 'Response' },
    ],
    packets: [
      { t: 0,    path: ['browser', 'sni'],    color: '#007AFF', label: 'HTTPS' },
      { t: 1900, path: ['sni',    'yamux'],   color: '#007AFF', label: 'open stream' },
      { t: 3800, path: ['yamux',  'client'],  color: '#007AFF', label: 'stream data' },
      { t: 5700, path: ['client', 'app'],     color: '#007AFF', label: 'HTTP req' },
      { t: 7600, path: ['app',    'client'],  color: '#34C759', label: 'HTTP resp' },
      { t: 9500, path: ['client', 'yamux'],   color: '#34C759' },
      { t: 11400,path: ['yamux',  'browser'], color: '#34C759', label: 'HTTPS resp' },
    ],
    /* 11400 + 1400 + 1000 = 13800 */
    cycleDuration: 13800,
    packetDuration: 1400,
  },

  /* ── 4. DNS-01 cert issuance ─────────────────────────── */
  'tls-cert': {
    title: 'Luồng 4: Wildcard cert qua DNS-01 challenge',
    description: 'Server → Let\'s Encrypt → Cloudflare DNS → cert được cấp.',
    viewBox: { w: 920, h: 400 },
    nodes: [
      { id: 'server', icon: 'tunnel-server',   label: 'Tunnel Server',  x: 140,  y: 200 },
      { id: 'le',     icon: 'lets-encrypt',    label: "Let's Encrypt",  x: 460,  y: 100 },
      { id: 'cf',     icon: 'cloudflare-dns',  label: 'Cloudflare DNS', x: 780,  y: 200 },
    ],
    edges: [
      { from: 'server', to: 'le',     curve: 0 },
      { from: 'le',     to: 'server', curve: 0 },
      { from: 'server', to: 'cf',     curve: 0 },
      { from: 'cf',     to: 'server', curve: 0 },
      { from: 'le',     to: 'cf',     curve: 0 },
    ],
    legend: [
      { color: '#007AFF', label: 'Request' },
      { color: '#fbbf24', label: 'Challenge' },
      { color: '#34C759', label: 'Success' },
    ],
    packets: [
      { t: 0,    path: ['server', 'le'],     color: '#007AFF', label: 'xin cert *.tunnel.com' },
      { t: 2400, path: ['le',     'server'], color: '#fbbf24', label: 'TXT challenge' },
      { t: 4800, path: ['server', 'cf'],     color: '#007AFF', label: 'add TXT record' },
      { t: 7200, path: ['cf',     'server'], color: '#34C759', label: 'DNS OK' },
      { t: 9600, path: ['server', 'le'],     color: '#007AFF', label: 'verify now' },
      { t: 12000,path: ['le',     'cf'],     color: '#fbbf24', label: 'DNS query' },
      { t: 14400,path: ['le',     'server'], color: '#34C759', label: 'cert 90d' },
      { t: 16800,path: ['server', 'cf'],     color: '#007AFF', label: 'delete TXT' },
    ],
    /* 16800 + 1400 + 1000 = 19200 */
    cycleDuration: 19200,
    packetDuration: 1400,
  },

  /* ── 5. WebSocket upgrade ────────────────────────────── */
  'websocket': {
    title: 'Luồng 5: WebSocket upgrade + bidirectional data',
    description: 'HTTP Upgrade → 101 Switching Protocols → raw bytes hai chiều.',
    viewBox: { w: 920, h: 360 },
    nodes: [
      { id: 'browser', icon: 'browser',       label: 'Browser',       x: 100,  y: 180 },
      { id: 'server',  icon: 'tunnel-server', label: 'Tunnel Server', x: 340,  y: 180 },
      { id: 'client',  icon: 'client',        label: 'Tunnel Client', x: 580,  y: 180 },
      { id: 'app',     icon: 'app',           label: 'LocalApp',      x: 820,  y: 180 },
    ],
    edges: [
      { from: 'browser', to: 'server', curve: 0 },
      { from: 'server',  to: 'client', curve: 0 },
      { from: 'client',  to: 'app',    curve: 0 },
    ],
    legend: [
      { color: '#007AFF', label: 'Upgrade req' },
      { color: '#34C759', label: '101 response' },
      { color: '#a78bfa', label: 'WS frames' },
    ],
    packets: [
      { t: 0,    path: ['browser', 'server'], color: '#007AFF', label: 'HTTP Upgrade' },
      { t: 1900, path: ['server',  'client'], color: '#007AFF', label: 'yamux stream' },
      { t: 3800, path: ['client',  'app'],    color: '#007AFF', label: 'Upgrade fwd' },
      { t: 5700, path: ['app',     'client'], color: '#34C759', label: '101 Switching' },
      { t: 7600, path: ['client',  'server'], color: '#34C759', label: '101 back' },
      { t: 9500, path: ['server',  'browser'],color: '#34C759', label: '101 OK' },
      /* bidirectional WS frames — shorter duration so they feel snappy */
      { t: 11400,path: ['browser', 'app'],    color: '#a78bfa', label: 'WS frame' },
      { t: 12800,path: ['app',     'browser'],color: '#a78bfa', label: 'WS frame' },
      { t: 14200,path: ['browser', 'app'],    color: '#a78bfa' },
      { t: 15600,path: ['app',     'browser'],color: '#a78bfa' },
    ],
    /* 15600 + 1400 + 1000 = 18000 */
    cycleDuration: 18000,
    packetDuration: 1400,
  },
};

/* ── SVG helpers ─────────────────────────────────────────── */

const SVG_NS = 'http://www.w3.org/2000/svg';

function svgEl(tag, attrs) {
  const el = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  return el;
}

/* Compute path D string for quadratic bezier or straight line.
   curve > 0 bows downward, < 0 bows upward. */
function pathD(x1, y1, x2, y2, curve) {
  if (!curve) return `M${x1},${y1} L${x2},${y2}`;
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  const dx = x2 - x1, dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy) || 1;
  const cx = mx - (dy / len) * curve;
  const cy = my + (dx / len) * curve;
  return `M${x1},${y1} Q${cx},${cy} ${x2},${y2}`;
}

/* Point on quadratic bezier at parameter t (0..1) */
function bezierPt(x1, y1, cx, cy, x2, y2, t) {
  const mt = 1 - t;
  return {
    x: mt * mt * x1 + 2 * mt * t * cx + t * t * x2,
    y: mt * mt * y1 + 2 * mt * t * cy + t * t * y2,
  };
}

/* Linear interpolation between two nodes at t (0..1) */
function linearPt(x1, y1, x2, y2, t) {
  return { x: x1 + (x2 - x1) * t, y: y1 + (y2 - y1) * t };
}

/* ── Build one diagram's DOM ─────────────────────────────── */

const NODE_RADIUS = 42; /* larger nodes */

function buildDiagram(container, id, cfg) {
  const { w, h } = cfg.viewBox;

  /* Node position lookup */
  const nodeMap = {};
  for (const n of cfg.nodes) nodeMap[n.id] = n;

  /* Edge curve lookup (bidirectional) */
  const edgeCurveMap = {};
  for (const e of cfg.edges) {
    edgeCurveMap[`${e.from}__${e.to}`] = e.curve || 0;
    if (!((`${e.to}__${e.from}`) in edgeCurveMap)) {
      edgeCurveMap[`${e.to}__${e.from}`] = -(e.curve || 0);
    }
  }

  /* ── Header ── */
  const header = document.createElement('div');
  header.className = 'flow-diagram-header';
  header.innerHTML = `
    <div class="flow-diagram-title">${cfg.title}</div>
    <p class="flow-diagram-desc">${cfg.description}</p>
  `;

  /* ── Legend ── */
  const legend = document.createElement('div');
  legend.className = 'flow-legend';
  legend.setAttribute('aria-label', 'Chú thích màu');
  legend.innerHTML = cfg.legend.map(l =>
    `<span class="flow-legend-item">
       <span class="flow-legend-dot" style="background:${l.color}"></span>
       ${l.label}
     </span>`
  ).join('');

  /* ── SVG ── */
  const svgWrap = document.createElement('div');
  svgWrap.className = 'flow-canvas';

  const svg = svgEl('svg', {
    viewBox: `0 0 ${w} ${h}`,
    role: 'img',
    'aria-label': cfg.title,
    preserveAspectRatio: 'xMidYMid meet',
  });

  /* Defs: arrow marker + per-color glow filters */
  const defs = svgEl('defs', {});
  const markerId = `arrow-${id}`;
  defs.innerHTML = `
    <marker id="${markerId}" markerWidth="8" markerHeight="8"
            refX="6" refY="3" orient="auto" markerUnits="strokeWidth">
      <path d="M0,0 L0,6 L8,3 z" fill="rgba(136,146,176,0.5)" />
    </marker>
    ${cfg.legend.map((l, i) => `
      <filter id="glow-${id}-${i}" x="-80%" y="-80%" width="260%" height="260%">
        <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
        <feMerge>
          <feMergeNode in="blur" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>`).join('')}
  `;
  svg.appendChild(defs);

  /* Color → filterId map for packet glow */
  const colorFilterMap = {};
  cfg.legend.forEach((l, i) => {
    colorFilterMap[l.color] = `url(#glow-${id}-${i})`;
  });

  /* Edges group */
  const edgesG = svgEl('g', { class: 'edges' });
  for (const e of cfg.edges) {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    const d = pathD(a.x, a.y, b.x, b.y, e.curve || 0);
    const path = svgEl('path', {
      class: 'flow-edge',
      d,
      'marker-end': `url(#${markerId})`,
    });
    edgesG.appendChild(path);
  }
  svg.appendChild(edgesG);

  /* Nodes group */
  const nodesG = svgEl('g', { class: 'nodes' });
  for (const n of cfg.nodes) {
    const g = svgEl('g', {
      class: 'node-group',
      transform: `translate(${n.x} ${n.y})`,
      role: 'img',
      'aria-label': n.label,
    });

    /* Background circle */
    const circle = svgEl('circle', {
      class: 'node-circle',
      r: NODE_RADIUS,
      cx: 0, cy: 0,
    });

    /* SVG icon group, centered slightly above centre */
    const iconWrapper = svgEl('g', {
      class: 'node-icon-wrapper',
      transform: 'translate(0 -4)',
    });
    iconWrapper.innerHTML = renderIcon(n.icon);

    /* Label below circle */
    const label = svgEl('text', {
      class: 'node-label',
      x: 0,
      y: NODE_RADIUS + 20,  /* 20px below circle edge */
      'text-anchor': 'middle',
      'dominant-baseline': 'hanging',
    });
    label.textContent = n.label;

    g.appendChild(circle);
    g.appendChild(iconWrapper);
    g.appendChild(label);
    nodesG.appendChild(g);
  }
  svg.appendChild(nodesG);

  /* Packets group — JS populates elements here during animation */
  const packetsG = svgEl('g', { class: 'packets' });
  svg.appendChild(packetsG);

  svgWrap.appendChild(svg);

  /* ── Create packet SVG elements ── */
  const packetEls = cfg.packets.map((pkt) => {
    const g = svgEl('g', { class: 'packet-group', opacity: '0' });

    const dot = svgEl('circle', {
      class: 'packet-dot',
      r: 7,
      fill: pkt.color,
      filter: colorFilterMap[pkt.color] || '',
    });

    g.appendChild(dot);

    if (pkt.label) {
      /* Background pill for readability */
      const pill = svgEl('rect', {
        x: -34, y: -26, width: 68, height: 16,
        rx: 5,
        fill: 'rgba(10,15,30,0.82)',
      });
      const txt = svgEl('text', {
        class: 'packet-label-text',
        x: 0, y: -18,
        'text-anchor': 'middle',
        'dominant-baseline': 'middle',
      });
      txt.textContent = pkt.label;
      g.appendChild(pill);
      g.appendChild(txt);
    }

    packetsG.appendChild(g);
    return g;
  });

  /* ── Controls ── */
  const controls = document.createElement('div');
  controls.className = 'flow-controls';
  controls.setAttribute('role', 'group');
  controls.setAttribute('aria-label', 'Điều khiển animation');

  const playBtn = document.createElement('button');
  playBtn.className = 'flow-btn play-pause';
  playBtn.setAttribute('aria-label', 'Tạm dừng animation');
  playBtn.textContent = '⏸ Pause';

  const resetBtn = document.createElement('button');
  resetBtn.className = 'flow-btn';
  resetBtn.setAttribute('aria-label', 'Reset animation');
  resetBtn.textContent = '⟳ Reset';

  const speedLabel = document.createElement('label');
  speedLabel.className = 'flow-speed-label';
  /* Default display: 0.75× */
  speedLabel.innerHTML = `Tốc độ: <span class="flow-speed-value">0.75×</span>`;

  const speedInput = document.createElement('input');
  speedInput.type = 'range';
  speedInput.className = 'flow-speed-input';
  speedInput.min   = '0.25';
  speedInput.max   = '2';
  speedInput.step  = '0.25';
  speedInput.value = '0.75'; /* default 0.75x */
  speedInput.setAttribute('aria-label', 'Tốc độ animation');
  speedLabel.appendChild(speedInput);

  controls.appendChild(resetBtn);
  controls.appendChild(playBtn);
  controls.appendChild(speedLabel);

  /* ── Assemble ── */
  container.appendChild(header);
  container.appendChild(legend);
  container.appendChild(svgWrap);
  container.appendChild(controls);

  /* ── Animation state ── */
  const state = {
    playing:   true,
    speed:     0.75, /* default 0.75x */
    startTime: null,
    pausedAt:  null,
    rafId:     null,
    visible:   true,
  };

  /* Compute packet SVG position at cycleTime ms into cycle */
  function getPacketPos(pkt, cycleTime) {
    const start = pkt.t;
    const end   = pkt.t + cfg.packetDuration;
    if (cycleTime < start || cycleTime >= end) return null;

    const progress = (cycleTime - start) / cfg.packetDuration; /* 0..1 */

    const nodeIds  = pkt.path;
    const segCount = nodeIds.length - 1;
    const segProgress = progress * segCount;
    const segIdx = Math.min(Math.floor(segProgress), segCount - 1);
    const segT   = segProgress - segIdx; /* 0..1 within segment */

    const fromNode = nodeMap[nodeIds[segIdx]];
    const toNode   = nodeMap[nodeIds[segIdx + 1]];
    const curveKey = `${nodeIds[segIdx]}__${nodeIds[segIdx + 1]}`;
    const curve    = edgeCurveMap[curveKey] || 0;

    if (curve !== 0) {
      const ax = fromNode.x, ay = fromNode.y;
      const bx = toNode.x,   by = toNode.y;
      const mx = (ax + bx) / 2, my = (ay + by) / 2;
      const dx = bx - ax, dy = by - ay;
      const len = Math.sqrt(dx * dx + dy * dy) || 1;
      const cx = mx - (dy / len) * curve;
      const cy = my + (dx / len) * curve;
      return bezierPt(ax, ay, cx, cy, bx, by, segT);
    }
    return linearPt(fromNode.x, fromNode.y, toNode.x, toNode.y, segT);
  }

  /* Respect prefers-reduced-motion */
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) {
    state.playing = false;
    playBtn.textContent = '▶ Play';
    playBtn.setAttribute('aria-label', 'Bắt đầu animation');
  }

  function tick(now) {
    if (!state.playing) return;
    if (state.startTime === null) state.startTime = now;

    const elapsed   = (now - state.startTime) * state.speed;
    const cycleTime = elapsed % cfg.cycleDuration;

    cfg.packets.forEach((pkt, i) => {
      const el  = packetEls[i];
      const pos = getPacketPos(pkt, cycleTime);
      if (pos) {
        el.setAttribute('opacity', '1');
        el.setAttribute('transform', `translate(${pos.x.toFixed(2)} ${pos.y.toFixed(2)})`);
      } else {
        el.setAttribute('opacity', '0');
      }
    });

    state.rafId = requestAnimationFrame(tick);
  }

  function startAnimation() {
    if (state.rafId) cancelAnimationFrame(state.rafId);
    if (state.pausedAt !== null) {
      state.startTime = performance.now() - state.pausedAt / state.speed;
      state.pausedAt  = null;
    }
    state.rafId = requestAnimationFrame(tick);
  }

  function stopAnimation() {
    if (state.rafId) {
      cancelAnimationFrame(state.rafId);
      state.rafId = null;
    }
    if (state.startTime !== null) {
      const elapsed  = (performance.now() - state.startTime) * state.speed;
      state.pausedAt = elapsed % cfg.cycleDuration;
    }
  }

  /* ── Controls wiring ── */
  playBtn.addEventListener('click', () => {
    state.playing = !state.playing;
    if (state.playing) {
      playBtn.textContent = '⏸ Pause';
      playBtn.setAttribute('aria-label', 'Tạm dừng animation');
      playBtn.classList.remove('active');
      if (state.visible) startAnimation();
    } else {
      playBtn.textContent = '▶ Play';
      playBtn.setAttribute('aria-label', 'Bắt đầu animation');
      playBtn.classList.add('active');
      stopAnimation();
    }
  });

  resetBtn.addEventListener('click', () => {
    state.startTime = null;
    state.pausedAt  = null;
    if (state.playing && state.visible) {
      startAnimation();
    } else {
      cfg.packets.forEach((_, i) => packetEls[i].setAttribute('opacity', '0'));
    }
  });

  speedInput.addEventListener('input', () => {
    const newSpeed = parseFloat(speedInput.value);
    /* Preserve cycle position when speed changes */
    if (state.startTime !== null && state.playing) {
      const elapsed  = (performance.now() - state.startTime) * state.speed;
      const cyclePos = elapsed % cfg.cycleDuration;
      state.startTime = performance.now() - cyclePos / newSpeed;
    }
    state.speed = newSpeed;
    speedLabel.querySelector('.flow-speed-value').textContent = `${newSpeed}×`;
  });

  /* ── IntersectionObserver — pause when off-screen ── */
  const observer = new IntersectionObserver(
    (entries) => {
      const visible = entries[0].isIntersecting;
      state.visible = visible;
      if (visible && state.playing) {
        startAnimation();
      } else {
        stopAnimation();
      }
    },
    { threshold: 0.1 }
  );
  observer.observe(container);
}

/* ── Public init ─────────────────────────────────────────── */

function initFlowDiagrams() {
  const containers = document.querySelectorAll('.flow-diagram[data-diagram]');
  containers.forEach((el) => {
    const id  = el.getAttribute('data-diagram');
    const cfg = DIAGRAMS[id];
    if (!cfg) {
      console.warn(`[flow-diagrams] Unknown diagram id: "${id}"`);
      return;
    }
    buildDiagram(el, id, cfg);
  });
}

/* Auto-init on DOMContentLoaded (or immediately if already loaded) */
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initFlowDiagrams);
} else {
  initFlowDiagrams();
}
