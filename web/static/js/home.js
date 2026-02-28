(function () {
    "use strict";

    var loginEl   = document.getElementById("login-prompt");
    var contentEl = document.getElementById("home-content");
    var gridEl    = document.getElementById("deck-grid");
    var emptyEl   = document.getElementById("empty-state");
    var spinnerEl = document.getElementById("spinner");
    var searchEl  = document.getElementById("deck-search");

    var allDecks = [];
    var isProfessor = false; // set after auth; used to show inactive badges

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
            var roles = user.roles || [];
            isProfessor = roles.indexOf("professor") >= 0 || roles.indexOf("admin") >= 0;
            app.renderTopbar(user);
            contentEl.classList.remove("hidden");
            loadDecks();
            loadStreak();
        });
    }

    function loadStreak() {
        api.get("/api/progress").then(function (res) {
            if (!res.ok) return;
            return res.json();
        }).then(function (d) {
            if (!d || !d.study_streak) return;
            var streak = d.study_streak;
            if (streak < 2) return;
            var banner = document.getElementById("streak-banner");
            if (!banner) return;
            var emoji = streak >= 30 ? "🌳" : streak >= 14 ? "🌿" : streak >= 7 ? "🌱" : "✨";
            banner.textContent = emoji + " " + streak + " dias consecutivos de estudo! Continue assim.";
            banner.classList.remove("hidden");
        }).catch(function () { /* silently ignore */ });
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
                           (d.description && d.description.toLowerCase().indexOf(q) >= 0) ||
                           (d.subject && d.subject.toLowerCase().indexOf(q) >= 0);
                  })
                : allDecks;
            if (filtered.length === 0) {
                gridEl.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Nenhum deck encontrado.</p>';
            } else {
                renderDecks(filtered, !!q);
            }
        }, 200));
    }

    function debounce(fn, ms) {
        var t;
        return function () { clearTimeout(t); t = setTimeout(fn, ms); };
    }

    // Group decks by subject; decks with no subject go to a special "Outros" group at the end.
    // When `flat` is true (search active) we skip grouping headers for a cleaner result view.
    function renderDecks(decks, flat) {
        if (flat) {
            gridEl.innerHTML = decks.map(renderDeckCard).join("");
            return;
        }

        // Build ordered groups: subjects alphabetically, then null-subject group last
        var groups   = {};   // subject -> [deck]
        var order    = [];   // ordered subject keys
        var noSubject = [];

        decks.forEach(function (d) {
            if (d.subject) {
                if (!groups[d.subject]) { groups[d.subject] = []; order.push(d.subject); }
                groups[d.subject].push(d);
            } else {
                noSubject.push(d);
            }
        });

        order.sort(function (a, b) { return a.localeCompare(b, "pt"); });

        var html = "";

        // If every deck has a subject, don't show the "no subject" fallback at all
        order.forEach(function (subj) {
            html += '<div class="subject-group">' +
                '<div class="subject-group__header">' +
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                        '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                        '<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                    '</svg>' +
                    '<span>' + app.esc(subj) + '</span>' +
                '</div>' +
                '<div class="deck-grid">' +
                    groups[subj].map(renderDeckCard).join("") +
                '</div>' +
            '</div>';
        });

        if (noSubject.length > 0) {
            var label = order.length > 0 ? "Sem matéria" : "";
            if (label) {
                html += '<div class="subject-group">' +
                    '<div class="subject-group__header subject-group__header--muted">' +
                        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                            '<path d="M4 6h16M4 10h16M4 14h8" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                        '</svg>' +
                        '<span>' + label + '</span>' +
                    '</div>' +
                    '<div class="deck-grid">' +
                        noSubject.map(renderDeckCard).join("") +
                    '</div>' +
                '</div>';
            } else {
                // All ungrouped — plain grid
                html += '<div class="deck-grid">' + noSubject.map(renderDeckCard).join("") + '</div>';
            }
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

    function deckStatus(d) {
        if (d.is_active === false) return "inactive";
        if (d.expires_at && new Date(d.expires_at) <= new Date()) return "expired";
        return "active";
    }

    function renderDeckCard(d) {
        var status     = deckStatus(d);
        var isDisabled = status !== "active"; // professor-only: deck hidden from students

        var desc = d.description ? '<p class="deck-desc">' + app.esc(d.description) + '</p>' : '';
        var dueLabel   = d.due_now === 1 ? "1 para hoje" : d.due_now + " para hoje";
        var totalLabel = d.total_cards === 1 ? "1 carta" : d.total_cards + " cartas";

        /* ── Inactive / expired banner (professors only) ── */
        var inactiveBanner = "";
        if (isProfessor && isDisabled) {
            var statusLabel = status === "expired" ? "Expirado" : "Inativo";
            var statusColor = status === "expired" ? "#B91C1C" : "#6B7280";
            inactiveBanner =
                '<div style="display:flex;align-items:center;gap:.4rem;font-size:.78rem;font-weight:600;' +
                'color:' + statusColor + ';background:' + (status === "expired" ? "#FEF2F2" : "#F3F4F6") + ';' +
                'border-radius:6px;padding:.35rem .65rem;margin-bottom:.6rem">' +
                '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                '<path d="M12 9v4M12 17h.01M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"' +
                ' stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                statusLabel + ' — não visível para alunos' +
                '</div>';
        }

        /* ── Review date info ── */
        var dateInfo = "";
        if (!isDisabled) {
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
        }

        var cardStyle = isDisabled ? ' style="opacity:.6;border-style:dashed"' : '';

        return '<div class="card deck-card"' + cardStyle + '>' +
            inactiveBanner +
            '<h3>' + app.esc(d.name) + '</h3>' +
            desc +
            (isDisabled ? '' :
                '<div class="deck-meta">' +
                    (d.due_now > 0
                        ? '<span class="badge badge-due">' + dueLabel + '</span>'
                        : '<span class="badge badge-ok">Em dia ✓</span>') +
                    '<span class="badge badge-total">' + totalLabel + '</span>' +
                '</div>' +
                '<div class="deck-dates">' + dateInfo + '</div>'
            ) +
            '<div class="deck-actions">' +
                (isDisabled
                    ? '<a href="/deck_manage.html?deckId=' + d.id + '&deckName=' + encodeURIComponent(d.name) +
                      '" class="btn btn-sm btn-outline">Gerenciar</a>'
                    : '<a href="/study.html?deckId=' + d.id + '&mode=due&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-primary"' +
                          (d.due_now === 0 ? ' disabled aria-disabled="true" tabindex="-1"' : '') +
                          '>Revisar</a>' +
                      '<a href="/study.html?deckId=' + d.id + '&mode=random&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Aleat\u00f3rio</a>' +
                      '<a href="/study.html?deckId=' + d.id + '&mode=wrong&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Errei</a>'
                ) +
            '</div>' +
        '</div>';
    }

    init();
})();
