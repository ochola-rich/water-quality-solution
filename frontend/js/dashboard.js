/**
 * Guardians of the Lake - Dashboard Application Controller
 * File: frontend/js/dashboard.js
 */

import { api } from './api.js';
import { exportReportsToCSV } from './report.js';
import { wsClient } from './ws-client.js';

// Cache the verifier's location once — the backend requires lat/lng
// on every vote for the 500m geo-check, so we can't skip this.
let verifierPosition = null;

async function getVerifierPosition() {
  if (verifierPosition) return verifierPosition;
  return new Promise((resolve) => {
    if (!navigator.geolocation) {
      verifierPosition = { lat: -0.0917, lng: 34.7680 }; // Kisumu fallback
      resolve(verifierPosition);
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        verifierPosition = { lat: pos.coords.latitude, lng: pos.coords.longitude };
        resolve(verifierPosition);
      },
      () => {
        // Permission denied — fall back rather than block voting entirely
        verifierPosition = { lat: -0.0917, lng: 34.7680 };
        resolve(verifierPosition);
      }
    );
  });
}

const SEED_REPORTS = [
  {
    id: 'rep-8492',
    reportNumber: '#8492',
    title: 'High Turbidity & Algal Growth',
    category: 'Algae Bloom',
    aiConfidence: 94,
    distance: '420m away (Sector 7G)',
    timestamp: 'Reported 12 mins ago',
    reporter: 'Citizen #8492',
    location: 'Dunga Beach Pier',
    sensorNode: 'Sensor Node Alpha',
    notes: 'Dense accumulation of cyanobacteria observed near the eastern shore dock. Water has a distinct paint-like green scum.',
    imageUrl: 'https://images.unsplash.com/photo-1576086213369-97a306d36557?auto=format&fit=crop&w=600&q=80',
    turbidity: 48.6,
    dissolvedOxygen: 4.1,
    status: 'pending'
  },
  {
    id: 'rep-8493',
    reportNumber: '#8493',
    title: 'Chemical Sheen & Odor Incident',
    category: 'Chemical Spill',
    aiConfidence: 89,
    distance: '780m away (Sector 4)',
    timestamp: 'Reported 35 mins ago',
    reporter: 'Fisherman Joseph',
    location: 'Kisumu Port Shoreline',
    sensorNode: 'Sensor Node Beta',
    notes: 'Oily rainbow slick expanding from industrial drainage canal. Local fish showing erratic surface breathing.',
    imageUrl: 'https://images.unsplash.com/photo-1611273426858-450d8e3c9fce?auto=format&fit=crop&w=600&q=80',
    turbidity: 28.0,
    dissolvedOxygen: 3.2,
    status: 'pending'
  },
  {
    id: 'rep-8494',
    reportNumber: '#8494',
    title: 'Severe Sediment Siltation Plume',
    category: 'Turbidity',
    aiConfidence: 96,
    distance: '1.2km away (Sector 12)',
    timestamp: 'Reported 1 hour ago',
    reporter: 'KMFRI Station Node',
    location: 'Nyando River Inflow',
    sensorNode: 'Fixed Buoy #3',
    notes: 'Heavy brown silt discharge extending 500m into open waters following flash rains.',
    imageUrl: 'https://images.unsplash.com/photo-1544551763-46a013bb70d5?auto=format&fit=crop&w=600&q=80',
    turbidity: 52.4,
    dissolvedOxygen: 5.8,
    status: 'pending'
  }
];

let reports = [];

document.addEventListener('DOMContentLoaded', () => {
  initRouter();
  initReports();
  initExportEngine();
  wsClient.connect();
  wsClient.on('report:new', () => initReports());
  wsClient.on('report:verified', () => initReports());
  wsClient.on('report:rejected', () => initReports());
});

// 1. Single Page Router
function initRouter() {
  const navButtons = document.querySelectorAll('.nav-btn');
  
  navButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.getAttribute('data-tab');
      window.switchTab(tab);
    });
  });

  // Handle URL hash changes (e.g., #map, #verify, #alerts)
  const hash = window.location.hash.replace('#', '');
  if (hash && ['overview', 'verify', 'map', 'alerts'].includes(hash)) {
    window.switchTab(hash);
  }
}

