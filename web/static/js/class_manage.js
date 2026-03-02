(function () {
    "use strict";

    var params   = new URLSearchParams(window.location.search);
    var classId  = params.get("classId");

    var spinnerEl    = document.getElementById("spinner");
    var deniedEl     = document.getElementById("access-denied");
    var pageContent  = document.getElementById("page-content");
    var classTitle   = document.getElementById("class-title");
    var classDescEl  = document.getElementById("class-desc");
    var statusBadge  = document.getElementById("class-status-badge");
    var btnToggle    = document.getElementById("btn-toggle-active");
    var btnDelete    = document.getElementById("btn-delete-class");
    var inviteCode   = document.getElementById("invite-code-display");
    var btnCopy      = document.getElementById("btn-copy-code");
    var btnRegen     = document.getElementById("btn-regen-code");
    var memberCount  = document.getElementById("member-count-text");

    var btnAddDeck        = document.getElementById("btn-add-deck");
    var addDeckPanel      = document.getElementById("add-deck-panel");
    var deckSelect        = document.getElementById("deck-select");
    var btnConfirmAddDeck = document.getElementById("btn-confirm-add-deck");
    var btnCancelAddDeck  = document.getElementById("btn-cancel-add-deck");
    var deckListEl        = document.getElementById("deck-list");

    // Stats refs
    var statsSpinner   = document.getElementById("stats-spinner");
    var statsContent   = document.getElementById("stats-content");
    var statsEmpty     = document.getElementById("stats-empty");
    var kpiMembers     = document.getElementById("kpi-members");
    var kpiActive      = document.getElementById("kpi-active");
    var kpiActive7     = document.getElementById("kpi-active7");
    var kpiReviews7    = document.getElementById("kpi-reviews7");
    var kpiAccuracy    = document.getElementById("kpi-accuracy");
    var kpiCards       = document.getElementById("kpi-cards");
    var deckStatsList  = document.getElementById("deck-stats-list");
    var hardCardsList  = document.getElementById("hard-cards-list");

    var currentClass   = null;
    var statsLoaded    = false;

    function init() {
        if (!classId) { window.location.href = "/classes.html"; return; }
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) { window.location.href = "/"; return; }
            var roles = user.roles || [];
            var isStaff = roles.indexOf("professor") >= 0 || roles.indexOf("admin") >= 0;
            if (!isStaff) {
                app.renderTopbar(user);
                if (deniedEl) deniedEl.classList.remove("hidden");
                return;
            }
            app.renderTopbar(user, { backHref: "/classes.html" });
            var back = document.getElementById("topbar-back");
            if (back) back.classList.remove("hidden");
            pageContent.classList.remove("hidden");
            loadClass();
            loadAllDecks();
            wireActions();
            wireTabs();
            // Support deep-link e.g. ?tab=stats from professor dashboard
            var defaultTab = params.get("tab");
            if (defaultTab) {
                var tabBtn = document.querySelector('.class-tab[data-tab="' + defaultTab + '"]');
                if (tabBtn) tabBtn.click();
            }
        });
    }

    function loadClass() {
        api.get("/api/classes/" + classId)
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (cl) {
                if (!cl) { window.location.href = "/classes.html"; return; }
                currentClass = cl;
                renderClassHeader(cl);
                loadClassDecks();
            });
    }

    function renderClassHeader(cl) {
        if (classTitle)  classTitle.textContent  = cl.name;
        if (classDescEl) {
            if (cl.description) {
                classDescEl.textContent = cl.description;
                classDescEl.classList.remove("hidden");
            }
        }
        if (inviteCode) inviteCode.textContent = cl.invite_code;
        updateStatusBadge(cl.is_active);
    }

    function updateStatusBadge(active) {
        if (!statusBadge) return;
        statusBadge.textContent = active ? "Ativa" : "Inativa";
        statusBadge.className = "badge " + (active ? "badge-green" : "badge-inactive");
        if (btnToggle) btnToggle.textContent = active ? "Desativar" : "Ativar";
    }

    function loadAllDecks() {
        api.get("/api/content/decks")
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                var items = data && data.items ? data.items : [];
                if (!deckSelect) return;
                while (deckSelect.options.length > 1) deckSelect.remove(1);
                items.forEach(function (d) {
                    var opt = document.createElement("option");
                    opt.value = d.id;
                    opt.textContent = d.name + (d.subject ? " (" + d.subject + ")" : "");
                    deckSelect.appendChild(opt);
                });
            });
    }

    function loadClassDecks() {
        api.get("/api/classes/" + classId + "/decks")
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                var items = data && data.items ? data.items : [];
                renderClassDecks(items);
                if (memberCount && currentClass) memberCount.textContent = (currentClass.member_count || 0) + " aluno(s) inscrito(s)";
            });
    }

    function renderClassDecks(decks) {
        if (!deckListEl) return;
        if (decks.length === 0) {
            deckListEl.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Nenhum deck associado. Clique em <strong>+ Adicionar deck</strong> para começar.</p>';
            return;
        }
        deckListEl.innerHTML = decks.map(function (d) {
            var subj = d.subject ? '<span class="badge" style="font-size:.7rem;background:#E8F5E9;color:#1B5E20;border:1px solid #A5D6A7">' + app.esc(d.subject) + '</span>' : '';
            return '<div class="class-deck-item" role="listitem">' +
                '<div style="display:flex;align-items:center;gap:.5rem;flex-wrap:wrap">' +
                    '<span class="class-deck-item__name">' + app.esc(d.deck_name) + '</span>' +
                    subj +
                    '<span class="text-muted" style="font-size:.8rem">' + d.card_count + ' carta(s)</span>' +
                '</div>' +
                '<button class="btn btn-ghost btn-sm btn-remove-deck" data-id="' + d.deck_id + '" data-name="' + app.esc(d.deck_name) + '"' +
                    ' title="Remover da turma" style="color:var(--danger);flex-shrink:0">' +
                    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>' +
                '</button>' +
            '</div>';
        }).join('');

        deckListEl.querySelectorAll(".btn-remove-deck").forEach(function (btn) {
            btn.addEventListener("click", function () {
                if (!confirm("Remover \"" + btn.dataset.name + "\" desta turma?")) return;
                api.del("/api/classes/" + classId + "/decks/" + btn.dataset.id)
                    .then(function (r) {
                        if (!r.ok) throw new Error("Erro ao remover deck");
                        loadClassDecks();
                        toast("Deck removido da turma.", "success");
                    })
                    .catch(function (e) { toast(e.message, "error"); });
            });
        });
    }

    function wireActions() {
        // Toggle active
        if (btnToggle) btnToggle.addEventListener("click", function () {
            if (!currentClass) return;
            var newActive = !currentClass.is_active;
            api.put("/api/classes/" + classId, {
                name: currentClass.name,
                description: currentClass.description || null,
                is_active: newActive
            }).then(function (r) { return r.ok ? r.json() : null; })
                .then(function (cl) {
                    if (!cl) return;
                    currentClass = cl;
                    updateStatusBadge(cl.is_active);
                    toast(cl.is_active ? "Turma ativada." : "Turma desativada.", "success");
                });
        });

        // Delete class
        if (btnDelete) btnDelete.addEventListener("click", function () {
            if (!currentClass) return;
            if (!confirm("Excluir a turma \"" + currentClass.name + "\"?\n\nOs decks e alunos não serão excluídos, apenas a associação.")) return;
            api.del("/api/classes/" + classId)
                .then(function (r) {
                    if (!r.ok) throw new Error("Erro ao excluir turma");
                    window.location.href = "/classes.html";
                })
                .catch(function (e) { toast(e.message, "error"); });
        });

        // Copy invite code
        if (btnCopy) btnCopy.addEventListener("click", function () {
            var code = inviteCode ? inviteCode.textContent : "";
            if (!code) return;
            navigator.clipboard.writeText(code).then(function () {
                toast("Código copiado!", "success");
            }).catch(function () {
                toast("Código: " + code, "success");
            });
        });

        // Regenerate invite code
        if (btnRegen) btnRegen.addEventListener("click", function () {
            if (!confirm("Gerar um novo código? O código atual deixará de funcionar.")) return;
            api.post("/api/classes/" + classId + "/invite")
                .then(function (r) { return r.ok ? r.json() : null; })
                .then(function (data) {
                    if (!data) return;
                    if (inviteCode) inviteCode.textContent = data.invite_code;
                    if (currentClass) currentClass.invite_code = data.invite_code;
                    toast("Novo código gerado.", "success");
                });
        });

        // Add deck panel
        if (btnAddDeck) btnAddDeck.addEventListener("click", function () {
            if (addDeckPanel) addDeckPanel.classList.remove("hidden");
            if (deckSelect) deckSelect.value = "";
        });
        if (btnCancelAddDeck) btnCancelAddDeck.addEventListener("click", function () {
            if (addDeckPanel) addDeckPanel.classList.add("hidden");
        });
        if (btnConfirmAddDeck) btnConfirmAddDeck.addEventListener("click", function () {
            var deckId = deckSelect ? deckSelect.value : "";
            if (!deckId) { toast("Selecione um deck.", "error"); return; }
            btnConfirmAddDeck.disabled = true;
            api.post("/api/classes/" + classId + "/decks", { deck_id: deckId })
                .then(function (r) {
                    if (!r.ok) return r.json().then(function (e) { throw new Error(e.detail || "Erro ao adicionar deck"); });
                    if (addDeckPanel) addDeckPanel.classList.add("hidden");
                    loadClassDecks();
                    toast("Deck adicionado à turma.", "success");
                })
                .catch(function (e) { toast(e.message, "error"); })
                .finally(function () { btnConfirmAddDeck.disabled = false; });
        });
    }

    /* ── Tabs ───────────────────────────────────────────── */
    function wireTabs() {
        document.querySelectorAll(".class-tab").forEach(function (tab) {
            tab.addEventListener("click", function () {
                document.querySelectorAll(".class-tab").forEach(function (t) {
                    t.classList.remove("class-tab--active");
                    t.setAttribute("aria-selected", "false");
                });
                document.querySelectorAll(".tab-panel").forEach(function (p) {
                    p.classList.add("hidden");
                });
                tab.classList.add("class-tab--active");
                tab.setAttribute("aria-selected", "true");
                var panel = document.getElementById("tab-" + tab.dataset.tab);
                if (panel) panel.classList.remove("hidden");

                if (tab.dataset.tab === "stats" && !statsLoaded) {
                    loadStats();
                }
            });
        });
    }

    /* ── Stats ──────────────────────────────────────────── */
    function loadStats() {
        if (statsSpinner) statsSpinner.classList.remove("hidden");
        if (statsContent) statsContent.classList.add("hidden");
        if (statsEmpty)   statsEmpty.classList.add("hidden");

        api.get("/api/classes/" + classId + "/stats")
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (s) {
                if (statsSpinner) statsSpinner.classList.add("hidden");
                if (!s) { if (statsEmpty) statsEmpty.classList.remove("hidden"); return; }
                statsLoaded = true;
                renderStats(s);
            })
            .catch(function () {
                if (statsSpinner) statsSpinner.classList.add("hidden");
                if (statsEmpty) statsEmpty.classList.remove("hidden");
            });
    }

    function renderStats(s) {
        if (statsContent) statsContent.classList.remove("hidden");

        // KPIs
        setText(kpiMembers,  s.total_members);
        setText(kpiActive,   s.active_members + " (" + pct(s.active_members, s.total_members) + "%)");
        setText(kpiActive7,  s.active_last_7d  + " (" + pct(s.active_last_7d, s.total_members) + "%)");
        setText(kpiReviews7, s.reviews_last_7d);
        setText(kpiAccuracy, fmt1(s.accuracy_pct) + "%");
        setText(kpiCards,    s.total_cards);

        // Deck breakdown — responsive cards
        var decks = s.deck_stats || [];
        if (deckStatsList) {
            if (decks.length === 0) {
                deckStatsList.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Nenhum deck com atividade ainda.</p>';
            } else {
                deckStatsList.innerHTML = decks.map(function (d) {
                    var accPct   = fmt1(d.accuracy_pct);
                    var accColor = d.accuracy_pct >= 70 ? 'var(--green-700,#388E3C)' : d.accuracy_pct >= 40 ? '#E65100' : '#B71C1C';
                    var barW     = Math.min(100, Math.round(d.accuracy_pct));
                    var lastAct  = d.last_activity ? new Date(d.last_activity).toLocaleDateString("pt-BR") : '—';
                    var subj     = d.subject
                        ? '<span class="deck-perf-card__badge">' + app.esc(d.subject) + '</span>'
                        : '';
                    return '<div class="deck-perf-card">' +
                        '<div class="deck-perf-card__header">' +
                            '<span class="deck-perf-card__name">' + app.esc(d.deck_name) + '</span>' +
                            subj +
                        '</div>' +
                        '<div class="deck-perf-card__stats">' +
                            '<div class="deck-perf-card__stat">' +
                                '<span class="deck-perf-card__stat-val">' + d.total_cards + '</span>' +
                                '<span class="deck-perf-card__stat-lbl">Cartas</span>' +
                            '</div>' +
                            '<div class="deck-perf-card__stat">' +
                                '<span class="deck-perf-card__stat-val">' + d.students_studied + '</span>' +
                                '<span class="deck-perf-card__stat-lbl">Alunos</span>' +
                            '</div>' +
                            '<div class="deck-perf-card__stat">' +
                                '<span class="deck-perf-card__stat-val">' + d.active_last_7d + '</span>' +
                                '<span class="deck-perf-card__stat-lbl">Ativos 7d</span>' +
                            '</div>' +
                        '</div>' +
                        '<div class="deck-perf-card__acc-row">' +
                            '<div class="deck-perf-card__bar-wrap">' +
                                '<div class="deck-perf-card__bar" style="width:' + barW + '%;background:' + accColor + '"></div>' +
                            '</div>' +
                            '<span class="deck-perf-card__acc-pct" style="color:' + accColor + '">' + accPct + '%</span>' +
                        '</div>' +
                        '<div class="deck-perf-card__footer">' +
                            '<span>Taxa de acerto</span>' +
                            '<span>Última atividade: ' + lastAct + '</span>' +
                        '</div>' +
                    '</div>';
                }).join('');
            }
        }

        // Hardest cards
        var hard = s.hardest_cards || [];
        if (hardCardsList) {
            if (hard.length === 0) {
                hardCardsList.innerHTML = '<p class="text-muted" style="font-size:.85rem">Ainda sem dados suficientes (mínimo 3 revisões por card).</p>';
            } else {
                hardCardsList.innerHTML = hard.map(function (c, i) {
                    var barW = Math.round(c.error_rate);
                    var barColor = c.error_rate >= 70 ? '#B71C1C' : c.error_rate >= 40 ? '#E65100' : '#F57F17';
                    return '<div class="hard-card-item">' +
                        '<div class="hard-card-item__rank">' + (i + 1) + '</div>' +
                        '<div class="hard-card-item__body">' +
                            '<div class="hard-card-item__question">' + app.esc(c.question) + '</div>' +
                            '<div class="hard-card-item__meta">' +
                                '<span class="text-muted">' + app.esc(c.deck_name) + '</span>' +
                                '<span>·</span>' +
                                '<span class="text-muted">' + c.total_reviews + ' revisões</span>' +
                            '</div>' +
                            '<div class="hard-card-item__bar-wrap">' +
                                '<div class="hard-card-item__bar" style="width:' + barW + '%;background:' + barColor + '"></div>' +
                            '</div>' +
                            '<div style="font-size:.78rem;color:' + barColor + ';font-weight:700;margin-top:.15rem">' +
                                fmt1(c.error_rate) + '% erro/difícil' +
                            '</div>' +
                        '</div>' +
                    '</div>';
                }).join('');
            }
        }
    }

    function setText(el, val) { if (el) el.textContent = val; }
    function fmt1(n) { return (Math.round((n || 0) * 10) / 10).toFixed(1); }
    function pct(a, b) { return b > 0 ? Math.round(a / b * 100) : 0; }

    init();
})();
