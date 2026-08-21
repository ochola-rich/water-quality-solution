/**
 * Citizen report submission — embedded as a dashboard tab
 * File: frontend/js/report-form.js
 */

import { api } from './api.js';

let currentPosition = null;

document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('report-form');
  if (!form) return; // section not present on this page, skip silently

  acquireLocation();
  form.addEventListener('submit', handleSubmit);

  document.getElementById('manual-entry-toggle').addEventListener('click', () => {
    document.getElementById('manual-entry-section').classList.toggle('hidden');
  });

  document.getElementById('manual-entry-confirm').addEventListener('click', () => {
    const lat = parseFloat(document.getElementById('manual-lat').value);
    const lng = parseFloat(document.getElementById('manual-lng').value);
    if (isNaN(lat) || isNaN(lng)) {
      setLocationStatus('Enter valid numeric latitude and longitude.', 'error');
      return;
    }
    setPosition(lat, lng, 'Location set manually');
    document.getElementById('manual-entry-section').classList.add('hidden');
  });
});

function acquireLocation() {
  if (!navigator.geolocation) {
    setLocationStatus('Geolocation not supported. Enter your location manually below.', 'error');
    return;
  }

  navigator.geolocation.getCurrentPosition(
    (pos) => {
      setPosition(pos.coords.latitude, pos.coords.longitude, `Location found (±${Math.round(pos.coords.accuracy)}m accuracy)`);
    },
    () => {
      // Denied or blocked — don't leave the user stuck, point them at the manual fallback
      setLocationStatus('Location access unavailable. Enter your location manually below.', 'error');
    }
  );
}

function setPosition(lat, lng, message) {
  currentPosition = { lat, lng };
  document.getElementById('report-lat').value = lat;
  document.getElementById('report-lng').value = lng;
  setLocationStatus(message, 'success');
}

function setLocationStatus(message, type) {
  const statusEl = document.getElementById('location-status');
  const icon = type === 'success' ? 'check_circle' : 'error';
  const iconColor = type === 'success' ? 'text-secondary' : 'text-error';
  statusEl.innerHTML = `<span class="material-symbols-outlined text-base ${iconColor}">${icon}</span><span>${message}</span>`;
  statusEl.className = `text-sm rounded-xl p-3 flex items-center gap-2 ${type === 'success' ? 'bg-surface-low' : 'bg-error-container text-[#93000a]'}`;
}

async function handleSubmit(e) {
  e.preventDefault();

  if (!currentPosition) {
    setLocationStatus('No location set. Use automatic detection or enter manually above.', 'error');
    return;
  }

  const submitBtn = document.getElementById('submit-btn');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Submitting...';

  const form = document.getElementById('report-form');
  const formData = new FormData(form);
  formData.set('lat', currentPosition.lat);
  formData.set('lng', currentPosition.lng);

  try {
    const report = await api.submitReport(formData);
    handleSubmitSuccess(report);
    form.reset();
    document.getElementById('report-lat').value = currentPosition.lat;
    document.getElementById('report-lng').value = currentPosition.lng;
  } catch (err) {
    showFeedback(`Submission failed: ${err.message}`, 'error');
  } finally {
    submitBtn.disabled = false;
    submitBtn.innerHTML = `<span class="material-symbols-outlined text-base">send</span> Submit Report`;
  }
}

async function handleSubmitSuccess(report) {
  if (typeof window.refreshReports === 'function') {
    window.refreshReports();
  }

  if (report.status === 'flagged') {
    showFeedback(`Report #${report.id} submitted, but flagged for review (unusual submission pattern detected). A verifier will check it manually.`, 'warning');
    return;
  }

  showFeedback(`Report #${report.id} submitted successfully! It's now pending peer verification.`, 'success');

  // Run AI assessment and append its advisory as a follow-up — non-blocking,
  // submission already succeeded regardless of whether this call works.
  try {
    const assessment = await api.assessReport({
      category: report.category,
      description: report.description,
      lat: report.lat,
      lng: report.lng,
    });
    appendAssessment(assessment);
  } catch (err) {
    console.warn('[AI] Assessment failed (non-critical):', err.message);
  }
}

function appendAssessment(assessment) {
  const fb = document.getElementById('submit-feedback');
  const confidencePct = Math.round(assessment.confidence_score * 100);
  const severityColors = {
    critical: 'bg-error-container text-[#93000a]',
    warning: 'bg-[#fff3cd] text-[#7a5b00]',
    normal: 'bg-secondary-container text-[#00513a]',
  };
  const badgeClass = severityColors[assessment.severity] || severityColors.normal;

  const assessmentEl = document.createElement('div');
  assessmentEl.className = `mt-3 p-3 rounded-xl text-xs space-y-1 ${badgeClass}`;
  assessmentEl.innerHTML = `
    <div class="flex justify-between items-center font-mono font-bold">
      <span>AI ASSESSMENT — ${assessment.severity.toUpperCase()}</span>
      <span>${confidencePct}% confidence</span>
    </div>
    <p>${assessment.advisory_notice}</p>
    <p class="opacity-75">Water Quality Index: ${assessment.water_quality_index}/100</p>
  `;
  fb.after(assessmentEl);
}

function showFeedback(message, type) {
  const fb = document.getElementById('submit-feedback');
  const styles = {
    success: 'bg-secondary-container text-[#00513a]',
    warning: 'bg-[#fff3cd] text-[#7a5b00]',
    error: 'bg-error-container text-[#93000a]',
  };
  fb.className = `p-4 rounded-xl text-sm ${styles[type]}`;
  fb.textContent = message;
  fb.classList.remove('hidden');
}