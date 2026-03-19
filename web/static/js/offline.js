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
        // Compare by calendar date, not timestamp, so a card scheduled for "today
        // at 15:00" is available from midnight — matching the backend's logic of
        // next_due < CURRENT_DATE + 1 day.
        var due   = new Date(review.next_due);
        var today = new Date();
        due.setHours(0, 0, 0, 0);
        today.setHours(0, 0, 0, 0);
        return due <= today;
    }

    function matchesTopic(card, topic) {
        if (!topic) return true;
        return card.topic && card.topic === topic;
    }

    // Picks a random element from an array (Fisher-Yates single draw).
    function pickRandFrom(arr) {
        return arr[Math.floor(Math.random() * arr.length)];
    }

    function pickNextDue(cards, reviews, topic) {
        // Collect all due cards that match the topic filter.
        var due = [];
        for (var i = 0; i < cards.length; i++) {
            var c = cards[i];
            if (!matchesTopic(c, topic)) continue;
            if (isDue(reviews[c.id])) due.push(c);
        }
        if (!due.length) return null;

        // Sort by effective due date ascending (cards never reviewed → epoch 0,
        // so they share the highest priority). This mirrors the backend's
        // ORDER BY COALESCE(next_due, '1970-01-01').
        due.sort(function (a, b) {
            var da = reviews[a.id] ? new Date(reviews[a.id].next_due).getTime() : 0;
            var db = reviews[b.id] ? new Date(reviews[b.id].next_due).getTime() : 0;
            return da - db;
        });

        // Among cards that share the oldest due date, pick randomly (tiebreak)
        // to avoid always seeing new cards in the same insertion order.
        var oldest = due[0];
        var oldestTime = reviews[oldest.id] ? new Date(reviews[oldest.id].next_due).getTime() : 0;
        var tied = due.filter(function (c) {
            var t = reviews[c.id] ? new Date(reviews[c.id].next_due).getTime() : 0;
            return t === oldestTime;
        });
        return pickRandFrom(tied);
    }

    function pickRandom(cards, topic) {
        var filtered = cards.filter(function (c) { return matchesTopic(c, topic); });
        if (!filtered.length) return null;
        return pickRandFrom(filtered);
    }

    function pickWrong(cards, reviews, topic) {
        // Collect wrong cards (last_result = 0) that are due.
        var wrong = [];
        for (var i = 0; i < cards.length; i++) {
            var c = cards[i];
            if (!matchesTopic(c, topic)) continue;
            var rv = reviews[c.id];
            if (rv && rv.last_result === 0 && isDue(rv)) wrong.push(c);
        }
        if (!wrong.length) return pickNextDue(cards, reviews, topic);

        // Most recently wrong first (mirrors backend ORDER BY updated_at DESC),
        // then random tiebreak within same updated_at.
        wrong.sort(function (a, b) {
            var ta = reviews[a.id] ? new Date(reviews[a.id].updated_at).getTime() : 0;
            var tb = reviews[b.id] ? new Date(reviews[b.id].updated_at).getTime() : 0;
            return tb - ta; // descending
        });
        var newest = reviews[wrong[0].id] ? new Date(reviews[wrong[0].id].updated_at).getTime() : 0;
        var tied = wrong.filter(function (c) {
            var t = reviews[c.id] ? new Date(reviews[c.id].updated_at).getTime() : 0;
            return t === newest;
        });
        return pickRandFrom(tied);
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
     * loadBundle — reads all cards + reviews for a deck from IndexedDB and
     * returns them in the same shape as GET /api/study/offline so that
     * study.js can drive one unified buildQueue() for both online and offline
     * sessions. Returns null when the deck has no cached cards.
     */
    function loadBundle(deckId) {
        return idbGetAllByIndex("cards", "deck_id", deckId).then(function (cards) {
            if (!cards || !cards.length) return null;
            return Promise.all(cards.map(function (c) { return idbGet("reviews", c.id); }))
                .then(function (reviewArr) {
                    var reviews = {};
                    reviewArr.forEach(function (rv) {
                        if (rv) reviews[rv.card_id] = rv;
                    });
                    return { cards: cards, reviews: reviews };
                });
        });
    }

    /**
     * saveBundle — persists a pre-fetched bundle (cards + reviews map) to
     * IndexedDB without re-fetching from the network. Called by study.js
     * after fetching the bundle online so the same data is immediately
     * available for offline use on the next visit.
     */
    function saveBundle(deckId, bundle) {
        return getDB().then(function (db) {
            return new Promise(function (resolve, reject) {
                var tx = db.transaction(["cards", "reviews", "deck_meta"], "readwrite");
                tx.onerror    = function () { reject(tx.error); };
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
    }

    /**
     * nextCard — returns the next card to study from IDB for the given deck
     * and mode ("due", "random", "wrong"). Returns null when no card available.
     *
     * @param {string}   deckId
     * @param {string}   mode        "due" | "random" | "wrong"
     * @param {string}   topic       "" = all topics
     * @param {string[]} excludeIDs  card IDs already shown this session (optional)
     */
    function nextCard(deckId, mode, topic, excludeIDs) {
        var excluded = excludeIDs && excludeIDs.length ? excludeIDs : [];

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

                // Remove already-seen cards from the candidate pool.
                var candidates = excluded.length
                    ? cards.filter(function (c) { return excluded.indexOf(c.id) === -1; })
                    : cards;

                if (!candidates.length) return null;

                if (mode === "random") return pickRandom(candidates, topic);
                if (mode === "wrong")  return pickWrong(candidates, reviews, topic);
                return pickNextDue(candidates, reviews, topic);
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
            // Uses a sentinel object to abort the chain early on 401.
            var ABORT = { aborted: true };

            return items.reduce(function (chain, item) {
                return chain.then(function (state) {
                    // Previous iteration triggered an abort (session expired).
                    if (state && state.aborted) return state;
                    var sent = state || 0;

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
                            // Success or already applied — remove from queue.
                            return idbDelete("answer_queue", item._id).then(function () {
                                return sent + 1;
                            });
                        }
                        if (res.status === 401) {
                            // Session expired — stop flushing. Items stay in the
                            // queue so they sync automatically after re-login.
                            return ABORT;
                        }
                        return sent; // other server error — keep item, try later
                    }).catch(function () {
                        return sent; // network error — keep item, try later
                    });
                });
            }, Promise.resolve(0)).then(function (state) {
                return (state && state.aborted) ? 0 : state;
            });
        });
    }

    // ── Expose ───────────────────────────────────────────────────────────────

    window.offlineStudy = {
        syncDeck:      syncDeck,
        isDeckCached:  isDeckCached,
        loadBundle:    loadBundle,
        saveBundle:    saveBundle,
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
