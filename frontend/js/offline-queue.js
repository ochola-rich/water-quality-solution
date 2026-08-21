/**
 * Durable report queue for intermittent connectivity.
 * Photos cannot be included in the JSON sync endpoint yet, so the UI makes that
 * limitation explicit instead of silently discarding a submission.
 */
import { api } from './api.js';

const QUEUE_KEY = 'guardians-offline-report-queue';

function readQueue() {
  try {
    return JSON.parse(localStorage.getItem(QUEUE_KEY) || '[]');
  } catch {
    return [];
  }
}

function writeQueue(queue) {
  localStorage.setItem(QUEUE_KEY, JSON.stringify(queue));
  window.dispatchEvent(new CustomEvent('guardians:queue-changed', { detail: queue.length }));
}

export function queuedReportCount() {
  return readQueue().length;
}

export function queueReport(report) {
  const queue = readQueue();
  const clientUUID = report.client_uuid || crypto.randomUUID();
  queue.push({
    ...report,
    client_uuid: clientUUID,
    queued_at: new Date().toISOString(),
  });
  writeQueue(queue);
  return clientUUID;
}

export async function syncQueuedReports() {
  const queue = readQueue();
  if (!queue.length || !navigator.onLine) return { syncedCount: 0, remaining: queue.length };

  const response = await api.syncReports(queue);
  const completed = new Set((response.reports || []).map((report) => report.client_uuid));
  writeQueue(queue.filter((report) => !completed.has(report.client_uuid)));
  return { syncedCount: response.synced_count || 0, remaining: queuedReportCount(), response };
}

export function installOfflineSync() {
  window.addEventListener('online', () => {
    syncQueuedReports().catch((error) => console.warn('[Offline queue] sync failed:', error.message));
  });
}
