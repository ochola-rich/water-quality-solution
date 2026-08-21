/**
 * Guardians of the Lake - Peer Verification Feed Controller
 * File: frontend/js/verify.js
 */

import { api } from './api.js';
import { formatCategory, formatRelativeTime } from './report.js';
import { wsClient } from './ws-client.js';

let reports = [];
let searchFilter = '';
let verifierPosition = null;

async function getVerifierPosition() {
  if (verifierPosition) return verifierPosition;
  return new Promise((resolve) => {
    if (!navigator.geolocation) {
      verifierPosition = { lat: -0.1022, lng: 34.7617 }; // Kisumu baseline
      resolve(verifierPosition);
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        verifierPosition = { lat: pos.coords.latitude, lng: pos.coords.longitude };
        resolve(verifierPosition);
      },
      () => {
        verifierPosition = { lat: -0.1022, lng: 34.7617 };
        resolve(verifierPosition);
      },
      { timeout: 3000 }
    );
  });
}

async function init() {
  initSearch();
  initWebSocket();
  await loadReports();
}

function initSearch() {
  const input = document.getElementById('search-input');
  if (input) {
    input.addEventListener('input', (e) => {
      searchFilter = e.target.value.toLowerCase().trim();
      renderFeed();
    });
  }
}

async function loadReports() {
  renderLoadingSkeleton();

  try {
    const data = await api.getReports('pending');
    reports = Array.isArray(data) ? data : [];
    renderFeed();
  } catch (err) {
    console.warn('[Verify] Failed to load pending reports:', err.message);
    renderErrorState(err.message);
  }
}

function renderLoadingSkeleton() {
  const container = document.getElementById('verify-feed');
  if (!container) return;

  container.innerHTML = `
    <div class="space-y-4">
      <div class="bg-white rounded-2xl p-5 border border-[#eff4ff] shadow-sm animate-pulse flex flex-col sm:flex-row gap-4">
        <div class="w-full sm:w-48 h-36 bg-gray-100 rounded-xl"></div>
        <div class="flex-1 space-y-3 py-1">
          <div class="h-4 bg-gray-100 rounded w-3/4"></div>
          <div class="h-3 bg-gray-100 rounded w-full"></div>
          <div class="h-3 bg-gray-100 rounded w-5/6"></div>
          <div class="h-8 bg-gray-100 rounded w-1/2 mt-4"></div>
        </div>
      </div>
    </div>
  `;
}

function renderErrorState(errMsg) {
  const container = document.getElementById('verify-feed');
  if (!container) return;

  container.innerHTML = `
    <div class="bg-white p-8 rounded-2xl border border-red-100 text-center shadow-sm">
      <span class="material-symbols-outlined text-4xl text-[#ba1a1a]">error</span>
      <h3 class="font-bold text-base text-[#002546] mt-2">Failed to load verification feed</h3>
      <p class="text-xs text-gray-500 mt-1">${escapeHtml(errMsg || 'Could not connect to backend server')}</p>
      <button onclick="window.reloadVerifyFeed()" class="mt-4 px-4 py-2 bg-[#002546] text-white font-mono text-xs rounded-xl hover:bg-[#0d3b66] transition-colors cursor-pointer">
        Retry Connection
      </button>
    </div>
  `;
}

window.reloadVerifyFeed = () => {
  loadReports();
};

