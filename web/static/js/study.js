(function () {
    "use strict";

    var params    = new URLSearchParams(window.location.search);
    var deckId    = params.get("deckId");
    var mode      = params.get("mode") || "due";
    var deckName  = params.get("deckName") ? decodeURIComponent(params.get("deckName")) : null;

    /* ── Stable DOM refs (never replaced) ─────────────────────────────────── */
    var counterEl    = document.getElementById("study-counter");
    var deckCtxEl    = document.getElementById("deck-context");
    var topicFilterEl = document.getElementById("topic-filter");
    var spinnerEl    = document.getElementById("study-spinner");
    var doneEl       = document.getElementById("study-done");
    var cardAreaEl   = document.getElementById("card-area");
    var cardDragEl   = document.getElementById("card-drag");
    var cardSceneEl  = document.getElementById("card-scene");
    var flashcardEl  = document.getElementById("flashcard");
    var headFront    = document.getElementById("head-front");
    var bodyFront    = document.getElementById("body-front");
    var headBack     = document.getElementById("head-back");
    var bodyBack     = document.getElementById("body-back");
    var answerBar    = document.getElementById("answer-bar");
    var btnWrong     = document.getElementById("btn-wrong");
    var btnHard      = document.getElementById("btn-hard");
    var btnRight     = document.getElementById("btn-right");
    var lblWrong     = document.getElementById("lbl-wrong");
    var lblRight     = document.getElementById("lbl-right");
    var lblHard      = document.getElementById("lbl-hard");
    var progressFill = document.getElementById("progress-fill");
    var progressLbl  = document.getElementById("progress-label");
    var progressTrack= document.getElementById("progress-track");

    /* ── State ─────────────────────────────────────────────────────────────── */
    var currentCard  = null;
    var flipped      = false;
    var submitting   = false;
    var remaining    = null;   // due_now for "due" mode
    var totalCards   = null;   // total cards in deck (for progress bar + session cap)
    var sessionCount = 0;      // cards answered in this session
    var activeTopic  = "";     // "" = all topics

    /* ── Swipe / drag state ─────────────────────────────────────────────────  */
    var SWIPE_THRESHOLD = 72;
    var LABEL_THRESHOLD = 36;
    var isDragging = false;
    var dragLocked = false;
    var startX = 0, startY = 0;
    var dragX  = 0, dragY  = 0;

    /* ════════════════════════════════════════════════════════════════════════
       INIT
    ════════════════════════════════════════════════════════════════════════ */
    function init() {
        if (!deckId) {
            showDone("Deck não encontrado", "Volte e escolha um deck.", true);
            return;
        }
        app.checkAuth().then(function (user) {
            if (!user) { window.location.href = "/"; return; }
            app.renderTopbar(user, { backHref: "/", noNav: true });
            renderDeckContext();
            loadTopics();
            loadStats();
            loadNext();
        });

        setupFlip();
        setupSwipe();
        setupButtons();
        setupKeyboard();
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

        Promise.all([flyDone, apiDone]).then(function (results) {
            var res = results[1];
            if (!res.ok) throw new Error("answer failed");

            sessionCount++;

            // Update "due" remaining counter
            if (mode === "due" && remaining !== null) {
                remaining = Math.max(0, remaining - 1);
            }

            // Check session cap for non-due modes
            if (mode !== "due" && totalCards !== null && sessionCount >= totalCards) {
                updateProgress();
                showDone("Sessão concluída!", "Você revisou " + sessionCount + " carta" + (sessionCount !== 1 ? "s" : "") + " desta vez.", false);
                return;
            }

            updateProgress();
            loadNext();
        }).catch(function () {
            submitting = false;
            cardDragEl.style.transition = "transform .35s cubic-bezier(.34,1.56,.64,1)";
            cardDragEl.style.transform  = "";
            setTimeout(function () {
                cardDragEl.style.transition = "";
                setAnswerButtons(true);
            }, 360);
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
       DECK CONTEXT LABEL
    ════════════════════════════════════════════════════════════════════════ */
    var MODE_LABELS = { due: "Revisão do dia", random: "Aleatório", wrong: "Errei recentemente" };

    function renderDeckContext() {
        if (!deckCtxEl) return;
        var label = MODE_LABELS[mode] || mode;
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
            nameHtml +
            '<span class="deck-ctx__mode">' + app.esc(label) + '</span>';
    }

    /* ════════════════════════════════════════════════════════════════════════
       TOPIC FILTER
    ════════════════════════════════════════════════════════════════════════ */
    function loadTopics() {
        if (!topicFilterEl) return;
        api.get("/api/study/topics?deckId=" + deckId)
            .then(function (res) { return res.ok ? res.json() : null; })
            .then(function (data) {
                var topics = (data && data.topics) ? data.topics : [];
                if (topics.length < 2) return; // not worth showing if ≤ 1 topic
                renderTopicFilter(topics);
            })
            .catch(function () { /* non-critical */ });
    }

    function renderTopicFilter(topics) {
        var html = '<button class="topic-chip active" data-topic="">Todos</button>';
        for (var i = 0; i < topics.length; i++) {
            html += '<button class="topic-chip" data-topic="' + app.esc(topics[i]) + '">' +
                app.esc(topics[i]) + '</button>';
        }
        topicFilterEl.innerHTML = html;
        topicFilterEl.classList.remove("hidden");

        topicFilterEl.addEventListener("click", function (e) {
            var btn = e.target.closest(".topic-chip");
            if (!btn) return;
            var chips = topicFilterEl.querySelectorAll(".topic-chip");
            for (var i = 0; i < chips.length; i++) chips[i].classList.remove("active");
            btn.classList.add("active");
            activeTopic = btn.getAttribute("data-topic") || "";
            // Reset session for new topic
            sessionCount = 0;
            currentCard = null;
            flipped = false;
            flashcardEl.classList.remove("is-flipped");
            setAnswerButtons(false);
            cardAreaEl.classList.add("hidden");
            doneEl.classList.add("hidden");
            spinnerEl.classList.remove("hidden");
            loadStats();
            loadNext();
        });
    }

    /* ════════════════════════════════════════════════════════════════════════
       STATS + PROGRESS
    ════════════════════════════════════════════════════════════════════════ */
    function loadStats() {
        api.get("/api/stats?deckId=" + deckId).then(function (res) {
            if (!res.ok) return;
            return res.json();
        }).then(function (stats) {
            if (!stats) return;
            remaining  = stats.due_now;
            totalCards = stats.total_cards || null;
            // Refresh display if a card is already showing
            if (currentCard) updateCardPosition();
            updateProgressBar();
        });
    }

    /* Sets the topbar centre to "Carta X de Y" — the card the student is NOW reading. */
    function updateCardPosition() {
        var total = resolveTotal();
        if (total === null || total === 0) {
            counterEl.classList.add("hidden");
            return;
        }
        var current = sessionCount + 1;           // 1-based position of current card
        counterEl.textContent = "Carta " + current + " de " + total;
        counterEl.classList.remove("hidden");
    }

    /* Updates only the thin progress bar (answered count / total). */
    function updateProgressBar() {
        var total = resolveTotal();
        if (total === null || total === 0) {
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

    /* Returns the session cap depending on mode. */
    function resolveTotal() {
        if (mode === "due") {
            return (remaining !== null && totalCards !== null)
                ? remaining + sessionCount   // initial due count
                : null;
        }
        return totalCards;
    }

    /* Called after each answer to advance both displays. */
    function updateProgress() {
        updateProgressBar();
        // topbar is refreshed in renderCard() when the next card appears
    }

    /* ════════════════════════════════════════════════════════════════════════
       LOAD NEXT CARD
    ════════════════════════════════════════════════════════════════════════ */
    function loadNext() {
        flipped     = false;
        submitting  = false;
        currentCard = null;
        isDragging  = false;
        dragX = 0; dragY = 0;

        flashcardEl.classList.remove("is-flipped");
        cardDragEl.style.transform  = "";
        cardDragEl.style.transition = "";
        setAnswerButtons(false);
        hideSwipeLabels();

        spinnerEl.classList.remove("hidden");
        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");
        doneEl.classList.add("hidden");

        var nextUrl = "/api/study/next?deckId=" + deckId + "&mode=" + mode;
        if (activeTopic) nextUrl += "&topic=" + encodeURIComponent(activeTopic);
        api.get(nextUrl)
            .then(function (res) {
                if (res.status === 204) {
                    var msg = mode === "due"
                        ? "Você revisou todas as cartas de hoje! Volte amanhã para continuar."
                        : "Nenhuma carta disponível neste modo.";
                    showDone("Parabéns!", msg, false);
                    return null;
                }
                if (!res.ok) throw new Error("load failed");
                return res.json();
            })
            .then(function (card) {
                if (!card) return;
                currentCard = card;
                renderCard();
            })
            .catch(function () {
                showDone("Erro", "Não foi possível carregar a carta. Tente novamente.", true);
            });
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

        // Badge HTML (shown on front header + back header)
        var badgeHTML =
            '<span class="badge badge-type" data-type="' + app.esc(type) + '">' +
            app.esc(currentCard.type) + '</span>';
        if (currentCard.topic) {
            badgeHTML += '<span class="badge badge-topic">' + app.esc(currentCard.topic) + '</span>';
        }

        // Front face
        headFront.innerHTML = badgeHTML;
        bodyFront.innerHTML = '<p>' + app.esc(currentCard.question) + '</p>';

        // Back face
        headBack.innerHTML = badgeHTML;
        var backHTML = '<p>' + app.esc(currentCard.answer) + '</p>';
        if (currentCard.source) {
            backHTML += '<p class="card-source">\uD83D\uDCDA ' + app.esc(currentCard.source) + '</p>';
        }
        bodyBack.innerHTML = backHTML;

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
    function showDone(title, message, isError) {
        spinnerEl.classList.add("hidden");
        cardAreaEl.classList.add("hidden");
        answerBar.classList.add("hidden");

        var iconSVG = isError
            ? '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" style="color:#E53935"><path d="M12 9v4M12 17h.01M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>'
            : '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" style="color:var(--green-800)"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M22 4L12 14.01l-3-3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

        doneEl.innerHTML =
            '<div class="done-icon">' + iconSVG + '</div>' +
            '<h2>' + app.esc(title) + '</h2>' +
            '<p>' + app.esc(message) + '</p>' +
            '<div class="done-actions">' +
                '<a href="/" class="btn btn-primary">Ver decks</a>' +
                (isError ? '' :
                    (mode === "random"
                        ? '<a href="/study.html?deckId=' + encodeURIComponent(deckId) + '&mode=random" class="btn btn-outline">Revisar de novo</a>'
                        : '<a href="/study.html?deckId=' + encodeURIComponent(deckId) + '&mode=random" class="btn btn-outline">Modo aleat\u00f3rio</a>')
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
