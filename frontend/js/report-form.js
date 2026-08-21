/**
 * Citizen report submission — embedded as a dashboard tab
 * File: frontend/js/report-form.js
 */

import { api } from './api.js';
import { classifyWaterPhoto } from './ai-classifier.js';
import { installOfflineSync, queueReport, syncQueuedReports } from './offline-queue.js';

let currentPosition = null;
let photoAssessment = null;

document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('report-form');
  if (!form) return; // section not present on this page, skip silently

  acquireLocation();
  installOfflineSync();
  syncQueuedReports().catch(() => {});
  form.addEventListener('submit', handleSubmit);

  document.getElementById('report-photo').addEventListener('change', assessPhoto);

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

async function assessPhoto(event) {
  const status = document.getElementById('photo-ai-status');
  const file = event.target.files[0];
  photoAssessment = null;
  if (!file) {
    status.classList.add('hidden');
    return;
  }
  status.className = 'mt-2 text-xs rounded-lg p-2 bg-surface-low text-outline';
  status.textContent = 'Analysing visual cues with MobileNet…';
  try {
    photoAssessment = await classifyWaterPhoto(file);
    const labels = photoAssessment.predictions.map((item) => `${item.label} (${Math.round(item.confidence * 100)}%)`).join(', ');
    status.textContent = `Visual cues (advisory only): ${labels}`;
  } catch {
    status.textContent = 'Photo saved for peer review. Visual classifier is unavailable offline.';
  }
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
  formData.set('client_uuid', crypto.randomUUID());
  if (photoAssessment) formData.set('ai_prediction', JSON.stringify(photoAssessment));

  try {
    const report = await api.submitReport(formData);
    handleSubmitSuccess(report);
    form.reset();
    document.getElementById('report-lat').value = currentPosition.lat;
    document.getElementById('report-lng').value = currentPosition.lng;
  } catch (err) {
    if (navigator.onLine) {
      showFeedback(`Submission failed: ${err.message}`, 'error');
      return;
    }
    const queuedID = queueReport({
      client_uuid: formData.get('client_uuid'),
      lat: currentPosition.lat,
      lng: currentPosition.lng,
      category: formData.get('category'),
      description: formData.get('description'),
      ai_prediction: formData.get('ai_prediction') || '',
      device_meta: '',
    });
    const photoNote = formData.get('photo')?.size ? ' The photo must be reattached when online.' : '';
    showFeedback(`You are offline. Report ${queuedID.slice(0, 8)} was saved and will sync automatically.${photoNote}`, 'warning');
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
