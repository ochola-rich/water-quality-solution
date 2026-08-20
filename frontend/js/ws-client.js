/**
 * WebSocket Live Stream Client & Alert Dispatcher
 */
export class LiveTelemetryClient {
  constructor(url = 'wss://lake-telemetry.kisumu.org/stream', onMessageCallback) {
    this.url = url;
    this.onMessage = onMessageCallback;
    this.socket = null;
    this.reconnectTimer = null;
    this.subscribers = new Set();
  }

  connect() {
    try {
      this.socket = new WebSocket(this.url);

      this.socket.onopen = () => {
        console.log('[WS] Connected to Lake Victoria telemetry stream');
        this.emitStatus('LIVE');
      };

      this.socket.onmessage = (event) => {
        const payload = JSON.parse(event.data);
        this.notify(payload);
      };

      this.socket.onclose = () => {
        console.warn('[WS] Stream disconnected. Reconnecting in 3s...');
        this.emitStatus('RECONNECTING');
        this.reconnectTimer = setTimeout(() => this.connect(), 3000);
      };

      this.socket.onerror = (err) => {
        console.error('[WS Error]', err);
        this.simulateFallbackTelemetry();
      };
    } catch (e) {
      this.simulateFallbackTelemetry();
    }
  }

  subscribe(callback) {
    this.subscribers.add(callback);
    return () => this.subscribers.delete(callback);
  }

  notify(data) {
    this.subscribers.forEach((cb) => cb(data));
  }

  emitStatus(status) {
    const el = document.getElementById('live-status-indicator');
    if (el) el.querySelector('span').textContent = status;
  }

  // Simulated live telemetry loop for offline/preview environments
  simulateFallbackTelemetry() {
    setInterval(() => {
      const mockEvent = {
        type: 'TELEMETRY_PULSE',
        timestamp: new Date().toLocaleTimeString(),
        hotspotId: 'hs-1',
        metrics: {
          dissolvedOxygen: (6.2 + (Math.random() * 0.4 - 0.2)).toFixed(2),
          turbidity: (24.5 + (Math.random() * 1.5 - 0.75)).toFixed(1),
          temp: (26.0 + (Math.random() * 0.2 - 0.1)).toFixed(1)
        }
      };
      this.notify(mockEvent);
    }, 4000);
  }
}

export const wsClient = new LiveTelemetryClient();