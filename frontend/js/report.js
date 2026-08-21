export function formatCategory(cat) {
  if (!cat) return 'General Anomaly';
  const map = {
    turbidity: 'Elevated Turbidity',
    algae: 'Algae Bloom',
    spill: 'Chemical / Oil Spill',
    smell: 'Odor / Sewage Incident',
    other: 'Water Quality Anomaly',
  };
  return map[cat.toLowerCase()] || cat.charAt(0).toUpperCase() + cat.slice(1);
}

export function formatRelativeTime(dateStr) {
  if (!dateStr) return 'Just now';
  const now = new Date();
  const date = new Date(dateStr);
  const diffSec = Math.floor((now - date) / 1000);

  if (isNaN(diffSec) || diffSec < 60) return 'Just now';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHour = Math.floor(diffMin / 60);
  if (diffHour < 24) return `${diffHour}h ago`;
  const diffDay = Math.floor(diffHour / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

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