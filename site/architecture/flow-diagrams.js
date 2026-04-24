/* ============================================================
   htn-tunnel Architecture — Animated Flow Diagrams
   Self-contained module. No dependencies.
   Call initFlowDiagrams() on DOMContentLoaded.

   Animation model:
   - Each diagram has a cycleDuration (ms). Time wraps around.
   - Each packet has a start time t and traverses its path nodes.
   - packetDuration ms per full path (split equally per segment).
   - requestAnimationFrame loop; IntersectionObserver pauses off-screen.
   ============================================================ */

/* ── Diagram data ────────────────────────────────────────── */

const DIAGRAMS = {

  /* ── 1. Toàn cảnh tunnel ─────────────────────────────── */
  'tunnel-overview': {
    title: 'Luồng 1: HTTPS request qua tunnel',
    description: 'Gói tin đi từ Browser → VPS → Máy dev → LocalApp → ngược lại.',
    viewBox: { w: 760, h: 300 },
    nodes: [
      { id: 'browser',  icon: '💻', label: 'Browser',       x: 80,  y: 150 },
      { id: 'server',   icon: '☁️', label: 'Tunnel Server', x: 280, y: 150 },
      { id: 'client',   icon: '🖥', label: 'Tunnel Client', x: 480, y: 150 },
      { id: 'app',      icon: '⚡', label: 'LocalApp:3000', x: 680, y: 150 },
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
    packets: [
      { t: 0,    path: ['browser', 'server'], color: '#007AFF', label: 'HTTPS req' },
      { t: 700,  path: ['server',  'client'], color: '#007AFF', label: 'yamux fwd' },
      { t: 1400, path: ['client',  'app'],    color: '#007AFF', label: 'HTTP req' },
      { t: 2200, path: ['app',     'client'], color: '#34C759', label: 'Response' },
      { t: 2900, path: ['client',  'server'], color: '#34C759' },
      { t: 3600, path: ['server',  'browser'],color: '#34C759', label: 'HTTPS resp' },
    ],
    cycleDuration: 5000,
    packetDuration: 600,
  },

  /* ── 2. Client auth handshake ────────────────────────── */
  'handshake': {
    title: 'Luồng 2: Client kết nối server lần đầu',
    description: 'TLS handshake → Auth → yamux upgrade → control stream.',
    viewBox: { w: 760, h: 300 },
    nodes: [
      { id: 'client', icon: '🖥', label: 'Tunnel Client', x: 160, y: 150 },
      { id: 'server', icon: '☁️', label: 'Tunnel Server', x: 600, y: 150 },
    ],
    edges: [
      { from: 'client', to: 'server', curve: -45 },
      { from: 'server', to: 'client', curve: 45  },
    ],
    legend: [
      { color: '#fbbf24', label: 'TLS/Control' },
      { color: '#007AFF', label: 'Auth / yamux' },
      { color: '#34C759', label: 'Accepted' },
    ],
    packets: [
      { t: 0,    path: ['client', 'server'], color: '#fbbf24', label: 'ClientHello' },
      { t: 700,  path: ['server', 'client'], color: '#fbbf24', label: 'ServerHello+Cert' },
      { t: 1400, path: ['client', 'server'], color: '#fbbf24', label: 'TLS Finished' },
      { t: 2100, path: ['client', 'server'], color: '#007AFF', label: 'Auth token' },
      { t: 2800, path: ['server', 'client'], color: '#34C759', label: 'AuthOK' },
      { t: 3500, path: ['client', 'server'], color: '#007AFF', label: 'yamux open' },
      { t: 4000, path: ['server', 'client'], color: '#007AFF', label: 'yamux ack' },
      { t: 4500, path: ['client', 'server'], color: '#fbbf24', label: 'ctrl stream' },
    ],
    cycleDuration: 5500,
    packetDuration: 550,
  },

  /* ── 3. HTTP request qua yamux ───────────────────────── */
  'http-request': {
    title: 'Luồng 3: HTTP request qua yamux',
    description: 'Browser → SNI Router → Yamux Session → Client → LocalApp.',
    viewBox: { w: 760, h: 320 },
    nodes: [
      { id: 'browser',  icon: '💻', label: 'Browser',       x: 80,  y: 160 },
      { id: 'sni',      icon: '🔀', label: 'SNI Router',    x: 220, y: 160 },
      { id: 'yamux',    icon: '📦', label: 'Yamux Session', x: 380, y: 160 },
      { id: 'client',   icon: '🖥', label: 'Client',        x: 540, y: 160 },
      { id: 'app',      icon: '⚡', label: 'LocalApp',      x: 680, y: 160 },
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
      { t: 600,  path: ['sni',    'yamux'],   color: '#007AFF', label: 'open stream' },
      { t: 1200, path: ['yamux',  'client'],  color: '#007AFF', label: 'stream data' },
      { t: 1800, path: ['client', 'app'],     color: '#007AFF', label: 'HTTP req' },
      { t: 2500, path: ['app',    'client'],  color: '#34C759', label: 'HTTP resp' },
      { t: 3100, path: ['client', 'yamux'],   color: '#34C759' },
      { t: 3700, path: ['yamux',  'browser'], color: '#34C759', label: 'HTTPS resp' },
    ],
    cycleDuration: 5000,
    packetDuration: 520,
  },

  /* ── 4. DNS-01 cert issuance ─────────────────────────── */
  'tls-cert': {
    title: 'Luồng 4: Wildcard cert qua DNS-01 challenge',
    description: 'Server → Let\'s Encrypt → Cloudflare DNS → cert được cấp.',
    viewBox: { w: 760, h: 320 },
    nodes: [
      { id: 'server',    icon: '☁️', label: 'Tunnel Server',  x: 120, y: 160 },
      { id: 'le',        icon: '🔐', label: "Let's Encrypt",  x: 380, y: 80  },
      { id: 'cf',        icon: '🌐', label: 'Cloudflare DNS', x: 640, y: 160 },
    ],
    edges: [
      { from: 'server', to: 'le', curve: 0 },
      { from: 'le',     to: 'server', curve: 0 },
      { from: 'server', to: 'cf', curve: 0 },
      { from: 'cf',     to: 'server', curve: 0 },
      { from: 'le',     to: 'cf', curve: 0 },
    ],
    legend: [
      { color: '#007AFF', label: 'Request' },
      { color: '#fbbf24', label: 'Challenge' },
      { color: '#34C759', label: 'Success' },
    ],
    packets: [
      { t: 0,    path: ['server', 'le'],     color: '#007AFF', label: 'xin cert *.tunnel.com' },
      { t: 900,  path: ['le',     'server'], color: '#fbbf24', label: 'TXT challenge' },
      { t: 1700, path: ['server', 'cf'],     color: '#007AFF', label: 'add TXT record' },
      { t: 2500, path: ['cf',     'server'], color: '#34C759', label: 'DNS OK' },
      { t: 3200, path: ['server', 'le'],     color: '#007AFF', label: 'verify now' },
      { t: 4000, path: ['le',     'cf'],     color: '#fbbf24', label: 'DNS query' },
      { t: 4800, path: ['le',     'server'], color: '#34C759', label: 'cert 90d' },
      { t: 6000, path: ['server', 'cf'],     color: '#007AFF', label: 'delete TXT' },
    ],
    cycleDuration: 7500,
    packetDuration: 700,
  },

  /* ── 5. WebSocket upgrade ────────────────────────────── */
  'websocket': {
    title: 'Luồng 5: WebSocket upgrade + bidirectional data',
    description: 'HTTP Upgrade → 101 Switching Protocols → raw bytes hai chiều.',
    viewBox: { w: 760, h: 300 },
    nodes: [
      { id: 'browser', icon: '💻', label: 'Browser',       x: 80,  y: 150 },
      { id: 'server',  icon: '☁️', label: 'Tunnel Server', x: 280, y: 150 },
      { id: 'client',  icon: '🖥', label: 'Tunnel Client', x: 480, y: 150 },
      { id: 'app',     icon: '⚡', label: 'LocalApp',      x: 680, y: 150 },
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
      { t: 700,  path: ['server',  'client'], color: '#007AFF', label: 'yamux stream' },
      { t: 1400, path: ['client',  'app'],    color: '#007AFF', label: 'Upgrade fwd' },
      { t: 2100, path: ['app',     'client'], color: '#34C759', label: '101 Switching' },
      { t: 2800, path: ['client',  'server'], color: '#34C759', label: '101 back' },
      { t: 3500, path: ['server',  'browser'],color: '#34C759', label: '101 OK' },
      /* bidirectional WS frames */
      { t: 4200, path: ['browser', 'app'],    color: '#a78bfa', label: 'WS frame' },
      { t: 4600, path: ['app',     'browser'],color: '#a78bfa', label: 'WS frame' },
      { t: 5000, path: ['browser', 'app'],    color: '#a78bfa' },
      { t: 5300, path: ['app',     'browser'],color: '#a78bfa' },
    ],
    cycleDuration: 6200,
    packetDuration: 520,
  },
};

/* ── SVG helpers ─────────────────────────────────────────── */

const SVG_NS = 'http://www.w3.org/2000/svg';

function svgEl(tag, attrs) {
  const el = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  return el;
}

/* Compute control point for a quadratic bezier curve.
   curve > 0 = bow downward, < 0 = bow upward. */
function pathD(x1, y1, x2, y2, curve) {
  if (!curve) return `M${x1},${y1} L${x2},${y2}`;
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  /* perpendicular offset */
  const dx = x2 - x1, dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy) || 1;
  const cx = mx - (dy / len) * curve;
  const cy = my + (dx / len) * curve;
  return `M${x1},${y1} Q${cx},${cy} ${x2},${y2}`;
}

