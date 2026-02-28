(function () {
    "use strict";

    /* ── DOM refs ────────────────────────────────── */
    var spinnerEl    = document.getElementById("spinner");
    var deniedEl     = document.getElementById("access-denied");
    var contentEl    = document.getElementById("teach-content");

    var deckSelect      = document.getElementById("deck-select");
    var btnNewDeck      = document.getElementById("btn-new-deck");
    var newDeckForm     = document.getElementById("new-deck-form");
    var newDeckName     = document.getElementById("new-deck-name");
    var newDeckSubject  = document.getElementById("new-deck-subject");
    var subjectDatalist = document.getElementById("subject-suggestions");
    var btnCreate       = document.getElementById("btn-create-deck");
    var btnCancel       = document.getElementById("btn-cancel-deck");
    var deckPickerGrid  = document.getElementById("deck-picker-grid");
    var deckEmptyMsg    = document.getElementById("deck-empty-msg");
    var activeDeckBar   = document.getElementById("active-deck-bar");
    var activeDeckInfo  = document.getElementById("active-deck-info");
    var deckManageLink  = document.getElementById("deck-manage-link");

    var dropzone     = document.getElementById("dropzone");
    var fileInput    = document.getElementById("csv-file");
    var fileNameEl   = document.getElementById("file-name");
    var previewArea  = document.getElementById("preview-area");
    var previewStats = document.getElementById("preview-stats");
    var previewWrap  = document.getElementById("preview-wrap");
    var btnImport    = document.getElementById("btn-import");
    var importHint   = document.getElementById("import-hint");
    var btnClear     = document.getElementById("btn-clear-file");
    var importResult = document.getElementById("import-result");

    var csvToggle      = document.getElementById("csv-toggle");
    var csvBody        = document.getElementById("csv-body");
    var csvGuideToggle = document.getElementById("csv-guide-toggle");
    var csvGuideBody   = document.getElementById("csv-guide-body");

    var manualToggle  = document.getElementById("manual-toggle");
    var manualBody    = document.getElementById("manual-body");
    var manualDeckInner = document.getElementById("manual-deck-inner");
    var cardQuestion  = document.getElementById("card-question");
    var cardAnswer    = document.getElementById("card-answer");
    var cardTopic     = document.getElementById("card-topic");
    var cardSource    = document.getElementById("card-source");
    var cardError     = document.getElementById("card-error");
    var btnSave       = document.getElementById("btn-save-card");

    function getSelectedType() {
        var radios = document.querySelectorAll('input[name="card-type"]');
        for (var i = 0; i < radios.length; i++) {
            if (radios[i].checked) return radios[i].value;
        }
        return "conceito";
    }

    function updateManualDeckCtx() {
        if (!manualDeckInner) return;
        if (currentDeckID) {
            manualDeckInner.className = "manual-deck-ctx--ok";
            manualDeckInner.innerHTML =
                '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color:var(--green-800);flex-shrink:0">' +
                '<path d="M5 12l4 4L19 8" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                'Deck ativo: <strong>' + app.esc(currentDeckName) + '</strong>';
        } else {
            manualDeckInner.className = "manual-deck-ctx--warn";
            manualDeckInner.innerHTML =
                '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color:#F9A825;flex-shrink:0;margin-top:.1rem">' +
                '<path d="M12 9v4M12 17h.01M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                '<p>Nenhum deck selecionado. ' +
                '<a href="#deck-picker-grid" id="link-goto-deck" style="color:var(--primary);font-weight:600">Selecione ou crie um deck</a> ' +
                'antes de salvar o card.</p>';
            var linkGoto = document.getElementById("link-goto-deck");
            if (linkGoto) {
                linkGoto.addEventListener("click", function (e) {
                    e.preventDefault();
                    if (deckPickerGrid) deckPickerGrid.scrollIntoView({ behavior: "smooth", block: "center" });
                });
            }
        }
    }

    var currentFile      = null;
    var previewResult    = null;
    var currentDeckID    = null;
    var currentDeckName  = "";
    var allDecks         = [];
    var currentUserID    = null;  // populated on init from /api/me
    var isAdmin          = false; // admins bypass ownership checks

    /* ── Helpers ─────────────────────────────────── */
    function canEditDeck(deck) {
        if (isAdmin) return true;
        return deck.created_by && deck.created_by === currentUserID;
    }

    /* ── Init ────────────────────────────────────── */
    function init() {
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) { window.location.href = "/"; return; }

            var roles = user.roles || [];
            var allowed = roles.indexOf("professor") >= 0 || roles.indexOf("admin") >= 0;
            if (!allowed) {
                app.renderTopbar(user);
                deniedEl.classList.remove("hidden");
                return;
            }
            isAdmin = roles.indexOf("admin") >= 0;
            currentUserID = user.user ? user.user.id : null;
            app.renderTopbar(user);
            contentEl.classList.remove("hidden");
            loadDecks();
        });
    }

    /* ── Decks ───────────────────────────────────── */
    function loadDecks() {
        api.get("/api/content/decks").then(function (res) {
            if (!res.ok) return;
            return res.json();
        }).then(function (page) {
            allDecks = (page && page.items) ? page.items : (page || []);
            renderDeckGrid(allDecks);
        });
    }

    function renderDeckGrid(decks) {
        if (!deckPickerGrid) return;

        // Rebuild hidden select (needed for CSV upload URL)
        while (deckSelect.options.length > 1) deckSelect.remove(1);
        var subjects = {};
        for (var i = 0; i < decks.length; i++) {
            var d = decks[i];
            var opt = document.createElement("option");
            opt.value = d.id;
            opt.textContent = d.name;
            deckSelect.appendChild(opt);
            if (d.subject) subjects[d.subject] = true;
        }
        // Populate autocomplete datalist
        if (subjectDatalist) {
            subjectDatalist.innerHTML = "";
            Object.keys(subjects).sort().forEach(function (s) {
                var o = document.createElement("option");
                o.value = s;
                subjectDatalist.appendChild(o);
            });
        }

        if (decks.length === 0) {
            deckPickerGrid.innerHTML = "";
            if (deckEmptyMsg) deckEmptyMsg.classList.remove("hidden");
            return;
        }
        if (deckEmptyMsg) deckEmptyMsg.classList.add("hidden");

        var now = new Date();
        deckPickerGrid.innerHTML = decks.map(function (d) {
            var isExpired = d.expires_at && new Date(d.expires_at) <= now;
            var active    = d.is_active && !isExpired;
            var statusTag = active
                ? '<span class="badge badge-green" style="font-size:.72rem">Ativo</span>'
                : (isExpired
                    ? '<span class="badge badge-expired" style="font-size:.72rem">Expirado</span>'
                    : '<span class="badge badge-inactive" style="font-size:.72rem">Inativo</span>');

            var owned      = canEditDeck(d);
            var isSelected = d.id === currentDeckID;
            var selectedCls = isSelected ? " deck-picker-card--selected" : "";
            // Non-owner decks are shown read-only with a visual indicator
            var readOnlyCls = !owned ? " deck-picker-card--readonly" : "";

            var ownerTag = !owned
                ? '<span class="badge" style="font-size:.68rem;background:#FFF3E0;color:#E65100;border:1px solid #FFCC80" title="Você pode visualizar mas não editar este deck">somente leitura</span>'
                : '<span class="badge" style="font-size:.68rem;background:#E8F5E9;color:#1B5E20;border:1px solid #A5D6A7">seu deck</span>';

            return '<div class="deck-picker-card' + selectedCls + readOnlyCls + '" role="listitem" data-id="' + d.id + '" data-name="' + app.esc(d.name) + '" data-owned="' + (owned ? '1' : '0') + '" tabindex="0" aria-pressed="' + isSelected + '">' +
                '<div class="deck-picker-card__head">' +
                    '<span class="deck-picker-card__name">' + app.esc(d.name) + '</span>' +
                    '<div style="display:flex;gap:.3rem;flex-wrap:wrap">' + statusTag + ownerTag + '</div>' +
                '</div>' +
                (d.subject ? '<div class="deck-picker-card__sub">' + app.esc(d.subject) + '</div>' : '') +
                '<div class="deck-picker-card__count">' + (d.total_cards || 0) + ' carta' + (d.total_cards !== 1 ? 's' : '') + '</div>' +
            '</div>';
        }).join('');

        // Click handler for each card
        deckPickerGrid.querySelectorAll('.deck-picker-card').forEach(function (el) {
            el.addEventListener('click', function () { selectDeck(el.dataset.id, el.dataset.name, el.dataset.owned === '1'); });
            el.addEventListener('keydown', function (e) {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); selectDeck(el.dataset.id, el.dataset.name, el.dataset.owned === '1'); }
            });
        });
    }

    function selectDeck(id, name, owned) {
        // If owned not explicitly supplied, derive it from the loaded deck list.
        if (owned === undefined) {
            var found = allDecks.filter(function (d) { return d.id === id; })[0];
            owned = found ? canEditDeck(found) : false;
        }
        currentDeckID   = id;
        currentDeckName = name;
        deckSelect.value = id;

        // Visual selection state
        deckPickerGrid.querySelectorAll('.deck-picker-card').forEach(function (el) {
            var selected = el.dataset.id === id;
            el.classList.toggle('deck-picker-card--selected', selected);
            el.setAttribute('aria-pressed', selected ? 'true' : 'false');
        });

        // Active deck bar — show manage link only if professor owns the deck (or is admin)
        if (activeDeckBar && activeDeckInfo && deckManageLink) {
            var icon = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color:var(--green-800)">' +
                '<path d="M5 12l4 4L19 8" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
            activeDeckInfo.innerHTML = icon + ' Deck selecionado: <strong>' + app.esc(name) + '</strong>';

            if (owned) {
                deckManageLink.href = "/deck_manage.html?deckId=" + id + "&deckName=" + encodeURIComponent(name);
                deckManageLink.style.display = "";
            } else {
                deckManageLink.style.display = "none";
            }
            activeDeckBar.classList.remove("hidden");
        }

        // Disable manual card form for non-owned decks
        if (btnSave) {
            btnSave.disabled = !owned;
            btnSave.title = owned ? "" : "Você não pode adicionar cards a um deck de outro professor";
        }

        updateManualDeckCtx();
        clearPreview();
    }

    btnNewDeck.addEventListener("click", function () {
        newDeckForm.classList.toggle("hidden");
        if (!newDeckForm.classList.contains("hidden")) newDeckName.focus();
    });

    btnCancel.addEventListener("click", function () {
        newDeckForm.classList.add("hidden");
        newDeckName.value = "";
        if (newDeckSubject) newDeckSubject.value = "";
    });

    btnCreate.addEventListener("click", createDeck);
    newDeckName.addEventListener("keydown", function (e) { if (e.key === "Enter") createDeck(); });

    function createDeck() {
        var name = newDeckName.value.trim();
        if (!name) { newDeckName.focus(); return; }
        var subject = (newDeckSubject && newDeckSubject.value.trim()) || null;
        btnCreate.disabled = true;
        api.post("/api/content/decks", { name: name, subject: subject })
            .then(function (res) {
                if (res.status === 409) {
                    toast("Já existe um deck com este nome. Escolha outro nome.", "error");
                    btnCreate.disabled = false;
                    return null;
                }
                return res.json();
            })
            .then(function (deck) {
                if (!deck || !deck.id) return;
                // Add to allDecks and re-render — include created_by so ownership resolves correctly
                allDecks.unshift({
                    id: deck.id,
                    name: deck.name,
                    subject: deck.subject,
                    is_active: true,
                    total_cards: 0,
                    created_by: currentUserID  // professor who just created it always owns it
                });
                renderDeckGrid(allDecks);
                selectDeck(deck.id, deck.name); // owned will be derived from allDecks above
                newDeckForm.classList.add("hidden");
                newDeckName.value = "";
                if (newDeckSubject) newDeckSubject.value = "";
                toast("Deck criado!", "ok");
            })
            .catch(function () { toast("Erro ao criar deck", "error"); })
            .finally(function () { btnCreate.disabled = false; });
    }

    /* ── Drop zone ───────────────────────────────── */
    dropzone.addEventListener("click", function () { fileInput.click(); });
    fileInput.addEventListener("change", function () {
        if (fileInput.files[0]) handleFile(fileInput.files[0]);
    });

    dropzone.addEventListener("dragover", function (e) {
        e.preventDefault(); dropzone.classList.add("dragover");
    });
    dropzone.addEventListener("dragleave", function () {
        dropzone.classList.remove("dragover");
    });
    dropzone.addEventListener("drop", function (e) {
        e.preventDefault(); dropzone.classList.remove("dragover");
        var f = e.dataTransfer.files[0];
        if (f) handleFile(f);
    });

    function handleFile(file) {
        if (file.size > 2 * 1024 * 1024) {
            toast("Arquivo muito grande (máx 2 MB)", "error"); return;
        }
        currentFile = file;
        fileNameEl.textContent = file.name;
        fileNameEl.classList.remove("hidden");
        runPreview(file);
    }

    function runPreview(file) {
        btnImport.disabled = true;
        importHint.textContent = "Analisando…";
        previewArea.classList.remove("hidden");
        previewStats.innerHTML = '<div class="spinner" style="width:20px;height:20px;margin:.5rem 0"></div>';
        previewWrap.innerHTML = "";
        importResult.classList.add("hidden");

        var uploadURL = "/api/content/upload-csv?dryRun=1" + (currentDeckID ? "&deckId=" + currentDeckID : "");
        api.upload(uploadURL, file)
            .then(function (res) { return res.json(); })
            .then(function (result) {
                previewResult = result;
                renderPreview(result);
            })
            .catch(function () {
                previewStats.innerHTML = '<span class="alert alert-error">Erro ao analisar arquivo.</span>';
                importHint.textContent = "";
            });
    }

    function renderPreview(result) {
        var valid   = result.valid_rows   || 0;
        var invalid = result.invalid_rows || 0;
        var total   = result.total_rows   || 0;
        var shown   = result.rows ? result.rows.length : 0;

        previewStats.innerHTML =
            '<span>Total: <strong>' + total + '</strong></span>' +
            '<span class="stat-ok">✓ ' + valid + ' ok</span>' +
            (invalid > 0 ? '<span class="stat-err">✗ ' + invalid + ' erro(s)</span>' : '') +
            (shown < total ? '<span class="text-muted">Mostrando ' + shown + ' primeiras linhas</span>' : '');

        if (!result.rows || result.rows.length === 0) {
            previewWrap.innerHTML = '<p class="text-muted" style="padding:.5rem">Nenhuma linha de dados encontrada.</p>';
        } else {
            var html = '<table class="preview-table"><thead><tr>' +
                '<th>#</th><th>Deck</th><th>Tipo</th><th>Pergunta</th><th>Resposta</th><th>Status</th>' +
                '</tr></thead><tbody>';
            for (var i = 0; i < result.rows.length; i++) {
                var row = result.rows[i];
                var cls = row.status === "ok" ? "row-ok" : "row-err";
                html += '<tr class="' + cls + '">' +
                    '<td>' + row.line + '</td>' +
                    '<td>' + app.esc(row.deck || '') + '</td>' +
                    '<td>' + app.esc(row.type || '') + '</td>' +
                    '<td title="' + app.esc(row.question || '') + '">' + app.esc(trunc(row.question, 60)) + '</td>' +
                    '<td title="' + app.esc(row.answer || '') + '">'   + app.esc(trunc(row.answer, 60))   + '</td>' +
                    '<td>' + (row.status === "ok"
                        ? '<span style="color:var(--success)">✓</span>'
                        : '<span class="err-msg">' + app.esc(row.error || "erro") + '</span>') +
                    '</td></tr>';
            }
            html += '</tbody></table>';
            previewWrap.innerHTML = html;
        }

        if (valid > 0) {
            btnImport.disabled = false;
            btnImport.textContent = "Importar " + valid + (valid === 1 ? " carta" : " cartas");
            importHint.textContent = invalid > 0 ? "Linhas com erro serão ignoradas." : "";
        } else {
            btnImport.disabled = true;
            importHint.textContent = "Nenhuma linha válida para importar.";
        }
    }

    btnImport.addEventListener("click", function () {
        if (!currentFile) return;
        btnImport.disabled = true;
        btnImport.textContent = "Importando…";
        importResult.classList.add("hidden");

        var importURL = "/api/content/upload-csv?dryRun=0" + (currentDeckID ? "&deckId=" + currentDeckID : "");
        api.upload(importURL, currentFile)
            .then(function (res) { return res.json(); })
            .then(function (result) {
                importResult.className = "alert alert-ok mt-1";
                importResult.textContent =
                    "Importado: " + result.imported_count + " novas, " +
                    result.updated_count + " atualizadas, " +
                    result.invalid_count + " ignoradas.";
                importResult.classList.remove("hidden");
                toast("Importação concluída!", "ok");
                clearPreview();
                reloadDecks();
            })
            .catch(function () {
                importResult.className = "alert alert-error mt-1";
                importResult.textContent = "Erro durante a importação.";
                importResult.classList.remove("hidden");
                btnImport.disabled = false;
                btnImport.textContent = "Tentar novamente";
            });
    });

    btnClear.addEventListener("click", clearPreview);

    function clearPreview() {
        currentFile = null;
        previewResult = null;
        fileInput.value = "";
        fileNameEl.classList.add("hidden");
        previewArea.classList.add("hidden");
    }

    function reloadDecks() {
        var prevID = currentDeckID;
        var prevName = currentDeckName;
        api.get("/api/content/decks").then(function (res) { return res.json(); }).then(function (page) {
            allDecks = (page && page.items) ? page.items : (page || []);
            renderDeckGrid(allDecks);
            if (prevID) selectDeck(prevID, prevName);
        });
    }

    /* ── Collapsible CSV section ─────────────────── */
    csvToggle.addEventListener("click", function () {
        var open = csvToggle.getAttribute("aria-expanded") === "true";
        csvToggle.setAttribute("aria-expanded", open ? "false" : "true");
        csvBody.classList.toggle("hidden", open);
    });

    /* ── CSV format guide sub-toggle ────────────── */
    if (csvGuideToggle && csvGuideBody) {
        csvGuideToggle.addEventListener("click", function () {
            var open = csvGuideToggle.getAttribute("aria-expanded") === "true";
            csvGuideToggle.setAttribute("aria-expanded", open ? "false" : "true");
            csvGuideBody.classList.toggle("hidden", open);
            csvGuideToggle.textContent = open ? "Ver formato e exemplos ▾" : "Ocultar formato ▴";
        });
    }

    /* ── Collapsible manual form ─────────────────── */
    manualToggle.addEventListener("click", function () {
        var open = manualToggle.getAttribute("aria-expanded") === "true";
        manualToggle.setAttribute("aria-expanded", open ? "false" : "true");
        if (open) {
            manualBody.classList.add("hidden");
        } else {
            manualBody.classList.remove("hidden");
            updateManualDeckCtx();
        }
    });

    btnSave.addEventListener("click", saveCard);

    function saveCard() {
        var deckID = currentDeckID;
        if (!deckID) {
            cardError.innerHTML =
                'Nenhum deck selecionado. ' +
                '<a href="#deck-picker-grid" style="color:var(--danger);font-weight:600;text-decoration:underline" id="err-deck-link">Selecione ou crie um deck</a> acima.';
            cardError.classList.remove("hidden");
            var errLink = document.getElementById("err-deck-link");
            if (errLink) {
                errLink.addEventListener("click", function (e) {
                    e.preventDefault();
                    if (deckPickerGrid) deckPickerGrid.scrollIntoView({ behavior: "smooth", block: "center" });
                });
            }
            return;
        }
        var q = cardQuestion.value.trim();
        var a = cardAnswer.value.trim();
        if (!q || !a) {
            cardError.textContent = "Pergunta e resposta são obrigatórias.";
            cardError.classList.remove("hidden"); return;
        }
        cardError.classList.add("hidden");
        btnSave.disabled = true;

        var body = {
            deck_id:  deckID,
            type:     getSelectedType(),
            question: q,
            answer:   a
        };
        var topic  = cardTopic.value.trim();
        var source = cardSource.value.trim();
        if (topic)  body.topic  = topic;
        if (source) body.source = source;

        api.post("/api/content/cards", body)
            .then(function (res) {
                if (!res.ok) return res.json().then(function (e) { throw new Error(e.detail || "erro"); });
                cardQuestion.value = "";
                cardAnswer.value   = "";
                cardTopic.value    = "";
                cardSource.value   = "";
                toast("Card salvo!", "ok");
            })
            .catch(function (err) {
                cardError.textContent = err.message || "Erro ao salvar card.";
                cardError.classList.remove("hidden");
            })
            .finally(function () {
                // Re-enable only if the current deck is still owned by this professor
                var activeDeck = allDecks.filter(function (d) { return d.id === currentDeckID; })[0];
                btnSave.disabled = activeDeck ? !canEditDeck(activeDeck) : true;
            });
    }

    /* ── Helpers ─────────────────────────────────── */
    function trunc(s, n) {
        if (!s) return "";
        return s.length > n ? s.slice(0, n) + "…" : s;
    }

    init();
})();