function renderFeed() {
  const container = document.getElementById('verify-feed');
  const counter = document.getElementById('pending-counter');
  if (!container) return;

  let filtered = reports.filter(r => r.status === 'pending' || r.status === 'unverified');
  if (searchFilter) {
    filtered = filtered.filter(r => 
      (r.description && r.description.toLowerCase().includes(searchFilter)) ||
      (r.notes && r.notes.toLowerCase().includes(searchFilter)) ||
      (r.category && r.category.toLowerCase().includes(searchFilter)) ||
      (String(r.id).includes(searchFilter))
    );
  }

  if (counter) counter.textContent = `${filtered.length} Pending`;

  if (!filtered.length) {
    container.innerHTML = `
      <div class="bg-white p-12 rounded-2xl border border-gray-100 text-center shadow-sm">
        <span class="material-symbols-outlined text-4xl text-[#006c4e]">check_circle</span>
        <h3 class="font-bold text-lg text-[#002546] mt-2">All Pending Reports Verified</h3>
        <p class="text-xs text-gray-500 mt-1">Great job! The environmental stream is fully up to date with community consensus.</p>
      </div>
    `;
    return;
  }

  container.innerHTML = filtered.map(r => {
    const rawPhoto = r.photo_path || r.imageUrl;
    const photoUrl = rawPhoto 
      ? (rawPhoto.startsWith('http') ? rawPhoto : `${api.getBaseUrl()}${rawPhoto}`)
      : 'https://images.unsplash.com/photo-1576086213369-97a306d36557?auto=format&fit=crop&w=600&q=80';
    
    const confidence = r.ai_prediction ? extractConfidence(r.ai_prediction) : (r.aiConfidence || 93);
    const distanceStr = r.distance_m ? `${Math.round(r.distance_m)}m away` : (r.distance || 'Kisumu Sector');
    const description = r.description || r.notes || 'Citizen report pending community verification.';
    const timeStr = r.created_at ? formatRelativeTime(r.created_at) : (r.timestamp || 'Recently');
    const author = r.user_id ? `Guardian #${r.user_id}` : (r.reporter || 'Field Observer');

    return `
      <div id="card-${r.id}" class="bg-white rounded-2xl p-5 border border-[#eff4ff] shadow-sm flex flex-col sm:flex-row gap-4 hover:border-[#002546]/30 transition-all">
        <div class="relative w-full sm:w-48 h-36 rounded-xl overflow-hidden flex-shrink-0 bg-gray-100">
          <img src="${photoUrl}" alt="${r.category}" class="w-full h-full object-cover" onerror="this.src='https://images.unsplash.com/photo-1544551763-46a013bb70d5?auto=format&fit=crop&w=600&q=80'" />
          <span class="absolute top-2 left-2 bg-black/60 text-white font-mono text-[10px] px-2 py-0.5 rounded-full font-bold">
            ● ${formatCategory(r.category)}
          </span>
        </div>

        <div class="flex-1 space-y-2">
          <div class="flex justify-between items-start">
            <h3 class="font-bold text-base text-[#002546] leading-tight">
              ${formatCategory(r.category)} Report #${r.id}
            </h3>
            <span class="bg-[#ffdad6] text-[#93000a] text-[11px] font-mono font-bold px-2 py-0.5 rounded-full">
              AI: ${confidence}% Confidence
            </span>
          </div>

          <p class="text-xs text-gray-600 line-clamp-2">${escapeHtml(description)}</p>

          <div class="grid grid-cols-2 text-[11px] font-mono text-gray-500 pt-1 gap-1">
            <span>📍 ${distanceStr}</span>
            <span>🕒 ${timeStr}</span>
            <span>👤 ${author}</span>
            <span>📡 GPS: ${r.lat != null ? Number(r.lat).toFixed(4) : '-0.1022'}, ${r.lng != null ? Number(r.lng).toFixed(4) : '34.7617'}</span>
          </div>

          <div class="flex gap-2 pt-2">
            <button id="vote-btn-confirm-${r.id}" onclick="window.confirmVote(${r.id})" class="flex-1 bg-[#002546] text-white py-2 px-3 rounded-lg font-mono text-xs font-bold hover:bg-[#0d3b66] transition-all flex items-center justify-center gap-1 cursor-pointer">
              <span class="material-symbols-outlined text-sm">check_circle</span> Confirm Alert
            </button>
            <button id="vote-btn-reject-${r.id}" onclick="window.rejectVote(${r.id})" class="bg-[#eff4ff] text-[#002546] hover:bg-[#dce9ff] py-2 px-3 rounded-lg font-mono text-xs font-bold border border-[#dce9ff] transition-all cursor-pointer">
              Reject (False Positive)
            </button>
          </div>
        </div>
      </div>
    `;
  }).join('');
}

window.confirmVote = async (id) => {
  await submitVote(id, 'confirm');
};

window.rejectVote = async (id) => {
  await submitVote(id, 'reject');
};

async function submitVote(id, voteType) {
  const r = reports.find(item => item.id === id);
  const btn = document.getElementById(`vote-btn-${voteType}-${id}`);
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="material-symbols-outlined text-sm animate-spin">sync</span> Submitting...`;
  }

  try {
    const pos = await getVerifierPosition();
    const lat = r && r.lat != null ? r.lat : pos.lat;
    const lng = r && r.lng != null ? r.lng : pos.lng;

    const res = await api.verifyReport(id, {
      verifier_id: 2, // Demo verifier ID
      vote: voteType,
      lat: lat,
      lng: lng,
    });

    reports = reports.filter(item => item.id !== id);
    renderFeed();

    const isConsensus = res.result?.new_status === 'verified';
    showFeedback(
      `Vote '${voteType}' recorded for Report #${id}. ${isConsensus ? 'Consensus reached! Report anchored in cryptographic ledger.' : 'Awaiting additional peer verification.'}`,
      'success'
    );
  } catch (err) {
    showFeedback(err.message, 'error');
  } finally {
    if (btn) btn.disabled = false;
  }
}

function showFeedback(msg, type) {
  const banner = document.getElementById('feedback-banner');
  if (!banner) return;
  banner.className = `p-3 rounded-xl font-medium text-xs flex items-center gap-2 ${type === 'success' ? 'bg-[#99f5cd] text-[#00513a]' : 'bg-[#ffdad6] text-[#93000a]'}`;
  banner.innerHTML = `<span class="material-symbols-outlined text-sm">${type === 'success' ? 'verified' : 'cancel'}</span> ${escapeHtml(msg)}`;
  banner.classList.remove('hidden');
  setTimeout(() => banner.classList.add('hidden'), 5000);
}

function initWebSocket() {
  wsClient.connect();

  wsClient.on('report:new', (report) => {
    reports.unshift(report);
    renderFeed();
    showFeedback(`New water quality report #${report.id} received from field!`, 'success');
  });

  wsClient.on('report:verified', (data) => {
    reports = reports.filter(r => r.id !== data.report_id);
    renderFeed();
  });
}

function extractConfidence(aiJson) {
  try {
    const data = typeof aiJson === 'string' ? JSON.parse(aiJson) : aiJson;
    if (data.confidence_score) return Math.round(data.confidence_score * 100);
  } catch (e) {}
  return 93;
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>"']/g, (m) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[m]));
}

document.addEventListener('DOMContentLoaded', init);