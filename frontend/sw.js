const CACHE_NAME = 'guardians-lake-v1';
const APP_SHELL = [
  '/', '/index.html', '/dashboard.html', '/verify.html', '/css/style.css',
  '/js/api.js', '/js/dashboard.js', '/js/report-form.js', '/js/report.js',
  '/js/verify.js', '/js/ws-client.js', '/js/offline-queue.js', '/js/ai-classifier.js',
];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL)));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))))
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response.ok && new URL(event.request.url).origin === self.location.origin) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        }
        return response;
      })
      .catch(() => caches.match(event.request).then((cached) => cached || caches.match('/dashboard.html')))
  );
});
