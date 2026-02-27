(function () {
    "use strict";

    var loginEl   = document.getElementById("login-prompt");
    var contentEl = document.getElementById("home-content");
    var gridEl    = document.getElementById("deck-grid");
    var emptyEl   = document.getElementById("empty-state");
    var spinnerEl = document.getElementById("spinner");
    var searchEl  = document.getElementById("deck-search");

    var allDecks = [];

    function showAuthError() {
        var params = new URLSearchParams(window.location.search);
        var err = params.get("error");
        if (!err) return;
        var msgs = {
            exchange_failed: "Falha ao trocar o código OAuth com o Google.",
            profile_failed:  "Não foi possível obter o perfil do Google.",
            login_failed:    "Erro interno ao criar sessão. Verifique se as migrações do banco foram aplicadas (make migrate-up).",
            invalid_state:   "Estado OAuth inválido. Tente novamente.",
            oauth_denied:    "Acesso negado pelo Google."
        };
        var msg = msgs[err] || ("Erro de autenticação: " + err);
        var el = document.createElement("div");
        el.style.cssText = "background:#fdecea;color:#b71c1c;border:1px solid #ef9a9a;border-radius:8px;padding:.75rem 1rem;margin:1rem 0;font-size:.9rem";
        el.textContent = msg;
        var wrap = document.querySelector(".wrap");
        if (wrap) wrap.insertBefore(el, wrap.firstChild);
        // Clean up URL without reload
        window.history.replaceState(null, "", "/");
    }

    function init() {
        showAuthError();
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) {
                app.renderTopbar(null);
                loginEl.classList.remove("hidden");
                return;
            }
            app.renderTopbar(user);
            contentEl.classList.remove("hidden");
            loadDecks();
        });
    }

    function loadDecks() {
        api.get("/api/decks").then(function (res) {
            if (!res.ok) throw new Error("failed");
            return res.json();
        }).then(function (page) {
            var decks = (page && page.items) ? page.items : (page || []);
            allDecks = decks || [];
            if (allDecks.length === 0) {
                emptyEl.classList.remove("hidden");
                return;
            }
            renderDecks(allDecks);
        }).catch(function () {
            gridEl.innerHTML = '<p style="color:var(--danger);padding:1rem">Erro ao carregar decks.</p>';
        });
    }

    if (searchEl) {
        searchEl.addEventListener("input", debounce(function () {
            var q = searchEl.value.toLowerCase().trim();
            var filtered = q
                ? allDecks.filter(function (d) {
                    return d.name.toLowerCase().indexOf(q) >= 0 ||
                           (d.description && d.description.toLowerCase().indexOf(q) >= 0);
                  })
                : allDecks;
            if (filtered.length === 0) {
                gridEl.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Nenhum deck encontrado.</p>';
            } else {
                renderDecks(filtered);
            }
        }, 200));
    }

    function debounce(fn, ms) {
        var t;
        return function () { clearTimeout(t); t = setTimeout(fn, ms); };
    }

    function renderDecks(decks) {
        var html = "";
        for (var i = 0; i < decks.length; i++) {
            html += renderDeckCard(decks[i]);
        }
        gridEl.innerHTML = html;
    }

    /* ── Date helpers ───────────────────────────────────────────────────────── */
    function daysFromNow(isoStr) {
        if (!isoStr) return null;
        var now   = new Date();
        var other = new Date(isoStr);
        var diff  = Math.round((other - now) / (1000 * 60 * 60 * 24));
        return diff;
    }

    function relativeDate(isoStr) {
        var d = daysFromNow(isoStr);
        if (d === null) return null;
        if (d === 0)  return "hoje";
        if (d === 1)  return "amanhã";
        if (d === -1) return "ontem";
        if (d > 1)    return "em " + d + " dias";
        return "há " + Math.abs(d) + " dia" + (Math.abs(d) !== 1 ? "s" : "");
    }

    function renderDeckCard(d) {
        var desc = d.description ? '<p class="deck-desc">' + app.esc(d.description) + '</p>' : '';
        var dueLabel   = d.due_now === 1 ? "1 para hoje" : d.due_now + " para hoje";
        var totalLabel = d.total_cards === 1 ? "1 carta" : d.total_cards + " cartas";

        /* ── Review date info ── */
        var dateInfo = "";
        if (d.last_studied) {
            var ago = relativeDate(d.last_studied);
            dateInfo += '<span class="deck-date">' +
                '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M12 8v4l3 3m6-3a9 9 0 1 1-18 0 9 9 0 0 1 18 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>' +
                'Estudado ' + app.esc(ago) + '</span>';
        } else {
            dateInfo += '<span class="deck-date deck-date--new">' +
                '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>' +
                'Nunca estudado</span>';
        }
        if (d.due_now === 0 && d.next_review) {
            var nxt = relativeDate(d.next_review);
            dateInfo += '<span class="deck-date deck-date--next">' +
                '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M5 12h14M12 5l7 7-7 7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                'Próxima revisão ' + app.esc(nxt) + '</span>';
        }

        return '<div class="card deck-card">' +
            '<h3>' + app.esc(d.name) + '</h3>' +
            desc +
            '<div class="deck-meta">' +
                (d.due_now > 0
                    ? '<span class="badge badge-due">' + dueLabel + '</span>'
                    : '<span class="badge badge-ok">Em dia ✓</span>') +
                '<span class="badge badge-total">' + totalLabel + '</span>' +
            '</div>' +
            '<div class="deck-dates">' + dateInfo + '</div>' +
            '<div class="deck-actions">' +
                '<a href="/study.html?deckId=' + d.id + '&mode=due&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-primary"' +
                    (d.due_now === 0 ? ' disabled aria-disabled="true" tabindex="-1"' : '') +
                    '>Revisar</a>' +
                '<a href="/study.html?deckId=' + d.id + '&mode=random&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Aleat\u00f3rio</a>' +
                '<a href="/study.html?deckId=' + d.id + '&mode=wrong&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Errei</a>' +
            '</div>' +
        '</div>';
    }

    init();
})();
