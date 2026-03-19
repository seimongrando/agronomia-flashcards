(function () {
    "use strict";

    var params      = new URLSearchParams(window.location.search);
    var deckId      = params.get("deckId");
    var mode        = params.get("mode") || "due";
    var deckName    = params.get("deckName")    ? decodeURIComponent(params.get("deckName"))    : null;
    var deckSubject = params.get("deckSubject") ? decodeURIComponent(params.get("deckSubject")) : null;

    /* ── Stable DOM refs (never replaced) ─────────────────────────────────── */
    var counterEl     = document.getElementById("study-counter");
    var deckCtxEl     = document.getElementById("deck-context");
    // topic-filter element removed; topics are no longer displayed on the study page
    var spinnerEl     = document.getElementById("study-spinner");
    var doneEl        = document.getElementById("study-done");
    var cardAreaEl    = document.getElementById("card-area");
    var cardDragEl    = document.getElementById("card-drag");
    var cardSceneEl   = document.getElementById("card-scene");
    var flashcardEl   = document.getElementById("flashcard");
    var headFront     = document.getElementById("head-front");
    var bodyFront     = document.getElementById("body-front");
    var headBack      = document.getElementById("head-back");
    var bodyBack      = document.getElementById("body-back");
    var answerBar     = document.getElementById("answer-bar");
    var answerHelp    = document.getElementById("answer-help");
    var btnHelpToggle = document.getElementById("btn-help-toggle");
    var btnWrong      = document.getElementById("btn-wrong");
    var btnHard       = document.getElementById("btn-hard");
    var btnRight      = document.getElementById("btn-right");
    var lblWrong      = document.getElementById("lbl-wrong");
    var lblRight      = document.getElementById("lbl-right");
    var lblHard       = document.getElementById("lbl-hard");
    var progressFill  = document.getElementById("progress-fill");
    var progressLbl   = document.getElementById("progress-label");
    var progressTrack = document.getElementById("progress-track");

    /* ── State ─────────────────────────────────────────────────────────────── */
    var currentCard    = null;
    var flipped        = false;
    var submitting     = false;
    var sessionCount   = 0;     // cards answered in this session
    var sessionCorrect = 0;     // result = 2
    var sessionHard    = 0;     // result = 1
    var sessionWrong   = 0;     // result = 0
    var sessionQueue   = [];    // ordered queue of cards; consumed via shift()
    var sessionBundle  = null;  // { cards, reviews } kept to rebuild queue for next random round
    var sessionTotal   = null;  // queue length fixed at session start — denominator for "Carta X de Y"

    // Session-expiry banner — shown at most once per study session when the JWT
    // expires mid-session; differs from the network-offline banner.
    var sessionExpiredBannerEl   = null;
    var sessionExpiredBannerShown = false;

    /* ── Swipe / drag state ─────────────────────────────────────────────────  */
    var SWIPE_THRESHOLD = 72;
    var LABEL_THRESHOLD = 36;
    var isDragging = false;
    var dragLocked = false;
    var startX = 0, startY = 0;
    var dragX  = 0, dragY  = 0;

    /* ─── Offline indicator ─────────────────────────────────────────────────── */
    var offlineBanner      = null;
    var offlineBannerTimer = null;

    function setOfflineBanner(offline) {
        if (offline && !offlineBanner) {
            offlineBanner = document.createElement("div");
            offlineBanner.className = "offline-banner";
            offlineBanner.setAttribute("role", "status");

            var msg = document.createElement("span");
            msg.textContent = "Modo offline — suas respostas serão salvas e enviadas ao reconectar.";

            var closeBtn = document.createElement("button");
            closeBtn.className = "offline-banner__close";
            closeBtn.setAttribute("aria-label", "Fechar aviso");
            closeBtn.innerHTML =
                '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                '<path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>' +
                '</svg>';
            closeBtn.addEventListener("click", function () { setOfflineBanner(false); });

            offlineBanner.appendChild(msg);
            offlineBanner.appendChild(closeBtn);
            document.body.appendChild(offlineBanner);

            // Auto-dismiss after 8 s so it never permanently blocks the UI.
            offlineBannerTimer = setTimeout(function () { setOfflineBanner(false); }, 8000);

        } else if (!offline && offlineBanner) {
            if (offlineBannerTimer) { clearTimeout(offlineBannerTimer); offlineBannerTimer = null; }
            offlineBanner.remove();
            offlineBanner = null;
        }
    }
    window.addEventListener("online",  function () { setOfflineBanner(false); });
    window.addEventListener("offline", function () { setOfflineBanner(true);  });

    // Shows a sticky (non-auto-dismissing) banner when the session expires
    // mid-study. Shown at most once so it doesn't repeat on every card answer.
    function setSessionExpiredBanner() {
        if (sessionExpiredBannerShown || sessionExpiredBannerEl) return;
        sessionExpiredBannerShown = true;

        sessionExpiredBannerEl = document.createElement("div");
        sessionExpiredBannerEl.className = "offline-banner offline-banner--warn";
        sessionExpiredBannerEl.setAttribute("role", "alert");

        var msg = document.createElement("span");
        msg.textContent = "Sessão expirada — suas respostas estão salvas e serão sincronizadas após o login.";

        var loginBtn = document.createElement("a");
        loginBtn.href = "/";
        loginBtn.className = "offline-banner__login";
        loginBtn.textContent = "Ir para login";

        sessionExpiredBannerEl.appendChild(msg);
        sessionExpiredBannerEl.appendChild(loginBtn);
        document.body.appendChild(sessionExpiredBannerEl);
    }

    /* ════════════════════════════════════════════════════════════════════════
       TEXT RENDERING
       Converts plain-text card content (with newlines) to safe HTML.
       Double newlines become paragraph breaks; single newlines become <br>.
    ════════════════════════════════════════════════════════════════════════ */
    function renderCardText(text) {
        if (!text) return "";
        // Escape HTML to prevent XSS
        var escaped = app.esc(text);
        // Normalise Windows line endings
        escaped = escaped.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
        // Double newline → paragraph break
        var paragraphs = escaped.split(/\n{2,}/);
        if (paragraphs.length > 1) {
            return paragraphs
                .map(function (p) { return "<p>" + p.replace(/\n/g, "<br>") + "</p>"; })
                .join("");
        }
        // Single newlines → <br>
        return "<p>" + escaped.replace(/\n/g, "<br>") + "</p>";
    }

    /* ════════════════════════════════════════════════════════════════════════
       INIT
    ════════════════════════════════════════════════════════════════════════ */
    function init() {
        if (!deckId) {
            showDone("Deck não encontrado", "Volte e escolha um deck.", true);
            return;
        }

        // Sets up the study UI after authentication (or offline bypass).
        // `user` may be null when offline with a pre-cached deck.
        function startStudy(user) {
            app.renderTopbar(user, { backHref: "/", noNav: true });
            renderDeckContext();

            var isOffline = !user || !navigator.onLine;
            if (isOffline) setOfflineBanner(true);
            initSession(isOffline);
        }

        // Attempt to reach /api/me.  Returns null on both auth failure AND
        // network failure (offline), so we must distinguish the two cases.
        app.checkAuth().then(function (user) {
            if (user) {
                startStudy(user);
                return;
            }

            // Auth returned null.  If the device is offline, try IndexedDB.
            if (!navigator.onLine && window.offlineStudy) {
                offlineStudy.isDeckCached(deckId)
                    .then(function (cached) {
                        if (cached) {
                            startStudy(null); // offline mode — no user object
                        } else {
                            showDone(
                                "Sem conexão",
                                "Você está offline e este deck ainda não foi sincronizado. " +
                                "Conecte-se à internet e abra o deck uma vez para ativar o modo offline.",
                                true
                            );
                        }
                    })
                    .catch(function () { window.location.href = "/"; });
                return;
            }

            // Online but auth failed (expired session) — send to login.
            window.location.href = "/";
        });

        setupFlip();
        setupSwipe();
        setupButtons();
        setupKeyboard();
        setupHelp();
    }

    /* ════════════════════════════════════════════════════════════════════════
       FLIP  —  3-D card turn
       All tap-to-flip logic lives in onDragEnd (unified for web + mobile).
       setPointerCapture is always called so click events land on cardDragEl
       (no handler) and never double-fire with the tap detection below.
       Keyboard navigation is kept on flashcardEl for accessibility.
    ════════════════════════════════════════════════════════════════════════ */
    function setupFlip() {
        flashcardEl.addEventListener("keydown", function (e) {
            if (e.key === " " || e.key === "Enter") {
                e.preventDefault();
                if (!currentCard || submitting) return;
                toggleFlip();
            }
        });
    }

    function toggleFlip() {
        flipped = !flipped;
        flashcardEl.classList.toggle("is-flipped", flipped);
        setAnswerButtons(flipped && !submitting);
    }

    /* ════════════════════════════════════════════════════════════════════════
       SWIPE  —  Pointer Events (touch + mouse)
    ════════════════════════════════════════════════════════════════════════ */
    function setupSwipe() {
        cardDragEl.addEventListener("pointerdown",   onDragStart, { passive: true });
        cardDragEl.addEventListener("pointermove",   onDragMove,  { passive: true });
        cardDragEl.addEventListener("pointerup",     onDragEnd);
        cardDragEl.addEventListener("pointercancel", onDragCancel);
    }

    function onDragStart(e) {
        if (submitting || !currentCard) return;
        isDragging = true;
        dragLocked = false;
        dragX = 0; dragY = 0;
        startX = e.clientX;
        startY = e.clientY;
        // Always capture so synthesized `click` lands on cardDragEl (no handler),
        // preventing any double-flip race between pointer and click events.
        cardDragEl.setPointerCapture(e.pointerId);
        cardDragEl.style.transition = "none";
    }

    function onDragMove(e) {
        if (!isDragging) return;
        var dx = e.clientX - startX;
        var dy = e.clientY - startY;

        // Lock direction on first significant movement
        if (!dragLocked && (Math.abs(dx) > 6 || Math.abs(dy) > 6)) {
            if (Math.abs(dy) > Math.abs(dx) * 1.6) {
                // Primarily vertical — release so card body can scroll
                isDragging = false;
                dragLocked = true;
                cardDragEl.releasePointerCapture(e.pointerId);
                return;
            }
        }
        if (dragLocked) return;

        dragX = dx;
        dragY = dy;
        cardDragEl.style.transform =
            "translateX(" + dragX + "px) translateY(" + (dragY * 0.25) + "px) rotate(" + (dragX * 0.055) + "deg)";
        updateSwipeLabels(dragX, dragY);
    }

    function onDragEnd(e) {
        if (!isDragging) return;
        isDragging = false;
        hideSwipeLabels();

        var dx = dragX, dy = dragY;
        dragX = 0; dragY = 0;

        // Tap (minimal movement) → toggle flip in either direction, on both web and mobile.
        if (Math.abs(dx) < 8 && Math.abs(dy) < 8) {
            snapBack();
            if (!submitting && currentCard) toggleFlip();
            return;
        }

        // Swipe gestures only make sense when the answer is visible.
        if (!flipped) { snapBack(); return; }

        if (dx < -SWIPE_THRESHOLD) {
            flyAndAnswer(0, "left");
        } else if (dx > SWIPE_THRESHOLD) {
            flyAndAnswer(2, "right");
        } else if (dy < -SWIPE_THRESHOLD) {
            flyAndAnswer(1, "up");
        } else {
            snapBack();
        }
    }

    function onDragCancel() {
        isDragging = false;
        dragX = 0; dragY = 0;
        hideSwipeLabels();
        snapBack();
    }

    function snapBack() {
        cardDragEl.style.transition = "transform .35s cubic-bezier(.34,1.56,.64,1)";
        cardDragEl.style.transform  = "";
        cardDragEl.addEventListener("transitionend", function h() {
            cardDragEl.removeEventListener("transitionend", h);
            cardDragEl.style.transition = "";
        });
    }

    function updateSwipeLabels(dx, dy) {
        lblWrong.classList.toggle("visible", dx < -LABEL_THRESHOLD);
        lblRight.classList.toggle("visible", dx >  LABEL_THRESHOLD);
        lblHard.classList.toggle("visible",  dy < -LABEL_THRESHOLD && Math.abs(dx) < LABEL_THRESHOLD * 1.5);
    }
    function hideSwipeLabels() {
        lblWrong.classList.remove("visible");
        lblRight.classList.remove("visible");
        lblHard.classList.remove("visible");
    }

    /* ════════════════════════════════════════════════════════════════════════
       FLY OFF + ANSWER
    ════════════════════════════════════════════════════════════════════════ */
    function flyAndAnswer(result, direction) {
        if (submitting || !flipped || !currentCard) return;
        submitting = true;
        setAnswerButtons(false);
        hideSwipeLabels();

        var targetX = direction === "left"  ? "-160vw" :
                      direction === "right" ? "160vw"  : "0";
        var targetY = direction === "up" ? "-160vh" : "0";
        var rotate  = direction === "left"  ? "-28deg" :
                      direction === "right" ? "28deg"  : "0";

        cardDragEl.style.transition = "transform .38s ease-in";
        cardDragEl.style.transform  =
            "translateX(" + targetX + ") translateY(" + targetY + ") rotate(" + rotate + ")";

        var flyDone = new Promise(function (r) { setTimeout(r, 390); });
        var apiDone = api.post("/api/study/answer", { card_id: currentCard.id, result: result });

        function onAnswerSuccess() {
            sessionCount++;
            if      (result === 2) sessionCorrect++;
            else if (result === 1) sessionHard++;
            else                   sessionWrong++;

            updateProgress();
            loadNext();
        }

        function onAnswerError() {
            submitting = false;
            cardDragEl.style.transition = "transform .35s cubic-bezier(.34,1.56,.64,1)";
            cardDragEl.style.transform  = "";
            setTimeout(function () {
                cardDragEl.style.transition = "";
                setAnswerButtons(true);
            }, 360);
        }

        var savedCardId = currentCard ? currentCard.id : null;

        Promise.all([flyDone, apiDone]).then(function (results) {
            var res = results[1];

            // Session expired (401) — queue locally and show a specific warning.
            // Do NOT show the "offline" banner since the device IS connected;
            // the problem is the auth token, not the network.
            if (res.status === 401) {
                if (!window.offlineStudy || !savedCardId) {
                    onAnswerError();
                    return;
                }
                offlineStudy.recordAnswer(savedCardId, result)
                    .then(function () {
                        setSessionExpiredBanner();
                        onAnswerSuccess();
                    })
                    .catch(function () { onAnswerError(); });
                return;
            }

            if (!res.ok) throw new Error("answer failed");
            onAnswerSuccess();
        }).catch(function () {
            // True network error (fetch rejected / no response) — save to offline
            // queue so the answer is sent as soon as connectivity is restored.
            if (!window.offlineStudy || !savedCardId) { onAnswerError(); return; }
            offlineStudy.recordAnswer(savedCardId, result)
                .then(function () {
                    // Register a Background Sync tag so the SW flushes the queue
                    // even if the user closes the tab before reconnecting.
                    if (navigator.serviceWorker && navigator.serviceWorker.ready) {
                        navigator.serviceWorker.ready.then(function (reg) {
                            if (reg.sync) reg.sync.register("agro-answer-queue").catch(function(){});
                        });
                    }
                    setOfflineBanner(true);
                    onAnswerSuccess();
                })
                .catch(function () { onAnswerError(); });
        });
    }

    /* ════════════════════════════════════════════════════════════════════════
       BUTTONS + KEYBOARD
    ════════════════════════════════════════════════════════════════════════ */
    function setupButtons() {
        btnWrong.addEventListener("click", function () { flyAndAnswer(0, "left"); });
        btnHard.addEventListener("click",  function () { flyAndAnswer(1, "up"); });
        btnRight.addEventListener("click", function () { flyAndAnswer(2, "right"); });
    }

    function setupHelp() {
        if (!btnHelpToggle || !answerHelp) return;
        btnHelpToggle.addEventListener("click", function () {
            var open = !answerHelp.classList.contains("hidden");
            answerHelp.classList.toggle("hidden", open);
            btnHelpToggle.setAttribute("aria-expanded", String(!open));
        });
    }

    function setupKeyboard() {
        document.addEventListener("keydown", function (e) {
            if (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA") return;
            if (e.key === " " || e.key === "Spacebar") {
                e.preventDefault();
                if (currentCard && !submitting) toggleFlip();
            }
            if (e.key === "1") flyAndAnswer(0, "left");
            if (e.key === "2") flyAndAnswer(1, "up");
            if (e.key === "3") flyAndAnswer(2, "right");
        });
    }

    /* ════════════════════════════════════════════════════════════════════════
       SESSION QUEUE  —  deterministic, in-memory card ordering
       Built once at session start from the offline bundle; consumed via
       sessionQueue.shift().  No network calls between cards.
    ════════════════════════════════════════════════════════════════════════ */

    // Returns true when a card is due today (mirrors offline.js isDue).
    function isDue(review) {
        if (!review || !review.next_due) return true;
        var due   = new Date(review.next_due);
        var today = new Date();
        due.setHours(0, 0, 0, 0);
        today.setHours(0, 0, 0, 0);
        return due <= today;
    }

    // "YYYY-MM-DD" in local time — used to group cards by date for shuffle.
    function localDateStr(d) {
        return d.getFullYear() + "-" +
               String(d.getMonth() + 1).padStart(2, "0") + "-" +
               String(d.getDate()).padStart(2, "0");
    }

    // Fisher-Yates in-place shuffle; returns the same array.
    function shuffle(arr) {
        for (var i = arr.length - 1; i > 0; i--) {
            var j = Math.floor(Math.random() * (i + 1));
            var tmp = arr[i]; arr[i] = arr[j]; arr[j] = tmp;
        }
        return arr;
    }

    // Shuffles cards within consecutive same-key groups, preserving group order.
    // Example: cards [A1, A2, B1, B2] → [A2, A1, B1, B2] (A and B shuffled internally).
    function shuffleWithinGroups(sorted, getKey) {
        var result = [];
        var i = 0;
        while (i < sorted.length) {
            var key   = getKey(sorted[i]);
            var group = [sorted[i++]];
            while (i < sorted.length && getKey(sorted[i]) === key) group.push(sorted[i++]);
            result = result.concat(shuffle(group));
        }
        return result;
    }

    /**
     * buildQueue — constructs the ordered card array for the session.
     * @param {Array}  cards   — all cards in the deck (from bundle)
     * @param {Object} reviews — map of cardId → review state (from bundle)
     * @param {string} mode    — "due" | "random" | "wrong"
     * @returns {Array} ordered card objects to study
     */
    function buildQueue(cards, reviews, mode) {
        if (!cards || !cards.length) return [];

        if (mode === "random") {
            return shuffle(cards.slice());
        }

        if (mode === "wrong") {
            var sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
            var wrong = cards.filter(function (c) {
                var rv = reviews[c.id];
                if (!rv || rv.last_result !== 0) return false;
                return new Date(rv.updated_at).getTime() >= sevenDaysAgo;
            });
            if (!wrong.length) {
                // No recent wrong cards — fall back to due mode so the user
                // always sees something when they click "Errei".
                return buildQueue(cards, reviews, "due");
            }
            // Most-recently-wrong first; shuffle within same-timestamp ties.
            wrong.sort(function (a, b) {
                var ta = reviews[a.id] ? new Date(reviews[a.id].updated_at).getTime() : 0;
                var tb = reviews[b.id] ? new Date(reviews[b.id].updated_at).getTime() : 0;
                return tb - ta;
            });
            return shuffleWithinGroups(wrong, function (c) {
                var rv = reviews[c.id];
                return rv ? localDateStr(new Date(rv.updated_at)) : "";
            });
        }

        // mode === "due" (default)
        var due = cards.filter(function (c) { return isDue(reviews[c.id]); });
        if (!due.length) return [];

        // Never-reviewed cards (no entry) get epoch 0 → appear first.
        due.sort(function (a, b) {
            var da = reviews[a.id] ? new Date(reviews[a.id].next_due).getTime() : 0;
            var db = reviews[b.id] ? new Date(reviews[b.id].next_due).getTime() : 0;
            return da - db;
        });
        // Shuffle within cards that share the same calendar day so insertion
        // order never leaks through.
        return shuffleWithinGroups(due, function (c) {
            var rv = reviews[c.id];
            return rv ? localDateStr(new Date(rv.next_due)) : "never";
        });
    }

    /**
     * initSession — fetches the full deck bundle (online or from IDB),
     * builds the session queue, and kicks off study.
     * @param {boolean} isOffline — true = read from IndexedDB, false = fetch API
     */
    function initSession(isOffline) {
        spinnerEl.classList.remove("hidden");
        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");
        doneEl.classList.add("hidden");

        var bundlePromise;
        if (isOffline) {
            bundlePromise = (window.offlineStudy
                ? offlineStudy.loadBundle(deckId).then(function (b) {
                      if (!b) throw new Error("not cached");
                      return b;
                  })
                : Promise.reject(new Error("no offline module")));
        } else {
            bundlePromise = api.get("/api/study/offline?deckId=" + encodeURIComponent(deckId))
                .then(function (res) {
                    if (!res.ok) throw new Error("fetch failed " + res.status);
                    return res.json();
                })
                .then(function (bundle) {
                    // Persist to IDB so this deck is available offline immediately.
                    if (window.offlineStudy) {
                        offlineStudy.saveBundle(deckId, bundle).catch(function () {});
                    }
                    return bundle;
                });
        }

        bundlePromise
            .then(function (bundle) {
                sessionBundle = bundle;
                var cards   = bundle.cards   || [];
                var reviews = bundle.reviews || {};

                sessionQueue = buildQueue(cards, reviews, mode);
                sessionTotal = sessionQueue.length;

                spinnerEl.classList.add("hidden");

                if (!sessionQueue.length) {
                    var emptyMsg = mode === "due"
                        ? "Nenhuma carta pendente para hoje. Volte amanhã!"
                        : mode === "wrong"
                            ? "Nenhuma carta com erro recente. Continue assim!"
                            : "Este deck não tem cartas.";
                    showDone("Tudo em dia!", emptyMsg, false);
                    return;
                }

                updateProgressBar();
                loadNext();
            })
            .catch(function () {
                if (!isOffline && window.offlineStudy) {
                    // Network failed — check IDB for cached data.
                    offlineStudy.isDeckCached(deckId)
                        .then(function (cached) {
                            if (cached) {
                                setOfflineBanner(true);
                                initSession(true);
                            } else {
                                spinnerEl.classList.add("hidden");
                                showDone(
                                    "Sem conexão",
                                    "Você está offline e este deck ainda não foi sincronizado. " +
                                    "Conecte-se à internet e abra o deck uma vez para ativar o modo offline.",
                                    true
                                );
                            }
                        })
                        .catch(function () {
                            spinnerEl.classList.add("hidden");
                            showDone("Erro", "Não foi possível carregar as cartas.", true);
                        });
                } else {
                    spinnerEl.classList.add("hidden");
                    showDone("Erro", "Não foi possível carregar as cartas.", true);
                }
            });
    }

    /* ════════════════════════════════════════════════════════════════════════
       DECK CONTEXT LABEL
    ════════════════════════════════════════════════════════════════════════ */
    var MODE_LABELS = { due: "Revisão do dia", random: "Aleatório", wrong: "Errei recentemente" };

    function renderDeckContext() {
        if (!deckCtxEl) return;
        var label = MODE_LABELS[mode] || mode;
        // Subject tag (shown before deck name when available)
        var subjectHtml = deckSubject
            ? '<span class="deck-ctx__subject">' + app.esc(deckSubject) + '</span>' +
              '<span class="deck-ctx__sep" aria-hidden="true">›</span>'
            : '';
        var nameHtml = deckName
            ? '<span class="deck-ctx__name">' + app.esc(deckName) + '</span>' +
              '<span class="deck-ctx__sep" aria-hidden="true">·</span>'
            : '';
        deckCtxEl.innerHTML =
            '<svg class="deck-ctx__icon" width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
            '<path d="M12 22V12" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>' +
            '<path d="M12 13C13 10 16 8 19 8C19 11 16.5 13.5 12 13Z" fill="currentColor"/>' +
            '<path d="M12 17C11 14 8 12 5 12C5 15 7.5 17.5 12 17Z" fill="currentColor" opacity="0.6"/>' +
            '</svg>' +
            subjectHtml +
            nameHtml +
            '<span class="deck-ctx__mode">' + app.esc(label) + '</span>';
    }


    /* ════════════════════════════════════════════════════════════════════════
       PROGRESS  —  driven by sessionTotal set once in initSession()
    ════════════════════════════════════════════════════════════════════════ */

    /* Sets the topbar centre to "Carta X de Y" — the card the student is NOW reading. */
    function updateCardPosition() {
        var total = sessionTotal;
        if (!total) {
            counterEl.classList.add("hidden");
            return;
        }
        counterEl.textContent = "Carta " + (sessionCount + 1) + " de " + total;
        counterEl.classList.remove("hidden");
    }

    /* Updates the thin progress bar (answered count / total). */
    function updateProgressBar() {
        var total = sessionTotal;
        if (!total) {
            progressFill.style.width = "0%";
            progressLbl.textContent  = "";
            progressTrack.setAttribute("aria-valuenow", 0);
            return;
        }
        var pct = Math.min(100, Math.round((sessionCount / total) * 100));
        progressFill.style.width = pct + "%";
        progressTrack.setAttribute("aria-valuenow", pct);
        progressLbl.textContent  = sessionCount + " respondida" + (sessionCount !== 1 ? "s" : "");
    }

    /* Called after each answer to advance both displays. */
    function updateProgress() {
        updateProgressBar();
        // topbar counter is refreshed in renderCard() when the next card appears
    }

    /* ════════════════════════════════════════════════════════════════════════
       LOAD NEXT CARD  —  reads from the in-memory session queue; no network
    ════════════════════════════════════════════════════════════════════════ */
    function loadNext() {
        // Reset interaction state for the incoming card.
        flipped    = false;
        submitting = false;
        isDragging = false;
        dragX = 0; dragY = 0;

        flashcardEl.classList.remove("is-flipped");
        cardDragEl.style.transform  = "";
        cardDragEl.style.transition = "";
        setAnswerButtons(false);
        hideSwipeLabels();

        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");
        doneEl.classList.add("hidden");
        spinnerEl.classList.add("hidden");
        if (answerHelp) {
            answerHelp.classList.add("hidden");
            if (btnHelpToggle) btnHelpToggle.setAttribute("aria-expanded", "false");
        }

        currentCard = sessionQueue.shift();

        if (!currentCard) {
            // Queue exhausted — session complete.
            if (mode === "random") {
                showRoundComplete();
            } else if (mode === "due") {
                showDone("Parabéns!", "Você revisou todas as cartas de hoje! Volte amanhã para continuar.", false);
            } else {
                showDone("Parabéns!", "Você revisou todas as cartas com erro recente!", false);
            }
            return;
        }

        renderCard();
    }

    /* ════════════════════════════════════════════════════════════════════════
       RENDER CARD  —  sets type theme + content on both faces
    ════════════════════════════════════════════════════════════════════════ */
    function renderCard() {
        updateCardPosition();   // show "Carta X de Y" in topbar for this card
        updateProgressBar();    // update the thin bar (answered / total)

        var type = (currentCard.type || "").toLowerCase();

        // Apply type theme to card scene so CSS cascade can color both faces
        cardSceneEl.setAttribute("data-type", type);

        // Badge HTML: only card type — topic removed to keep the study view clean
        var badgeHTML =
            '<span class="badge badge-type" data-type="' + app.esc(type) + '">' +
            app.esc(currentCard.type) + '</span>';

        // Front face
        headFront.innerHTML = badgeHTML;
        bodyFront.innerHTML = renderCardText(currentCard.question);
        bodyFront.scrollTop = 0;   // reset scroll so new question starts at the top

        // Back face
        headBack.innerHTML = badgeHTML;
        var backHTML = renderCardText(currentCard.answer);
        if (currentCard.source) {
            backHTML += '<p class="card-source">\uD83D\uDCDA ' + app.esc(currentCard.source) + '</p>';
        }
        bodyBack.innerHTML = backHTML;
        bodyBack.scrollTop = 0;    // reset scroll so answer also starts at the top

        // Reveal card with enter animation
        spinnerEl.classList.add("hidden");
        cardAreaEl.classList.remove("hidden");
        answerBar.classList.remove("hidden");

        flashcardEl.classList.add("card-enter");
        flashcardEl.addEventListener("animationend", function () {
            flashcardEl.classList.remove("card-enter");
        }, { once: true });
    }

    /* ════════════════════════════════════════════════════════════════════════
       DONE / ERROR STATE
    ════════════════════════════════════════════════════════════════════════ */
    // Called when random mode exhausts all cards in the deck (one full round).
    // Rebuilds a freshly shuffled queue and lets the user start the next round.
    function showRoundComplete() {
        spinnerEl.classList.add("hidden");
        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");
        if (answerHelp) answerHelp.classList.add("hidden");

        doneEl.innerHTML =
            '<div style="text-align:center;padding:2rem 1rem">' +
            '<svg width="44" height="44" viewBox="0 0 24 24" fill="none" style="color:var(--green-800);margin-bottom:.75rem">' +
            '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
            '<path d="M22 4L12 14.01l-3-3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
            '</svg>' +
            '<h2 style="font-size:1.2rem;font-weight:700;margin:0 0 .4rem">Rodada concluída!</h2>' +
            '<p style="color:var(--text-secondary);margin:0 0 1.5rem;font-size:.95rem">' +
            'Você viu todas as cartas do deck. Começando nova rodada\u2026' +
            '</p>' +
            '<button id="btn-next-round" class="btn btn-primary">Próxima rodada</button>' +
            '</div>';
        doneEl.classList.remove("hidden");

        var btnNextRound = document.getElementById("btn-next-round");
        if (btnNextRound) {
            btnNextRound.addEventListener("click", function () {
                // Rebuild a fresh shuffled queue for the new round using the
                // same bundle — no network call needed.
                if (sessionBundle) {
                    sessionQueue = buildQueue(
                        sessionBundle.cards   || [],
                        sessionBundle.reviews || {},
                        "random"
                    );
                    sessionTotal  = sessionQueue.length;
                    sessionCount  = 0;
                    sessionCorrect = 0;
                    sessionHard   = 0;
                    sessionWrong  = 0;
                    updateProgressBar();
                }
                doneEl.classList.add("hidden");
                loadNext();
            });
        }
    }

    function showDone(title, message, isError) {
        spinnerEl.classList.add("hidden");
        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");
        if (answerHelp) answerHelp.classList.add("hidden");

        var iconSVG = isError
            ? '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" style="color:#E53935"><path d="M12 9v4M12 17h.01M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>'
            : '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" style="color:var(--green-800)"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M22 4L12 14.01l-3-3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

        // Session summary (only when at least 1 card was answered)
        var summaryHTML = '';
        if (!isError && sessionCount > 0) {
            var accuracy = Math.round(sessionCorrect * 100 / sessionCount);
            var fillClass = accuracy >= 80 ? 'done-accuracy__fill--ok'
                          : accuracy >= 50 ? 'done-accuracy__fill--mid'
                          :                  'done-accuracy__fill--low';
            var pctColor  = accuracy >= 80 ? 'var(--green-800)'
                          : accuracy >= 50 ? '#e65100'
                          :                  '#b71c1c';
            var motivMsg  = accuracy >= 80 ? 'Excelente! Continue assim.'
                          : accuracy >= 50 ? 'Bom trabalho. Foque nas cartas erradas.'
                          :                  'Revise os fundamentos. Você vai melhorar!';
            summaryHTML =
                '<div class="done-summary">' +
                  '<div class="done-stat-grid">' +
                    '<div class="done-stat">' +
                      '<span class="done-stat__val">' + sessionCount + '</span>' +
                      '<span class="done-stat__lbl">revisadas</span>' +
                    '</div>' +
                    '<div class="done-stat done-stat--ok">' +
                      '<span class="done-stat__val">' + sessionCorrect + '</span>' +
                      '<span class="done-stat__lbl">acertei</span>' +
                    '</div>' +
                    '<div class="done-stat done-stat--hard">' +
                      '<span class="done-stat__val">' + sessionHard + '</span>' +
                      '<span class="done-stat__lbl">dif\u00edcil</span>' +
                    '</div>' +
                    '<div class="done-stat done-stat--wrong">' +
                      '<span class="done-stat__val">' + sessionWrong + '</span>' +
                      '<span class="done-stat__lbl">errei</span>' +
                    '</div>' +
                  '</div>' +
                  '<div class="done-accuracy">' +
                    '<div class="done-accuracy__bar">' +
                      '<div class="done-accuracy__fill ' + fillClass + '" style="width:' + accuracy + '%"></div>' +
                    '</div>' +
                    '<span class="done-accuracy__pct" style="color:' + pctColor + '">' + accuracy + '%</span>' +
                  '</div>' +
                  '<p class="done-motiv">' + app.esc(motivMsg) + '</p>' +
                '</div>';
        }

        doneEl.innerHTML =
            '<div class="done-icon">' + iconSVG + '</div>' +
            '<h2>' + app.esc(title) + '</h2>' +
            '<p>' + app.esc(message) + '</p>' +
            summaryHTML +
            '<div class="done-actions">' +
                '<a href="/" class="btn btn-primary">Ver decks</a>' +
                (isError ? '' :
                    (mode === "random"
                        ? '<a href="/study.html?deckId=' + encodeURIComponent(deckId) + '&mode=random' +
                          (deckName    ? '&deckName='    + encodeURIComponent(deckName)    : '') +
                          (deckSubject ? '&deckSubject=' + encodeURIComponent(deckSubject) : '') +
                          '" class="btn btn-outline">Revisar de novo</a>'
                        : '<a href="/study.html?deckId=' + encodeURIComponent(deckId) + '&mode=random' +
                          (deckName    ? '&deckName='    + encodeURIComponent(deckName)    : '') +
                          (deckSubject ? '&deckSubject=' + encodeURIComponent(deckSubject) : '') +
                          '" class="btn btn-outline">Modo aleat\u00f3rio</a>')
                ) +
            '</div>';
        doneEl.classList.remove("hidden");
    }

    /* ── Helpers ────────────────────────────────────────────────────────────── */
    function setAnswerButtons(enabled) {
        btnWrong.disabled = !enabled;
        btnHard.disabled  = !enabled;
        btnRight.disabled = !enabled;
    }

    init();
})();
