/**
 * CSV & GeoJSON Export Engine
 */
export class ReportExporter {
  /**
   * Compiles verified water quality telemetry into a downloadable CSV
   */
  static exportToCSV(reports, filename = 'lake_victoria_reports.csv') {
    const headers = [
      'Report_ID',
      'Title',
      'Location',
      'Category',
      'Status',
      'Turbidity_NTU',
      'DO_mg_L',
      'pH',
      'Water_Temp_C',
      'Timestamp',
      'Reporter_Name'
    ];

    const rows = reports.map((r) => [
      r.reportNumber,
      `"${(r.title || '').replace(/"/g, '""')}"`,
      `"${(r.location || '').replace(/"/g, '""')}"`,
      r.category || 'General',
      r.status || 'unverified',
      r.turbidity,
      r.dissolvedOxygen,
      r.ph,
      r.temp,
      r.timestamp,
      `"${(r.reporter || '').replace(/"/g, '""')}"`
    ]);

    const csvContent = [headers.join(','), ...rows.map((row) => row.join(','))].join('\n');
    this.triggerDownload(csvContent, 'text/csv', filename);
  }

  /**
   * Exports Hotspot coordinates as a standardized GIS GeoJSON file
   */
  static exportHotspotsGeoJSON(hotspots, filename = 'kisumu_hotspots.geojson') {
    const geojson = {
      type: 'FeatureCollection',
      name: 'Lake Victoria Hotspots',
      crs: { type: 'name', properties: { name: 'urn:ogc:def:crs:OGC:1.3:CRS84' } },
      features: hotspots.map((hs) => ({
        type: 'Feature',
        geometry: {
          type: 'Point',
          coordinates: [hs.coordinates[1], hs.coordinates[0]] // [Lng, Lat]
        },
        properties: {
          id: hs.id,
          name: hs.name,
          location: hs.location,
          severity: hs.severity,
          type: hs.type,
          ...hs.metrics
        }
      }))
    };

    const jsonString = JSON.stringify(geojson, null, 2);
    this.triggerDownload(jsonString, 'application/geo+json', filename);
  }

  static triggerDownload(content, mimeType, filename) {
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }
}