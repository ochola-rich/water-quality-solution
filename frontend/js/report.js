/**
 * CSV & Telemetry Report Export Utility
 * File: frontend/js/report.js
 */

export function exportReportsToCSV(reports, filename = 'lake_victoria_reports.csv') {
  if (!reports || !reports.length) {
    alert('No records available to export.');
    return;
  }

  const headers = ['Report_ID', 'Title', 'Category', 'Location', 'Status', 'DO_mgL', 'Turbidity_NTU', 'pH', 'Timestamp', 'Reporter'];
  
  const rows = reports.map(r => [
    r.reportNumber || r.id,
    `"${(r.title || '').replace(/"/g, '""')}"`,
    r.category || 'General',
    `"${(r.location || '').replace(/"/g, '""')}"`,
    r.status || 'unverified',
    r.dissolvedOxygen || '6.2',
    r.turbidity || '24.0',
    r.ph || '7.6',
    r.timestamp || new Date().toLocaleDateString(),
    `"${(r.reporter || 'Community Observer').replace(/"/g, '""')}"`
  ]);

  const csvContent = [headers.join(','), ...rows.map(e => e.join(','))].join('\n');
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.setAttribute('download', filename);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}