/* Agronomia Flashcards — Service Worker */
const CACHE = 'agro-v4';

// Only pre-cache static assets that change rarely (images / icons).
// HTML, CSS and JS use network-first so code updates are always reflected
// immediately without needing to bump this version number again.
const PRECACHE = [
  '/static/icons/icon-192.png',
  '/static/icons/icon-512.png',
];

self.addEventListener('install', e => {
  e.waitUntil(
    caches.open(CACHE)
      .then(c => c.addAll(PRECACHE))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', e => {
  // Delete ALL previous cache versions so stale JS/CSS is never served again.
  e.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);

  // Only handle same-origin GET requests.
  // Excludes chrome-extension://, Google Fonts, avatar images, etc.
  if (url.origin !== self.location.origin) return;
  if (e.request.method !== 'GET') return;

  // Never intercept API, auth or the SW itself.
  if (url.pathname.startsWith('/api/')) return;
  if (url.pathname.startsWith('/auth/')) return;
  if (url.pathname === '/sw.js') return;

  // Helper: store a response in cache without consuming the original.
  // The clone must be created synchronously before any async gap,
  // otherwise the original body may already be consumed.
  function storeInCache(request, response) {
    const clone = response.clone(); // synchronous clone — body not yet consumed
    caches.open(CACHE).then(c => c.put(request, clone));
  }

  // Images: cache-first (icons are stable; fall back to cache when offline).
  if (e.request.destination === 'image') {
    e.respondWith(
      caches.match(e.request).then(cached => {
        if (cached) return cached;
        return fetch(e.request).then(res => {
          if (res.ok) storeInCache(e.request, res);
          return res;
        });
      })
    );
    return;
  }

  // HTML, CSS, JS: network-first — ensures code updates are always live.
  // Falls back to cache only when fully offline.
  e.respondWith(
    fetch(e.request)
      .then(res => {
        if (res.ok) storeInCache(e.request, res);
        return res;
      })
      .catch(() => caches.match(e.request))
  );
});
