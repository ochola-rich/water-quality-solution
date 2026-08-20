/**
 * Lake Overview Dashboard Controller
 */
import { wsClient } from './ws-client.js';
import { LimnologyClassifier } from './ai-classifier.js';
import { ReportExporter } from './report.js';

document.addEventListener('DOMContentLoaded', () => {
  // 1. Initial State
  const defaultReadings = {
    dissolvedOxygen: 6.4,
    turbidity: 24.8,
    ph: 7.6,
    chlorophyllA: 18.5,
    waterTemp: 26.2
  };

  const initialHotspots = [
    { id: 'hs-1', name: 'Algae Bloom', color: '#ba1a1a', severity: 'critical', x: 40, y: 30, coordinates: [-0.1022, 34.7523] },
    { id: 'hs-2', name: 'Turbidity Plume', color: '#5c2d00', severity: 'medium', x: 25, y: 45, coordinates: [-0.2185, 34.6150] },
    { id: 'hs-3', name: 'Safe Inlet', color: '#006c4e', severity: 'low', x: 65, y: 60, coordinates: [-0.3421, 34.8210] }
  ];

  // 2. Render Gauge & Health Score
  function updateHealthGauge(readings) {
    const assessment = LimnologyClassifier.calculateHealthScore(readings);
    const scoreVal = document.getElementById('score-value');
    const statusVal = document.getElementById('score-status');
    const progressCircle = document.getElementById('gauge-progress');

    if (scoreVal) scoreVal.textContent = assessment.score;
    if (statusVal) statusVal.textContent = assessment.status;

    if (progressCircle) {
      const radius = 40;
      const circumference = 2 * Math.PI * radius;
      const offset = circumference - (assessment.score / 100) * circumference;
      progressCircle.style.strokeDasharray = `${circumference}`;
      progressCircle.style.strokeDashoffset = `${offset}`;
    }
  }

  updateHealthGauge(defaultReadings);

  // 3. Connect Live WebSocket Telemetry
  wsClient.connect();
  wsClient.subscribe((event) => {
    if (event.type === 'TELEMETRY_PULSE' && event.metrics) {
      updateHealthGauge({
        ...defaultReadings,
        dissolvedOxygen: parseFloat(event.metrics.dissolvedOxygen),
        turbidity: parseFloat(event.metrics.turbidity)
      });
    }
  });

  // 4. Attach CSV Export Listener
  const exportBtn = document.getElementById('export-data-btn');
  if (exportBtn) {
    exportBtn.addEventListener('click', () => {
      const storedReports = JSON.parse(localStorage.getItem('lake_reports') || '[]');
      ReportExporter.exportToCSV(storedReports);
    });
  }
});