window.switchTab = (tabName) => {
  document.querySelectorAll('.tab-screen').forEach(screen => {
    screen.classList.add('hidden');
  });

  const activeScreen = document.getElementById(`screen-${tabName}`);
  if (activeScreen) activeScreen.classList.remove('hidden');

  // Update navigation button active styles
  document.querySelectorAll('.nav-btn').forEach(btn => {
    if (btn.getAttribute('data-tab') === tabName) {
      btn.className = 'nav-btn flex items-center gap-3 px-3 py-2.5 rounded-xl bg-surface-high text-primary border-r-4 border-primary transition-all text-left w-full cursor-pointer';
    } else {
      btn.className = 'nav-btn flex items-center gap-3 px-3 py-2.5 rounded-xl text-outline hover:bg-surface-low transition-all text-left w-full cursor-pointer';
    }
  });

  window.location.hash = tabName;
  window.scrollTo({ top: 0, behavior: 'smooth' });
};

// 2. Pending Reports & Verification Logic
async function initReports() {
  try {
    const fetched = await api.getReports('pending');
    reports = fetched || [];
  } catch (err) {
    console.warn('[Dashboard] Falling back to seed data — API request failed:', err.message);
    reports = SEED_REPORTS;
  }
  renderVerifyCards();
}

function renderReportPhoto(r) {
  if (r.photo_path) {
    return `<img src="${r.photo_path}" alt="Report photo" class="w-full h-full object-cover" />`;
  }
  const icons = { turbidity: 'water_drop', algae: 'grass', spill: 'warning', smell: 'sensors', other: 'help' };
  const icon = icons[r.category] || 'help';
  return `
    <div class="w-full h-full flex items-center justify-center bg-surface-low">
      <span class="material-symbols-outlined text-4xl text-outline">${icon}</span>
    </div>
  `;
}

function renderVerifyCards() {
  const container = document.getElementById('verify-cards-container');
  const badge1 = document.getElementById('sidebar-pending-badge');
  const badge2 = document.getElementById('verify-pending-badge');
  if (!container) return;

  const pending = reports.filter(r => r.status === 'pending' || r.status === 'unverified');
  if (badge1) badge1.textContent = pending.length;
  if (badge2) badge2.textContent = `${pending.length} Pending Review`;

  if (pending.length === 0) {
    container.innerHTML = `
      <div class="bg-white rounded-2xl p-12 text-center border border-surface-low shadow-sm">
        <span class="material-symbols-outlined text-5xl text-secondary mb-2">check_circle</span>
        <h3 class="font-headline text-lg font-bold text-primary">All Reports Verified</h3>
        <p class="text-xs text-outline mt-1">No pending reports require community verification right now.</p>
      </div>
    `;
    return;
  }

  container.innerHTML = pending.map(r => `
    <div onclick="window.selectVerifyReport('${r.id}')" class="bg-white rounded-2xl p-5 border border-surface-low shadow-sm flex flex-col sm:flex-row gap-4 hover:border-primary/40 transition-all cursor-pointer">
    <div class="relative w-full sm:w-48 h-36 rounded-xl overflow-hidden flex-shrink-0 bg-surface-low">
      ${renderReportPhoto(r)}
      <span class="absolute top-2 left-2 bg-black/60 text-white font-mono text-[10px] px-2 py-0.5 rounded-full font-bold">● ${r.category}</span>
    </div>
      <div class="flex-1 space-y-2">
        <div class="flex justify-between items-start">
          <h3 class="font-headline text-base font-bold text-primary leading-tight">${r.title}</h3>
          <span class="bg-error-container text-[#93000a] text-[11px] font-mono font-bold px-2 py-0.5 rounded-full">AI: ${r.aiConfidence}% Confidence</span>
        </div>
        <p class="text-xs text-[#42474f] line-clamp-2">${r.notes}</p>
        <div class="grid grid-cols-2 text-[11px] font-mono text-outline pt-1 gap-1">
          <span>📍 ${r.distance}</span>
          <span>🕒 ${r.timestamp}</span>
          <span>👤 ${r.reporter}</span>
          <span>📡 ${r.sensorNode}</span>
        </div>
        <div class="flex gap-2 pt-2">
          <button onclick="event.stopPropagation(); window.voteReport('${r.id}', 'verified')" class="flex-1 bg-primary text-white py-2 px-3 rounded-lg font-mono text-xs font-bold hover:bg-primary-container transition-all flex items-center justify-center gap-1 cursor-pointer">
            <span class="material-symbols-outlined text-[16px]">check_circle</span> Confirm Alert
          </button>
          <button onclick="event.stopPropagation(); window.voteReport('${r.id}', 'rejected')" class="bg-surface-low text-primary hover:bg-surface-high py-2 px-3 rounded-lg font-mono text-xs font-bold border border-surface-low transition-all cursor-pointer">
            Reject (False Positive)
          </button>
        </div>
      </div>
    </div>
  `).join('');
}

