/**
 * Native WebSocket Stream Client
 * File: frontend/js/ws-client.js
 */

export class LiveTelemetryClient {
  constructor(url = '/ws/dashboard') {
    this.url = url;
    this.ws = null;
    this.handlers = new Map();
    this.isConnected = false;
  }

  connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host || 'localhost:3000';
    const wsUrl = `${protocol}//${host}${this.url}`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        this.isConnected = true;
        this.emit('connection', { status: 'connected' });
        console.log('[WS] Connected to Go WebSocket stream');
      };

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          const type = data.type || data.event || 'message';
          this.emit(type, data.payload || data);
        } catch (e) {
          console.error('[WS] Error parsing message:', e);
        }
      };

      this.ws.onclose = () => {
        this.isConnected = false;
        this.emit('connection', { status: 'disconnected' });
        setTimeout(() => this.connect(), 5000);
      };

      this.ws.onerror = () => {
        this.isConnected = false;
        this.emit('connection', { status: 'error' });
      };
    } catch (e) {
      console.warn('[WS] Native WebSocket not reachable, using fallback.');
    }
  }

  on(event, callback) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, []);
    }
    this.handlers.get(event).push(callback);
  }

  emit(event, data) {
    if (this.handlers.has(event)) {
      this.handlers.get(event).forEach(cb => cb(data));
    }
  }
}

export const wsClient = new LiveTelemetryClient();