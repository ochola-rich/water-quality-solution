/**
 * Guardians of the Lake - Unified API Helper
 * File: frontend/js/api.js
 *
 * Endpoint contract verified directly against backend handlers
 * (handlers/verify.go, handlers/dashboard.go) — not guessed.
 */

// Intelligent origin detection: If opened via standard static dev server (port 5500, 8080, 5173, etc.) or file://,
// target the Go backend on port 3000. If served directly by Go Fiber or deployed, use window.location.origin.
const isDevFrontendServer = typeof window !== 'undefined' && (
  !window.location.origin ||
  window.location.origin === 'null' ||
  !window.location.origin.startsWith('http') ||
  window.location.port === '5500' ||
  window.location.port === '8080' ||
  window.location.port === '5173' ||
  window.location.port === '8000'
);

const BASE_URL = isDevFrontendServer 
  ? `${window.location.protocol === 'https:' ? 'https:' : 'http:'}//${window.location.hostname || 'localhost'}:3000` 
  : (window.location.origin || 'http://localhost:3000');

export class ApiError extends Error {
  constructor(message, status = 500, data = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

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
      throw new ApiError(errorData.error || `Request failed with status ${response.status}`, response.status, errorData);
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

  // Alias for consumers that need the complete alert history.
  async getAlerts() {
    return this.getAllAlerts();
  },

  async syncReports(reports) {
    return apiFetch('/api/reports/sync', { method: 'POST', body: { reports } });
  },

  async getHealth() {
    return apiFetch('/api/dashboard/health', { method: 'GET' });
  },

  // Alias for getHealth
  async getLakeHealth() {
    return this.getHealth();
  },

  // Alias for getMapPoints
  async getDashboardPoints(options = {}) {
    return this.getMapPoints(options);
  },

  // Alias for getTrends
  async getDashboardTrends(days = 7) {
    return this.getTrends(days);
  },

  // GET /api/users/leaderboard
  async getLeaderboard() {
    return apiFetch('/api/users/leaderboard', { method: 'GET' });
  },

  // GET /api/users/:id
  async getUserProfile(id = 1) {
    return apiFetch(`/api/users/${id}`, { method: 'GET' });
  },

  // GET /api/rewards/summary
  async getRewardStats() {
    return apiFetch('/api/rewards/summary', { method: 'GET' });
  },

  // GET /api/users/:id/rewards
  async getUserRewards(id = 1) {
    return apiFetch(`/api/users/${id}/rewards`, { method: 'GET' });
  },

  getBaseUrl() {
    return BASE_URL;
  },

  async downloadExportCSV(status = 'verified', filename = 'lake_victoria_reports.csv') {
    const url = `${BASE_URL}/api/dashboard/export?status=${encodeURIComponent(status)}`;
    const response = await fetch(url, {
      headers: { 'Accept': 'text/csv' },
    });
    if (!response.ok) {
      throw new ApiError(`Export failed with status ${response.status}`, response.status);
    }
    const blob = await response.blob();
    const blobUrl = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    link.setAttribute('download', filename);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(blobUrl);
    return true;
  },
};
