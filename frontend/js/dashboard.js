/**
 * Guardians of the Lake - Dashboard Controller
 * Pure Vanilla JS + Leaflet.js + Native WebSocket Client
 */

// 1. Initial Mock Data for Lake Victoria (Kisumu Bay)
const INITIAL_HOTSPOTS = [
  { id: 'hs-1', name: 'Algae Bloom Outbreak', lat: -0.1022, lng: 34.7523, severity: 'critical', color: '#ba1a1a', do: 3.1, turbidity: 48.6, location: 'Dunga Beach Pier' },
  { id: 'hs-2', name: 'Silt Turbidity Plume', lat: -0.2185, lng: 34.6150, severity: 'medium', color: '#5c2d00', do: 5.8, turbidity: 36.5, location: 'Kendu Bay Landing Site' },
  { id: 'hs-3', name: 'Clean Sanctuary Inlet', lat: -0.3421, lng: 34.8210, severity: 'safe', color: '#006c4e', do: 7.9, turbidity: 8.4, location: 'Homa Bay Inflow' }
];

const INITIAL_ACTIVITIES = [
  { icon: 'campaign', color: 'text-[#ba1a1a]', text: '<strong>High turbidity</strong> reported near Dunga Beach.', meta: '10 mins ago • Unverified' },
  { icon: 'verified', color: 'text-[#006c4e]', text: 'Report #8492 <strong>verified</strong> by local station.', meta: '45 mins ago • System' },
  { icon: 'science', color: 'text-[#0d3b66]', text: 'Routine water sample collected at Site A.', meta: '2 hours ago • Field Team' },
  { icon: 'science', color: 'text-[#0d3b66]', text: 'Routine water sample collected at Site B.', meta: '3 hours ago • Field Team' }
];

let map = null;

document.addEventListener('DOMContentLoaded', () => {
  initHealthGauge(72, 'Moderate');
  initLeafletMap();
  renderActivityFeed();
  initNativeWebSocket();
  initExportButton();
});

// 2. Render Circular Gauge
function initHealthGauge(score, status) {
  const scoreEl = document.getElementById('score-value');
  const statusEl = document.getElementById('score-status');
  const circle = document.getElementById('gauge-progress');

  if (scoreEl) scoreEl.textContent = score;
  if (statusEl) statusEl.textContent = status;

  if (circle) {
    const circumference = 2 * Math.PI * 40; // 251.32
    const offset = circumference - (score / 100) * circumference;
    circle.style.strokeDashoffset = offset;
  }
}

// 3. Leaflet.js Map Initialization centered on Kisumu Gulf
function initLeafletMap() {
  const mapElement = document.getElementById('leaflet-map');
  if (!mapElement) return;

  // Center on Kisumu Gulf, Lake Victoria
  map = L.map('leaflet-map', {
    zoomControl: true,
    attributionControl: false
  }).setView([-0.18, 34.72], 10);

  // High-performance OpenStreetMap Tile Layer
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 18,
  }).addTo(map);

  // Add Circular Marker Hotspots
  INITIAL_HOTSPOTS.forEach(hs => {
    const circle = L.circleMarker([hs.lat, hs.lng], {
      radius: hs.severity === 'critical' ? 10 : 8,
      fillColor: hs.color,
      color: '#ffffff',
      weight: 2,
      opacity: 1,
      fillOpacity: 0.9
    }).addTo(map);

    circle.bindPopup(`
      <div style="font-family: 'Work Sans', sans-serif; padding: 4px;">
        <span style="font-size: 11px; font-family: monospace; font-weight: bold; color: ${hs.color}; text-transform: uppercase;">${hs.severity} SEVERITY</span>
        <h4 style="font-size: 14px; font-weight: bold; margin: 4px 0 2px 0; color: #002546;">${hs.name}</h4>
        <p style="font-size: 12px; color: #737780; margin: 0 0 8px 0;">${hs.location}</p>
        <div style="font-family: monospace; font-size: 11px; display: flex; gap: 8px; background: #eff4ff; padding: 6px; border-radius: 6px;">
          <span>DO: <strong>${hs.do} mg/L</strong></span>
          <span>Turbidity: <strong>${hs.turbidity} NTU</strong></span>
        </div>
      </div>
    `);
  });
}

// 4. Render Activity List
function renderActivityFeed() {
  const container = document.getElementById('activity-feed-container');
  if (!container) return;

  container.innerHTML = INITIAL_ACTIVITIES.map(act => `
    <div class="pt-2 flex gap-3 cursor-pointer hover:bg-[#eff4ff] p-2 rounded transition-colors">
      <span class="material-symbols-outlined ${act.color} mt-0.5">${act.icon}</span>
      <div class="text-sm">
        <p class="text-[#0d1c2e]">${act.text}</p>
        <p class="font-mono text-xs text-[#737780] mt-1">${act.meta}</p>
      </div>
    </div>
  `).join('');
}

// 5. Native WebSocket Connection to Go (Fiber)
function initNativeWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/telemetry`;

  let ws;
  try {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('[WS] Connected to Go Fiber telemetry stream');
      updateWSStatus(true);
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.score) {
        initHealthGauge(data.score, data.status || 'Moderate');
      }
    };

    ws.onclose = () => {
      updateWSStatus(false);
      // Auto-fallback pulse simulation if Go backend is offline during test
      startMockTelemetryLoop();
    };

    ws.onerror = () => {
      updateWSStatus(false);
      startMockTelemetryLoop();
    };
  } catch (err) {
    startMockTelemetryLoop();
  }
}

function updateWSStatus(isOnline) {
  const dot = document.getElementById('ws-indicator-dot');
  const txt = document.getElementById('ws-status-text');
  if (txt) txt.textContent = isOnline ? 'LIVE' : 'SIMULATED';
}

function startMockTelemetryLoop() {
  setInterval(() => {
    const randomScore = Math.min(100, Math.max(50, Math.round(72 + (Math.random() * 4 - 2))));
    initHealthGauge(randomScore, randomScore >= 70 ? 'Good' : 'Moderate');
  }, 4000);
}

// 6. CSV Export Logic
function initExportButton() {
  const btn = document.getElementById('export-data-btn');
  if (!btn) return;

  btn.addEventListener('click', () => {
    const csvRows = [
      ['Report_ID', 'Title', 'Location', 'DO_mgL', 'Turbidity_NTU', 'Status'],
      ['#8493', 'High Turbidity Plume', 'Dunga Beach Pier', '4.1', '48.6', 'Unverified'],
      ['#8492', 'Microcystis Algae Bloom', 'Kendu Bay Landing', '3.2', '38.0', 'Verified'],
      ['#8491', 'Water Hyacinth Mat', 'Rusinga Narrows', '5.9', '22.4', 'Verified']
    ];

    const csvContent = csvRows.map(r => r.join(',')).join('\n');
    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'lake_victoria_telemetry.csv';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  });
}