/* Point on a quadratic bezier at parameter t (0..1) */
function bezierPt(x1, y1, cx, cy, x2, y2, t) {
  const mt = 1 - t;
  return {
    x: mt * mt * x1 + 2 * mt * t * cx + t * t * x2,
    y: mt * mt * y1 + 2 * mt * t * cy + t * t * y2,
  };
}

/* Linear point between two nodes at parameter t (0..1) */
function linearPt(x1, y1, x2, y2, t) {
  return { x: x1 + (x2 - x1) * t, y: y1 + (y2 - y1) * t };
}

/* ── Build one diagram's DOM ─────────────────────────────── */

function buildDiagram(container, id, cfg) {
  const { w, h } = cfg.viewBox;

  /* Lookup node positions */
  const nodeMap = {};
  for (const n of cfg.nodes) nodeMap[n.id] = n;

  /* Lookup edge curve by from+to (bidirectional lookup) */
  const edgeCurveMap = {};
  for (const e of cfg.edges) {
    edgeCurveMap[`${e.from}__${e.to}`] = e.curve || 0;
    /* Reverse direction uses negative curve to mirror */
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
      <filter id="glow-${id}-${i}" x="-50%" y="-50%" width="200%" height="200%">
        <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
        <feMerge>
          <feMergeNode in="blur" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>`).join('')}
  `;
  svg.appendChild(defs);

  /* Build a color→filterId map for fast lookup during animation */
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

    const circle = svgEl('circle', {
      class: 'node-circle',
      r: 30,
      cx: 0, cy: 0,
    });

    /* Emoji icon — positioned slightly above centre */
    const icon = svgEl('text', {
      class: 'node-icon',
      x: 0, y: -2,
      'text-anchor': 'middle',
      'dominant-baseline': 'middle',
    });
    icon.textContent = n.icon;

    /* Label below circle */
    const label = svgEl('text', {
      class: 'node-label',
      x: 0,
      y: 38,
      'text-anchor': 'middle',
      'dominant-baseline': 'hanging',
    });
    label.textContent = n.label;

    g.appendChild(circle);
    g.appendChild(icon);
    g.appendChild(label);
    nodesG.appendChild(g);
  }
  svg.appendChild(nodesG);

  /* Packets group — JS populates circles here */
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
      /* Small background pill for label readability */
      const pill = svgEl('rect', {
        x: -28, y: -22, width: 56, height: 14,
        rx: 4,
        fill: 'rgba(10,15,30,0.75)',
      });
      const txt = svgEl('text', {
        class: 'packet-label-text',
        x: 0, y: -15,
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
  speedLabel.innerHTML = `Tốc độ: <span class="flow-speed-value">1×</span>`;
  const speedInput = document.createElement('input');
  speedInput.type = 'range';
  speedInput.className = 'flow-speed-input';
  speedInput.min = '0.5';
  speedInput.max = '2';
  speedInput.step = '0.25';
  speedInput.value = '1';
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
    playing: true,
    speed: 1,
    startTime: null,        /* performance.now() at cycle start */
    pausedAt: null,         /* cycle-time when paused */
    rafId: null,
    visible: true,
  };

  /* Compute packet position for cycleTime ms into cycle */
  function getPacketPos(pkt, cycleTime) {
    const start = pkt.t;
    const end = pkt.t + cfg.packetDuration;
    if (cycleTime < start || cycleTime >= end) return null;

    const progress = (cycleTime - start) / cfg.packetDuration; /* 0..1 */

    /* Multi-node path: split progress across segments */
    const nodeIds = pkt.path;
    const segCount = nodeIds.length - 1;
    const segProgress = progress * segCount;
    const segIdx = Math.min(Math.floor(segProgress), segCount - 1);
    const segT = segProgress - segIdx; /* 0..1 within segment */

    const fromId = nodeIds[segIdx];
    const toId = nodeIds[segIdx + 1];
    const fromNode = nodeMap[fromId];
    const toNode = nodeMap[toId];

    /* Find curve for this segment — check both directions */
    const curveKey = `${fromId}__${toId}`;
    const curve = edgeCurveMap[curveKey] || 0;

    if (curve !== 0) {
      /* Quadratic bezier */
      const ax = fromNode.x, ay = fromNode.y;
      const bx = toNode.x, by = toNode.y;
      const mx = (ax + bx) / 2, my = (ay + by) / 2;
      const dx = bx - ax, dy = by - ay;
      const len = Math.sqrt(dx * dx + dy * dy) || 1;
      const cx = mx - (dy / len) * curve;
      const cy = my + (dx / len) * curve;
      return bezierPt(ax, ay, cx, cy, bx, by, segT);
    } else {
      return linearPt(fromNode.x, fromNode.y, toNode.x, toNode.y, segT);
    }
  }

  /* Check reduced motion preference — pause by default */
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) {
    state.playing = false;
    playBtn.textContent = '▶ Play';
    playBtn.setAttribute('aria-label', 'Bắt đầu animation');
  }

  function tick(now) {
    if (!state.playing) return;

    if (state.startTime === null) state.startTime = now;
    const elapsed = (now - state.startTime) * state.speed;
    const cycleTime = elapsed % cfg.cycleDuration;

    /* Update each packet */
    cfg.packets.forEach((pkt, i) => {
      const el = packetEls[i];
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
    /* If resuming from pause, adjust startTime so cycle continues */
    if (state.pausedAt !== null) {
      state.startTime = performance.now() - state.pausedAt / state.speed;
      state.pausedAt = null;
    }
    state.rafId = requestAnimationFrame(tick);
  }

  function stopAnimation() {
    if (state.rafId) {
      cancelAnimationFrame(state.rafId);
      state.rafId = null;
    }
    /* Record where we are in the cycle */
    if (state.startTime !== null) {
      const elapsed = (performance.now() - state.startTime) * state.speed;
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
    state.pausedAt = null;
    if (state.playing && state.visible) {
      startAnimation();
    } else {
      /* Show initial frame with all packets hidden */
      cfg.packets.forEach((_, i) => packetEls[i].setAttribute('opacity', '0'));
    }
  });

  speedInput.addEventListener('input', () => {
    const newSpeed = parseFloat(speedInput.value);
    /* Preserve cycle position when speed changes */
    if (state.startTime !== null && state.playing) {
      const elapsed = (performance.now() - state.startTime) * state.speed;
      const cyclePos = elapsed % cfg.cycleDuration;
      state.startTime = performance.now() - cyclePos / newSpeed;
    } else if (state.pausedAt !== null) {
      /* pausedAt is already a cycle-position, no scaling needed */
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

/* ── Public init function ────────────────────────────────── */

function initFlowDiagrams() {
  const containers = document.querySelectorAll('.flow-diagram[data-diagram]');
  containers.forEach((el) => {
    const id = el.getAttribute('data-diagram');
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
