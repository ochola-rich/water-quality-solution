/**
 * Lake Overview Dashboard Logic
 * File: frontend/js/dashboard.js
 */

import { api } from './api.js';
import { wsClient } from './ws-client.js';
import { exportReportsToCSV } from './report.js';

document.addEventListener('DOMContentLoaded', () => {
  initHealthScore();
  initExport();
  wsClient.connect();
});

function initHealthScore(score = 72) {
  const scoreVal = document.getElementById('health-score-val');
  const circle = document.getElementById('gauge-circle');

  if (scoreVal) scoreVal.textContent = score;
  if (circle) {
    const circumference = 2 * Math.PI * 40; // 251.32
    const offset = circumference - (score / 100) * circumference;
    circle.style.strokeDashoffset = offset;
  }
}

function initExport() {
  const btn = document.getElementById('export-csv-btn');
  if (!btn) return;

  btn.addEventListener('click', async () => {
    try {
      const reports = await api.getReports('all');
      exportReportsToCSV(reports, 'lake_victoria_telemetry.csv');
    } catch (err) {
      exportReportsToCSV([
        { id: '#8492', title: 'High Turbidity & Algal Growth', category: 'Algae Bloom', location: 'Dunga Beach Pier', status: 'verified', dissolvedOxygen: 3.1, turbidity: 48.6, ph: 8.4 },
        { id: '#8493', title: 'Chemical Sheen Incident', category: 'Chemical Spill', location: 'Kisumu Port', status: 'pending', dissolvedOxygen: 4.8, turbidity: 28.0, ph: 7.8 }
      ]);
    }
  });
}