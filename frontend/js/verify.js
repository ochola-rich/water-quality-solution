/**
 * Verification Screen Logic
 * File: frontend/js/verify.js
 */

import { api } from './api.js';

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
    status: 'pending'
  }
];

let reports = [];

async function init() {
  try {
    const data = await api.getReports('pending');
    reports = data && data.length ? data : SEED_REPORTS;
  } catch (err) {
    reports = SEED_REPORTS;
  }
  renderFeed();
}

function renderFeed() {
  const container = document.getElementById('verify-feed');
  const counter = document.getElementById('pending-counter');
  if (!container) return;

  const pendingList = reports.filter(r => r.status === 'pending' || r.status === 'unverified');
  if (counter) counter.textContent = `${pendingList.length} Pending`;

  if (pendingList.length === 0) {
    container.innerHTML = `
      <div class="bg-white p-12 rounded-2xl border border-gray-100 text-center shadow-sm">
        <span class="material-symbols-outlined text-4xl text-[#006c4e]">check_circle</span>
        <h3 class="font-bold text-lg text-[#002546] mt-2">All Pending Reports Verified</h3>
        <p class="text-xs text-gray-500 mt-1">Great job! The environmental stream is fully up to date.</p>
      </div>
    `;
    return;
  }

  container.innerHTML = pendingList.map(r => `
    <div id="card-${r.id}" class="bg-white rounded-2xl p-5 border border-[#eff4ff] shadow-sm flex flex-col sm:flex-row gap-4">
      <div class="relative w-full sm:w-48 h-36 rounded-xl overflow-hidden flex-shrink-0 bg-gray-100">
        <img src="${r.imageUrl}" alt="${r.title}" class="w-full h-full object-cover" />
        <span class="absolute top-2 left-2 bg-black/60 text-white font-mono text-[10px] px-2 py-0.5 rounded-full font-bold">
          ● ${r.category}
        </span>
      </div>

      <div class="flex-1 space-y-2">
        <div class="flex justify-between items-start">
          <h3 class="font-bold text-base text-[#002546] leading-tight">${r.title}</h3>
          <span class="bg-[#ffdad6] text-[#93000a] text-[11px] font-mono font-bold px-2 py-0.5 rounded-full">
            AI: ${r.aiConfidence}% Confidence
          </span>
        </div>

        <p class="text-xs text-gray-600 line-clamp-2">${r.notes}</p>

        <div class="grid grid-cols-2 text-[11px] font-mono text-gray-500 pt-1 gap-1">
          <span>📍 ${r.distance}</span>
          <span>🕒 ${r.timestamp}</span>
          <span>👤 ${r.reporter}</span>
          <span>📡 ${r.sensorNode}</span>
        </div>

        <div class="flex gap-2 pt-2">
          <button onclick="window.confirmVote('${r.id}')" class="flex-1 bg-[#002546] text-white py-2 px-3 rounded-lg font-mono text-xs font-bold hover:bg-[#0d3b66] transition-all flex items-center justify-center gap-1 cursor-pointer">
            <span class="material-symbols-outlined text-sm">check_circle</span> Confirm Alert
          </button>
          <button onclick="window.rejectVote('${r.id}')" class="bg-[#eff4ff] text-[#002546] hover:bg-[#dce9ff] py-2 px-3 rounded-lg font-mono text-xs font-bold border border-[#dce9ff] transition-all cursor-pointer">
            Reject (False Positive)
          </button>
        </div>
      </div>
    </div>
  `).join('');
}

window.confirmVote = async (id) => {
  try {
    await api.verifyReport(id, 'verified');
  } catch (e) {}

  reports = reports.map(r => r.id === id ? { ...r, status: 'verified' } : r);
  showFeedback(`Report ${id} successfully confirmed and published to the live lake map!`, 'success');
  renderFeed();
};

window.rejectVote = async (id) => {
  try {
    await api.verifyReport(id, 'rejected');
  } catch (e) {}

  reports = reports.map(r => r.id === id ? { ...r, status: 'rejected' } : r);
  showFeedback(`Report ${id} marked as false positive.`, 'error');
  renderFeed();
};

function showFeedback(msg, type) {
  const banner = document.getElementById('feedback-banner');
  if (!banner) return;
  banner.className = `p-3 rounded-xl font-medium text-xs flex items-center gap-2 ${type === 'success' ? 'bg-[#99f5cd] text-[#00513a]' : 'bg-[#ffdad6] text-[#93000a]'}`;
  banner.innerHTML = `<span class="material-symbols-outlined text-sm">${type === 'success' ? 'verified' : 'cancel'}</span> ${msg}`;
  banner.classList.remove('hidden');
  setTimeout(() => banner.classList.add('hidden'), 4000);
}

document.addEventListener('DOMContentLoaded', init);