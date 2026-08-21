/**
 * Guardians of the Lake - Unified API Helper
 * File: frontend/js/api.js
 *
 * Endpoint contract verified directly against backend handlers
 * (handlers/verify.go, handlers/dashboard.go) — not guessed.
 */

const BASE_URL = window.location.origin || 'http://localhost:3000';

async function apiFetch(endpoint, options = {}) {
  const url = endpoint.startsWith('http') ? endpoint : `${BASE_URL}${endpoint}`;

  const defaultHeaders = {
    'Accept': 'application/json',
  };

  if (!(options.body instanceof FormData)) {
    defaultHeaders['Content-Type'] = 'application/json';
  }

  const config = {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
  };

  if (options.body && !(options.body instanceof FormData) && typeof options.body === 'object') {
    config.body = JSON.stringify(options.body);
  }

  try {
    const response = await fetch(url, config);
    if (!response.ok) {
      // Backend returns errors as { "error": "..." }, not { "message": "..." }
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.error || `Request failed with status ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.warn(`[API] Network request to ${endpoint} failed:`, error.message);
    throw error;
  }
}

export const api = {
  // POST /api/ai/assess — { category, description, lat, lng } → water quality assessment
  // confidence_score is 0.0–1.0, NOT a percentage — multiply by 100 to display
  async assessReport({ category, description, lat, lng }) {
    return apiFetch('/api/ai/assess', {
      method: 'POST',
      body: { category, description, lat, lng },
    });
  },
  // POST /api/reports — multipart form (photo + lat/lng + category + description)
  async submitReport(formData) {
    return apiFetch('/api/reports', {
      method: 'POST',
      body: formData, // FormData — do NOT set Content-Type manually
    });
  },

  // GET /api/reports?status=pending
  async getReports(status = 'pending') {
    const query = status && status !== 'all' ? `?status=${status}` : '';
    return apiFetch(`/api/reports${query}`, { method: 'GET' });
  },

  // GET /api/reports/:id
  async getReport(id) {
    return apiFetch(`/api/reports/${id}`, { method: 'GET' });
  },

  // POST /api/reports/:id/verify
  // Backend REQUIRES lat/lng — request is rejected with 400 if both are 0.
  // verifierId defaults server-side to 2 (demo) if omitted, but pass it if you have it.
  async verifyReport(id, { vote, lat, lng, verifierId } = {}) {
    if (vote !== 'confirm' && vote !== 'reject') {
      throw new Error(`vote must be 'confirm' or 'reject', got '${vote}'`);
    }
    if (!lat || !lng) {
      throw new Error('lat and lng are required to submit a verification vote');
    }
    return apiFetch(`/api/reports/${id}/verify`, {
      method: 'POST',
      body: { vote, lat, lng, verifier_id: verifierId },
    });
  },

  // GET /api/reports/:id/verifications
  async getVerifications(id) {
    return apiFetch(`/api/reports/${id}/verifications`, { method: 'GET' });
  },

  // GET /api/dashboard/summary
  async getDashboardSummary() {
    return apiFetch('/api/dashboard/summary', { method: 'GET' });
  },

  // GET /api/dashboard/points?status=&category= — GeoJSON FeatureCollection for Leaflet
  async getMapPoints({ status = 'all', category = 'all' } = {}) {
    return apiFetch(`/api/dashboard/points?status=${status}&category=${category}`, { method: 'GET' });
  },

  // GET /api/dashboard/trends?days=7 — array of { date, category, count }
  // NOTE: there is no backend health-score endpoint. If you want a 0-100
  // "health score," it has to be computed client-side from this trend data
  // (e.g. weighting spill/algae/turbidity counts) — not fetched directly.
  async getTrends(days = 7) {
    return apiFetch(`/api/dashboard/trends?days=${days}`, { method: 'GET' });
  },

  // GET /api/alerts/active
  async getActiveAlerts() {
    return apiFetch('/api/alerts/active', { method: 'GET' });
  },

  // GET /api/alerts
  async getAllAlerts() {
    return apiFetch('/api/alerts', { method: 'GET' });
  },
};