/**
 * Guardians of the Lake - Unified API Helper
 * File: frontend/js/api.js
 */

const BASE_URL = window.location.origin || 'http://localhost:8080';

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
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.message || `Request failed with status ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.warn(`[API] Network request to ${endpoint} failed:`, error.message);
    throw error;
  }
}

export const api = {
  // GET /api/reports?status=pending
  async getReports(status = 'pending') {
    const query = status && status !== 'all' ? `?status=${status}` : '';
    return apiFetch(`/api/reports${query}`, { method: 'GET' });
  },

  // POST /api/reports/:id/verify
  async verifyReport(id, status = 'verified', notes = '') {
    return apiFetch(`/api/reports/${id}/verify`, {
      method: 'POST',
      body: { status, notes, verifiedAt: new Date().toISOString() },
    });
  },

  // GET /api/health
  async getHealthScore() {
    return apiFetch('/api/health', { method: 'GET' });
  },

  // GET /api/hotspots
  async getHotspots() {
    return apiFetch('/api/hotspots', { method: 'GET' });
  },

  // GET /api/alerts
  async getAlerts() {
    return apiFetch('/api/alerts', { method: 'GET' });
  }
};