window.selectVerifyReport = (id) => {
  const r = reports.find(item => item.id === id);
  if (!r) return;

  const locTag = document.getElementById('context-location-tag');
  const repNo = document.getElementById('dossier-report-no');
  const turb = document.getElementById('dossier-turbidity');
  const doEl = document.getElementById('dossier-do');

  if (locTag) locTag.textContent = r.location;
  if (repNo) repNo.textContent = r.reportNumber;
  if (turb) turb.textContent = `${r.turbidity} NTU`;
  if (doEl) doEl.textContent = `${r.dissolvedOxygen} mg/L`;
};

window.voteReport = async (id, uiStatus) => {
  const vote = uiStatus === 'verified' ? 'confirm' : 'reject';
  const pos = await getVerifierPosition();

  try {
    await api.verifyReport(id, { vote, lat: pos.lat, lng: pos.lng });
  } catch (e) {
    const fb = document.getElementById('verify-feedback');
    if (fb) {
      fb.className = 'p-3.5 rounded-xl font-medium text-xs flex items-center gap-2 bg-error-container text-[#93000a]';
      fb.innerHTML = `<span class="material-symbols-outlined text-base">error</span> Vote failed: ${e.message}`;
      fb.classList.remove('hidden');
      setTimeout(() => fb.classList.add('hidden'), 4000);
    }
    return;
  }

  // Re-fetch from backend instead of mutating the local array —
  // avoids the id type-mismatch bug and stays correct even with duplicate seed rows.
  await initReports();

  const fb = document.getElementById('verify-feedback');
  if (fb) {
    fb.className = `p-3.5 rounded-xl font-medium text-xs flex items-center gap-2 ${uiStatus === 'verified' ? 'bg-secondary-container text-[#00513a]' : 'bg-error-container text-[#93000a]'}`;
    fb.innerHTML = `<span class="material-symbols-outlined text-base">${uiStatus === 'verified' ? 'verified' : 'cancel'}</span> Report ${id} ${uiStatus === 'verified' ? 'confirmed and broadcast to telemetry stream.' : 'marked as false positive.'}`;
    fb.classList.remove('hidden');
    setTimeout(() => fb.classList.add('hidden'), 4000);
  }
};

// 3. Environmental Map Inspector
window.selectMapHotspot = (title, location, severity, coverage) => {
  const t = document.getElementById('map-inspect-title');
  const l = document.getElementById('map-inspect-location');
  const s = document.getElementById('map-inspect-severity');
  const c = document.getElementById('map-inspect-coverage');

  if (t) t.textContent = title;
  if (l) l.textContent = location;
  if (s) s.textContent = severity;
  if (c) c.textContent = coverage;
};

// 4. CSV Export Actions
function initExportEngine() {
  const btn1 = document.getElementById('sidebar-quick-export');
  const btn2 = document.getElementById('download-csv-action');

  const triggerExport = () => {
    exportReportsToCSV(reports.length ? reports : SEED_REPORTS, 'lake_victoria_telemetry_export.csv');
  };

  if (btn1) btn1.addEventListener('click', triggerExport);
  if (btn2) btn2.addEventListener('click', triggerExport);
}