(function () {
    "use strict";

    var loginEl   = document.getElementById("login-prompt");
    var contentEl = document.getElementById("home-content");
    var gridEl    = document.getElementById("deck-grid");
    var emptyEl   = document.getElementById("empty-state");
    var spinnerEl = document.getElementById("spinner");
    var searchEl  = document.getElementById("deck-search");

    var allDecks    = [];
    var hiddenEl    = document.getElementById("hidden-decks-section");
    var hiddenGrid  = document.getElementById("hidden-deck-grid");
    var hiddenCount = document.getElementById("hidden-deck-count");
    var hiddenToggle = document.getElementById("btn-toggle-hidden");
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
            isProfessor = app.effectiveIsStaff(roles);
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
            decks = decks || [];
            // Split visible vs hidden decks
            allDecks = decks.filter(function (d) { return !d.hidden; });
            var hiddenDecks = decks.filter(function (d) { return d.hidden; });
            if (allDecks.length === 0 && hiddenDecks.length === 0) {
                emptyEl.classList.remove("hidden");
                return;
            }
            if (allDecks.length === 0) {
                emptyEl.classList.remove("hidden");
            }
            renderDecks(allDecks);
            renderHiddenSection(hiddenDecks);
        }).catch(function () {
            gridEl.innerHTML = '<p style="color:var(--danger);padding:1rem">Erro ao carregar decks.</p>';
        });
    }

    function renderHiddenSection(hiddenDecks) {
        if (!hiddenEl || !hiddenGrid || isProfessor) return;
        if (hiddenDecks.length === 0) {
            hiddenEl.classList.add("hidden");
            return;
        }
        if (hiddenCount) hiddenCount.textContent = hiddenDecks.length;
        hiddenGrid.innerHTML = hiddenDecks.map(function (d) {
            return '<div class="card deck-card deck-card--hidden">' +
                '<h3 style="font-size:.95rem;margin:0 0 .4rem">' + app.esc(d.name) + '</h3>' +
                (d.subject ? '<div class="text-muted" style="font-size:.8rem;margin-bottom:.5rem">' + app.esc(d.subject) + '</div>' : '') +
                '<button class="btn btn-sm btn-outline btn-unhide" data-deck-id="' + d.id + '" data-deck-name="' + app.esc(d.name) + '">' +
                    '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M1 12S5 4 12 4s11 8 11 8-4 8-11 8S1 12 1 12z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '<circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2"/>' +
                    '</svg> Exibir' +
                '</button>' +
            '</div>';
        }).join('');

        hiddenGrid.querySelectorAll('.btn-unhide').forEach(function (btn) {
            btn.addEventListener('click', function () {
                btn.disabled = true;
                api.post('/api/me/deck-hidden', { deck_id: btn.dataset.deckId, hidden: false })
                    .then(function (res) {
                        if (!res.ok) throw new Error();
                        loadDecks(); // reload to reflect change
                    })
                    .catch(function () { btn.disabled = false; });
            });
        });

        hiddenEl.classList.remove("hidden");
    }

    if (hiddenToggle) {
        hiddenToggle.addEventListener('click', function () {
            if (!hiddenGrid) return;
            var isHidden = hiddenGrid.classList.toggle("hidden");
            hiddenToggle.textContent = isHidden ? 'Mostrar' : 'Ocultar';
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
    function attachHideListeners(container) {
        container.querySelectorAll(".btn-deck-hide").forEach(function (btn) {
            btn.addEventListener("click", function (e) {
                e.stopPropagation();
                e.preventDefault();
                btn.disabled = true;
                var deckId = btn.dataset.deckId;
                api.post('/api/me/deck-hidden', { deck_id: deckId, hidden: true })
                    .then(function (res) {
                        if (!res.ok) throw new Error();
                        loadDecks();
                    })
                    .catch(function () { btn.disabled = false; });
            });
        });
    }

    function renderDecks(decks, flat) {
        if (flat) {
            gridEl.innerHTML = decks.map(renderDeckCard).join("");
            attachHideListeners(gridEl);
            return;
        }

        // Separate private decks, class decks, and general decks
        var privateDecks = [];
        var classGroups  = {};   // class_name -> [deck]
        var classOrder   = [];   // ordered class names
        var groups       = {};   // subject -> [deck]
        var order        = [];   // ordered subject keys
        var noSubject    = [];

        decks.forEach(function (d) {
            if (d.is_private) {
                privateDecks.push(d);
            } else if (d.class_name) {
                if (!classGroups[d.class_name]) { classGroups[d.class_name] = []; classOrder.push(d.class_name); }
                classGroups[d.class_name].push(d);
            } else if (d.subject) {
                if (!groups[d.subject]) { groups[d.subject] = []; order.push(d.subject); }
                groups[d.subject].push(d);
            } else {
                noSubject.push(d);
            }
        });

        order.sort(function (a, b) { return a.localeCompare(b, "pt"); });
        classOrder.sort(function (a, b) { return a.localeCompare(b, "pt"); });

        var chevronSvg =
            '<svg class="subject-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
            '<path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>' +
            '</svg>';

        var html = "";

        // Private decks section
        if (privateDecks.length > 0) {
            var key0      = "sg_collapsed__private";
            var coll0     = sessionStorage.getItem(key0) === "1";
            var sgCls0    = coll0 ? " subject-group--collapsed" : "";
            var expanded0 = coll0 ? "false" : "true";
            html += '<div class="subject-group' + sgCls0 + '" data-sg-key="' + key0 + '">' +
                '<button class="subject-group__header subject-group__header--private" aria-expanded="' + expanded0 + '">' +
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<rect x="3" y="11" width="18" height="11" rx="2" stroke="currentColor" stroke-width="2"/>' +
                    '<path d="M7 11V7a5 5 0 0 1 10 0v4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '</svg>' +
                    '<span>Meus Cards Pessoais</span>' +
                    chevronSvg +
                '</button>' +
                '<div class="deck-grid">' + privateDecks.map(renderDeckCard).join("") + '</div>' +
            '</div>';
        }

        // Class groups (turmas)
        classOrder.forEach(function (className) {
            var key1      = "sg_collapsed_class_" + className.replace(/\s+/g, "_");
            var coll1     = sessionStorage.getItem(key1) === "1";
            var sgCls1    = coll1 ? " subject-group--collapsed" : "";
            var expanded1 = coll1 ? "false" : "true";
            html += '<div class="subject-group' + sgCls1 + '" data-sg-key="' + app.esc(key1) + '">' +
                '<button class="subject-group__header subject-group__header--class" aria-expanded="' + expanded1 + '">' +
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2"/>' +
                    '<path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '</svg>' +
                    '<span>' + app.esc(className) + '</span>' +
                    chevronSvg +
                '</button>' +
                '<div class="deck-grid">' + classGroups[className].map(renderDeckCard).join("") + '</div>' +
            '</div>';
        });

        // Subject groups (general decks)
        // Each subject group is collapsible; state persisted in sessionStorage.
        order.forEach(function (subj) {
            var key       = "sg_collapsed_" + subj;
            var collapsed = sessionStorage.getItem(key) === "1";
            var sgCls     = collapsed ? " subject-group--collapsed" : "";
            var expanded  = collapsed ? "false" : "true";
            var gridId    = "sg-grid-" + app.esc(subj.replace(/\s+/g, "_"));
            html += '<div class="subject-group' + sgCls + '" data-sg-key="' + app.esc(key) + '">' +
                '<button class="subject-group__header" aria-expanded="' + expanded + '" aria-controls="' + app.esc(gridId) + '">' +
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                        '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                        '<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                    '</svg>' +
                    '<span>' + app.esc(subj) + '</span>' +
                    chevronSvg +
                '</button>' +
                '<div class="deck-grid" id="' + app.esc(gridId) + '">' +
                    groups[subj].map(renderDeckCard).join("") +
                '</div>' +
            '</div>';
        });

        if (noSubject.length > 0) {
            var label = order.length > 0 ? "Sem matéria" : "";
            if (label) {
                var key2      = "sg_collapsed__none";
                var collapsed2 = sessionStorage.getItem(key2) === "1";
                var sgCls2    = collapsed2 ? " subject-group--collapsed" : "";
                var expanded2 = collapsed2 ? "false" : "true";
                var gridId2   = "sg-grid--none";
                html += '<div class="subject-group' + sgCls2 + '" data-sg-key="' + key2 + '">' +
                    '<button class="subject-group__header subject-group__header--muted" aria-expanded="' + expanded2 + '" aria-controls="' + gridId2 + '">' +
                        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                            '<path d="M4 6h16M4 10h16M4 14h8" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                        '</svg>' +
                        '<span>' + label + '</span>' +
                        chevronSvg +
                    '</button>' +
                    '<div class="deck-grid" id="' + gridId2 + '">' +
                        noSubject.map(renderDeckCard).join("") +
                    '</div>' +
                '</div>';
            } else {
                // All ungrouped — plain grid (no subject headers)
                html += '<div class="deck-grid">' + noSubject.map(renderDeckCard).join("") + '</div>';
            }
        }

        gridEl.innerHTML = html;

        // Attach toggle listeners to all subject-group headers
        gridEl.querySelectorAll(".subject-group__header").forEach(function (btn) {
            btn.addEventListener("click", function () {
                var sg  = btn.closest(".subject-group");
                var key = sg.dataset.sgKey;
                var isCollapsed = sg.classList.toggle("subject-group--collapsed");
                btn.setAttribute("aria-expanded", isCollapsed ? "false" : "true");
                sessionStorage.setItem(key, isCollapsed ? "1" : "0");
            });
        });

        attachHideListeners(gridEl);
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
        var isDisabled = status !== "active";

        var dueLabel   = d.due_now === 1 ? "1 para hoje" : d.due_now + " para hoje";
        var totalLabel = d.total_cards === 1 ? "1 carta" : d.total_cards + " cartas";
        var studyBase  = "/study.html?deckId=" + d.id + "&deckName=" + encodeURIComponent(d.name) +
                         (d.subject ? "&deckSubject=" + encodeURIComponent(d.subject) : "");

        /* ── Deck type indicator ── */
        var typeChip = "";
        if (d.is_private) {
            typeChip = '<span class="deck-type-chip deck-type-chip--private">' +
                '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                '<rect x="3" y="11" width="18" height="11" rx="2" stroke="currentColor" stroke-width="2.2"/>' +
                '<path d="M7 11V7a5 5 0 0 1 10 0v4" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>' +
                '</svg>Pessoal</span>';
        } else if (d.class_name) {
            typeChip = '<span class="deck-type-chip deck-type-chip--class">' +
                '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>' +
                '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2.2"/>' +
                '</svg>' + app.esc(d.class_name) + '</span>';
        }

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
                dateInfo += '<span class="deck-date">' +
                    '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M12 8v4l3 3m6-3a9 9 0 1 1-18 0 9 9 0 0 1 18 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>' +
                    'Estudado ' + app.esc(relativeDate(d.last_studied)) + '</span>';
            } else {
                dateInfo += '<span class="deck-date deck-date--new">' +
                    '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>' +
                    'Nunca estudado</span>';
            }
            if (d.due_now === 0 && d.next_review) {
                dateInfo += '<span class="deck-date deck-date--next">' +
                    '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0"><path d="M5 12h14M12 5l7 7-7 7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                    'Próxima revisão ' + app.esc(relativeDate(d.next_review)) + '</span>';
            }
        }

        var cardStyle = isDisabled ? ' style="opacity:.6;border-style:dashed"' : '';

        /* ── Private deck: simpler card (no SM-2 buttons, go to my_deck.html) ── */
        if (d.is_private && !isDisabled) {
            return '<div class="card deck-card">' +
                '<div class="deck-card__type-row">' + typeChip + '</div>' +
                '<h3>' + app.esc(d.name) + '</h3>' +
                '<div class="deck-meta">' +
                    '<span class="badge badge-total">' + totalLabel + '</span>' +
                '</div>' +
                '<div class="deck-dates">' + dateInfo + '</div>' +
                '<div class="deck-actions">' +
                    '<a href="' + studyBase + '&mode=random" class="btn btn-sm btn-primary"' +
                        (d.total_cards === 0 ? ' disabled aria-disabled="true" tabindex="-1"' : '') + '>Estudar</a>' +
                    '<a href="/my_deck.html?deckId=' + d.id + '&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Cards</a>' +
                '</div>' +
            '</div>';
        }

        // Hide button: only for students, on general (non-private, non-class) decks
        var isGeneral = !d.is_private && !d.class_name;
        var hideBtn = (!isProfessor && isGeneral && !isDisabled)
            ? '<button class="btn-deck-hide" data-deck-id="' + d.id + '" title="Ocultar este deck da página inicial" aria-label="Ocultar deck">' +
              '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
              '<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
              '<line x1="1" y1="1" x2="23" y2="23" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
              '</svg>' +
              '</button>'
            : '';

        return '<div class="card deck-card" data-deck-id="' + d.id + '"' + cardStyle + '>' +
            inactiveBanner +
            (typeChip || hideBtn
                ? '<div class="deck-card__type-row" style="justify-content:space-between;align-items:center">' + (typeChip || '<span></span>') + hideBtn + '</div>'
                : '') +
            '<h3>' + app.esc(d.name) + '</h3>' +
            (d.description ? '<p class="deck-desc">' + app.esc(d.description) + '</p>' : '') +
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
                    ? '<a href="/deck_manage.html?deckId=' + d.id + '&deckName=' + encodeURIComponent(d.name) + '" class="btn btn-sm btn-outline">Gerenciar</a>'
                    : '<a href="' + studyBase + '&mode=due" class="btn btn-sm btn-primary"' +
                          (d.due_now === 0 ? ' disabled aria-disabled="true" tabindex="-1"' : '') + '>Revisar</a>' +
                      '<a href="' + studyBase + '&mode=random" class="btn btn-sm btn-outline">Aleatório</a>' +
                      '<a href="' + studyBase + '&mode=wrong" class="btn btn-sm btn-outline">Errei</a>'
                ) +
            '</div>' +
        '</div>';
    }

    init();
})();
