/**
 * Service Worker — Agronomia Flashcards
 *
 * Estratégias:
 *   - HTML/CSS/JS: network-first com cache fallback (garantia de atualizações).
 *   - Imagens/ícones: cache-first (estáticos, quase nunca mudam).
 *   - Background Sync: fila de respostas offline enviada ao reconectar.
 *   - Push Notifications: exibe notificação ao receber evento do servidor.
 */
"use strict";

var CACHE     = "agro-v6";
var SYNC_TAG  = "agro-answer-queue";

// Critical files that must be available for offline use.
// Cached eagerly on install so the study page works even on first offline visit.
var SHELL_FILES = [
    "/",
    "/study.html",
    "/static/css/style.css",
    "/static/js/app.js",
    "/static/js/offline.js",
    "/static/js/study.js",
    "/static/manifest.json",
    "/static/icons/icon-192.png",
    "/static/icons/icon-512.png"
];

// ── Install: pré-carrega o app shell e ativa imediatamente ───────────────────
self.addEventListener("install", function (e) {
    e.waitUntil(
        caches.open(CACHE).then(function (cache) {
            // Individual adds so a single missing icon doesn't abort the install.
            return Promise.all(
                SHELL_FILES.map(function (url) {
                    return cache.add(url).catch(function (err) {
                        console.warn("[SW] pré-cache falhou para " + url + ":", err);
                    });
                })
            );
        }).then(function () { return self.skipWaiting(); })
    );
});

// ── Activate: limpa caches antigos ────────────────────────────────────────────
self.addEventListener("activate", function (e) {
    e.waitUntil(
        caches.keys().then(function (keys) {
            return Promise.all(
                keys.filter(function (k) { return k !== CACHE; })
                    .map(function (k)   { return caches.delete(k); })
            );
        }).then(function () { return self.clients.claim(); })
    );
});

// ── Fetch ─────────────────────────────────────────────────────────────────────
self.addEventListener("fetch", function (e) {
    var url = new URL(e.request.url);

    // Only handle same-origin GET requests.
    if (url.origin !== self.location.origin) return;
    if (e.request.method !== "GET") return;

    // Never intercept auth, the SW itself, or API calls
    // (API offline fallback is handled in study.js + offline.js via IDB).
    if (url.pathname.startsWith("/api/"))  return;
    if (url.pathname.startsWith("/auth/")) return;
    if (url.pathname === "/sw.js")         return;

    // Helper: cache a network response without consuming it.
    function storeInCache(req, res) {
        var clone = res.clone();
        caches.open(CACHE).then(function (c) { c.put(req, clone); });
    }

    // Images: cache-first (icons are stable; fall back to cache when offline).
    if (e.request.destination === "image") {
        e.respondWith(
            caches.match(e.request).then(function (cached) {
                if (cached) return cached;
                return fetch(e.request).then(function (res) {
                    if (res.ok) storeInCache(e.request, res);
                    return res;
                });
            })
        );
        return;
    }

    // HTML / CSS / JS: network-first — live code updates always win.
    // Falls back to cache so the shell loads when fully offline.
    e.respondWith(
        fetch(e.request)
            .then(function (res) {
                if (res.ok) storeInCache(e.request, res);
                return res;
            })
            .catch(function () { return caches.match(e.request); })
    );
});

// ── Background Sync ───────────────────────────────────────────────────────────
// The page registers a sync tag ("agro-answer-queue") after queuing answers
// offline. The browser fires this event when connectivity is restored.
self.addEventListener("sync", function (e) {
    if (e.tag !== SYNC_TAG) return;
    e.waitUntil(flushAnswerQueue());
});

/**
 * flushAnswerQueue — opens the IndexedDB answer_queue store directly from
 * the service worker and sends each pending answer to POST /api/study/answer.
 * Mirrors the logic in offline.js:flushQueue() but runs in the SW context.
 */
function flushAnswerQueue() {
    return openDB().then(function (db) {
        return getAllFromStore(db, "answer_queue").then(function (items) {
            if (!items || !items.length) return;

            return items.reduce(function (chain, item) {
                return chain.then(function () {
                    return fetch("/api/study/answer", {
                        method: "POST",
                        credentials: "same-origin",
                        headers: {
                            "Content-Type": "application/json",
                            "X-Requested-With": "XMLHttpRequest"
                        },
                        body: JSON.stringify({ card_id: item.card_id, result: item.result })
                    }).then(function (res) {
                        if (res.ok || res.status === 409) {
                            return deleteFromStore(db, "answer_queue", item._id);
                        }
                    }).catch(function () {
                        // Network error — leave in queue for next sync attempt.
                    });
                });
            }, Promise.resolve());
        });
    }).catch(function (err) {
        console.warn("[SW] flushAnswerQueue error:", err);
    });
}

// ── Push Notifications ────────────────────────────────────────────────────────
self.addEventListener("push", function (e) {
    var data = {};
    try {
        data = e.data ? e.data.json() : {};
    } catch (_) {
        data = { title: "Agronomia Flashcards", body: e.data ? e.data.text() : "" };
    }

    var title   = data.title || "Agronomia Flashcards";
    var options = {
        body:             data.body  || "Você tem cards para revisar hoje!",
        icon:             data.icon  || "/static/icons/icon-192.png",
        badge:            data.badge || "/static/icons/icon-192.png",
        tag:              "agro-daily-reminder",  // replaces previous notification (no spam)
        renotify:         false,
        requireInteraction: false,
        data: { url: data.url || "/" }
    };

    e.waitUntil(self.registration.showNotification(title, options));
});

// Open notification → navigate to the study page.
self.addEventListener("notificationclick", function (e) {
    e.notification.close();
    var targetUrl = (e.notification.data && e.notification.data.url) || "/";
    e.waitUntil(
        self.clients.matchAll({ type: "window", includeUncontrolled: true })
            .then(function (clients) {
                // Focus an existing window if available.
                for (var i = 0; i < clients.length; i++) {
                    if (clients[i].url === targetUrl && "focus" in clients[i]) {
                        return clients[i].focus();
                    }
                }
                // Otherwise open a new tab.
                if (self.clients.openWindow) {
                    return self.clients.openWindow(targetUrl);
                }
            })
    );
});

// ── IDB helpers (service-worker context — no offline.js available) ────────────

var DB_NAME    = "agro-offline-v1";
var DB_VERSION = 1;

function openDB() {
    return new Promise(function (resolve, reject) {
        var req = indexedDB.open(DB_NAME, DB_VERSION);
        req.onsuccess = function (e) { resolve(e.target.result); };
        req.onerror   = function (e) { reject(e.target.error); };
    });
}

function getAllFromStore(db, storeName) {
    return new Promise(function (resolve, reject) {
        var tx  = db.transaction(storeName, "readonly");
        var req = tx.objectStore(storeName).getAll();
        req.onsuccess = function () { resolve(req.result); };
        req.onerror   = function () { reject(req.error); };
    });
}

function deleteFromStore(db, storeName, key) {
    return new Promise(function (resolve, reject) {
        var tx  = db.transaction(storeName, "readwrite");
        var req = tx.objectStore(storeName).delete(key);
        req.onsuccess = function () { resolve(); };
        req.onerror   = function () { reject(req.error); };
    });
}
