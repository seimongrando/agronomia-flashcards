/**
 * offline.js — Gerenciador de estudo offline para Agronomia Flashcards.
 *
 * Responsabilidades:
 *   1. IndexedDB: armazenar cards e estado de review localmente.
 *   2. SM-2 em JavaScript: espelho exato do algoritmo Go (service/study.go).
 *   3. Fila de respostas: salvar offline, enviar quando conectar.
 *   4. Indicador de estado: pendingCount() para o UI.
 *
 * Interface pública em window.offlineStudy:
 *   syncDeck(deckId)           → Promise — baixa cards + reviews para IDB
 *   isDeckCached(deckId)       → Promise<boolean>
 *   nextCard(deckId, mode, topic) → Promise<card|null>
 *   recordAnswer(cardId, result)  → Promise — atualiza SM-2 local + fila
 *   pendingCount()             → Promise<number>
 *   flushQueue()               → Promise — envia fila ao servidor
 */
(function () {
    "use strict";

    var DB_NAME    = "agro-offline-v1";
    var DB_VERSION = 1;

    // ── IndexedDB setup ───────────────────────────────────────────────────────

    var dbPromise = new Promise(function (resolve, reject) {
        var req = indexedDB.open(DB_NAME, DB_VERSION);

        req.onupgradeneeded = function (e) {
            var db = e.target.result;
            // Cards: all card data for offline study (includes answer).
            if (!db.objectStoreNames.contains("cards")) {
                var cards = db.createObjectStore("cards", { keyPath: "id" });
                cards.createIndex("deck_id", "deck_id", { unique: false });
            }
            // Reviews: local SM-2 state per card (mirrored from server + updated offline).
            if (!db.objectStoreNames.contains("reviews")) {
                db.createObjectStore("reviews", { keyPath: "card_id" });
            }
            // answer_queue: pending answers to send when back online.
            if (!db.objectStoreNames.contains("answer_queue")) {
                db.createObjectStore("answer_queue", { autoIncrement: true, keyPath: "_id" });
            }
            // deck_meta: tracks which decks are cached and when.
            if (!db.objectStoreNames.contains("deck_meta")) {
                db.createObjectStore("deck_meta", { keyPath: "deck_id" });
            }
        };
        req.onsuccess = function (e) { resolve(e.target.result); };
        req.onerror   = function (e) { reject(e.target.error); };
    });

    function getDB() { return dbPromise; }

    // Generic IDB helpers returning Promises.
    function idbGet(store, key) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readonly");
                var req = tx.objectStore(store).get(key);
                req.onsuccess = function () { resolve(req.result); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    function idbPut(store, value) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readwrite");
                var req = tx.objectStore(store).put(value);
                req.onsuccess = function () { resolve(); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    function idbGetAll(store) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readonly");
                var req = tx.objectStore(store).getAll();
                req.onsuccess = function () { resolve(req.result); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    function idbGetAllByIndex(store, index, key) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readonly");
                var req = tx.objectStore(store).index(index).getAll(key);
                req.onsuccess = function () { resolve(req.result); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    function idbDelete(store, key) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readwrite");
                var req = tx.objectStore(store).delete(key);
                req.onsuccess = function () { resolve(); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    function idbClear(store) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(store, "readwrite");
                var req = tx.objectStore(store).clear();
                req.onsuccess = function () { resolve(); };
                req.onerror   = function () { reject(req.error); };
            });
        });
    }

    // ── SM-2 Algorithm (mirrors service/study.go Schedule()) ─────────────────

    var SM2_MIN_EF  = 1.3;
    var SM2_MAX_EF  = 2.5;
    var SM2_INIT_EF = 2.5;

    function sm2Schedule(result, streak, intervalDays, easeFactor) {
        if (!easeFactor || easeFactor <= 0) easeFactor = SM2_INIT_EF;
        if (!intervalDays || intervalDays <= 0) intervalDays = 1;
        if (!streak) streak = 0;

        var nextInterval, newEF, newStreak;

        function clamp(ef) { return Math.min(SM2_MAX_EF, Math.max(SM2_MIN_EF, ef)); }
        function max1(n)   { return Math.max(1, Math.round(n)); }

        if (result === 0) {                          // wrong
            nextInterval = 1;
            newEF        = clamp(easeFactor - 0.20);
            newStreak    = 0;
        } else if (result === 1) {                   // hard
            nextInterval = max1(intervalDays * 1.2);
            newEF        = clamp(easeFactor - 0.15);
            newStreak    = streak + 1;
        } else {                                     // correct
            if      (streak === 0) nextInterval = 1;
            else if (streak === 1) nextInterval = 6;
            else                   nextInterval = max1(intervalDays * easeFactor);
            newEF     = clamp(easeFactor + 0.10);
            newStreak = streak + 1;
        }

        var nextDue = new Date();
        nextDue.setDate(nextDue.getDate() + nextInterval);

        return {
            streak:       newStreak,
            ease_factor:  newEF,
            interval_days: nextInterval,
            next_due:     nextDue.toISOString(),
            last_result:  result
        };
    }

    // ── Card selection helpers ────────────────────────────────────────────────

    function isDue(review) {
        if (!review) return true; // never reviewed = due
        return new Date(review.next_due) <= new Date();
    }

    function matchesTopic(card, topic) {
        if (!topic) return true;
        return card.topic && card.topic === topic;
    }

    function pickNextDue(cards, reviews, topic) {
        // Return first card that is due and matches topic filter.
        for (var i = 0; i < cards.length; i++) {
            var c = cards[i];
            if (!matchesTopic(c, topic)) continue;
            if (isDue(reviews[c.id])) return c;
        }
        return null;
    }

    function pickRandom(cards, topic) {
        var filtered = cards.filter(function (c) { return matchesTopic(c, topic); });
        if (!filtered.length) return null;
        return filtered[Math.floor(Math.random() * filtered.length)];
    }

    function pickWrong(cards, reviews, topic) {
        // Cards with last_result = 0 (wrong) that are due.
        for (var i = 0; i < cards.length; i++) {
            var c = cards[i];
            if (!matchesTopic(c, topic)) continue;
            var rv = reviews[c.id];
            if (rv && rv.last_result === 0 && isDue(rv)) return c;
        }
        return pickNextDue(cards, reviews, topic);
    }

    // ── Public API ────────────────────────────────────────────────────────────

    /**
     * syncDeck — downloads all cards + current review state from the server
     * and saves to IndexedDB. Call once when the deck is opened online.
     */
    function syncDeck(deckId) {
        return fetch("/api/study/offline?deckId=" + encodeURIComponent(deckId), {
            credentials: "same-origin",
            headers: { "X-Requested-With": "XMLHttpRequest" }
        }).then(function (res) {
            if (!res.ok) throw new Error("sync failed " + res.status);
            return res.json();
        }).then(function (bundle) {
            // Write cards to IDB.
            return getDB().then(function (db) {
                return new Promise(function (resolve, reject) {
                    var tx = db.transaction(["cards", "reviews", "deck_meta"], "readwrite");
                    tx.onerror   = function () { reject(tx.error); };
                    tx.oncomplete = resolve;

                    var cardStore   = tx.objectStore("cards");
                    var reviewStore = tx.objectStore("reviews");
                    var metaStore   = tx.objectStore("deck_meta");

                    (bundle.cards || []).forEach(function (c) { cardStore.put(c); });
                    Object.keys(bundle.reviews || {}).forEach(function (cardId) {
                        reviewStore.put(Object.assign({ card_id: cardId }, bundle.reviews[cardId]));
                    });
                    metaStore.put({ deck_id: deckId, synced_at: new Date().toISOString() });
                });
            });
        });
    }

    /**
     * isDeckCached — true if the deck was previously synced.
     */
    function isDeckCached(deckId) {
        return idbGet("deck_meta", deckId).then(function (meta) {
            return !!meta;
        });
    }

    /**
     * nextCard — returns the next card to study from IDB for the given deck
     * and mode ("due", "random", "wrong"). Returns null when no card available.
     */
    function nextCard(deckId, mode, topic) {
        return idbGetAllByIndex("cards", "deck_id", deckId).then(function (cards) {
            if (!cards || !cards.length) return null;
            // Load all reviews for cards in this deck.
            return Promise.all(cards.map(function (c) {
                return idbGet("reviews", c.id);
            })).then(function (reviewArr) {
                var reviews = {};
                reviewArr.forEach(function (rv, i) {
                    if (rv) reviews[cards[i].id] = rv;
                });
                if (mode === "random") return pickRandom(cards, topic);
                if (mode === "wrong")  return pickWrong(cards, reviews, topic);
                return pickNextDue(cards, reviews, topic);
            });
        });
    }

    /**
     * recordAnswer — updates the local SM-2 state for a card and queues the
     * answer for later sync to the server.
     */
    function recordAnswer(cardId, result) {
        return idbGet("reviews", cardId).then(function (existing) {
            var streak       = existing ? existing.streak       : 0;
            var intervalDays = existing ? existing.interval_days : 1;
            var easeFactor   = existing ? existing.ease_factor  : SM2_INIT_EF;

            var newState = sm2Schedule(result, streak, intervalDays, easeFactor);
            newState.card_id = cardId;

            return idbPut("reviews", newState).then(function () {
                // Add to the answer queue for later sync.
                return idbPut("answer_queue", {
                    card_id:     cardId,
                    result:      result,
                    answered_at: new Date().toISOString()
                });
            });
        });
    }

    /**
     * pendingCount — number of answers waiting to be synced to the server.
     */
    function pendingCount() {
        return idbGetAll("answer_queue").then(function (all) {
            return all ? all.length : 0;
        });
    }

    /**
     * flushQueue — sends all queued answers to POST /api/study/answer.
     * Removes each entry from the queue on success.
     * Safe to call repeatedly; idempotent on already-processed answers.
     */
    function flushQueue() {
        return idbGetAll("answer_queue").then(function (items) {
            if (!items || !items.length) return 0;

            // Send sequentially to avoid race conditions on the server.
            return items.reduce(function (chain, item) {
                return chain.then(function (sent) {
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
                            return idbDelete("answer_queue", item._id).then(function () {
                                return sent + 1;
                            });
                        }
                        return sent; // keep in queue if server error
                    }).catch(function () {
                        return sent; // network error — keep in queue
                    });
                });
            }, Promise.resolve(0));
        });
    }

    // ── Expose ───────────────────────────────────────────────────────────────

    window.offlineStudy = {
        syncDeck:      syncDeck,
        isDeckCached:  isDeckCached,
        nextCard:      nextCard,
        recordAnswer:  recordAnswer,
        pendingCount:  pendingCount,
        flushQueue:    flushQueue
    };

    // Auto-flush when connectivity is restored.
    window.addEventListener("online", function () {
        offlineStudy.flushQueue().then(function (sent) {
            if (sent > 0) {
                window.toast && window.toast(sent + " resposta(s) sincronizada(s).", "success");
            }
        });
    });
})();
