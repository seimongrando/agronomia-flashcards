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
    var deckLink     = document.getElementById("deck-link");
    var deckManageLink = document.getElementById("deck-manage-link");

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

    var csvToggle     = document.getElementById("csv-toggle");
    var csvBody       = document.getElementById("csv-body");

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
            var name = deckSelect.options[deckSelect.selectedIndex].text;
            manualDeckInner.className = "manual-deck-ctx--ok";
            manualDeckInner.innerHTML =
                '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color:var(--green-800);flex-shrink:0">' +
                '<path d="M5 12l4 4L19 8" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                'Deck ativo: <strong>' + app.esc(name) + '</strong>';
        } else {
            manualDeckInner.className = "manual-deck-ctx--warn";
            manualDeckInner.innerHTML =
                '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color:#F9A825;flex-shrink:0;margin-top:.1rem">' +
                '<path d="M12 9v4M12 17h.01M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                '<p>Nenhum deck selecionado. ' +
                '<a href="#deck-select" id="link-goto-deck" style="color:var(--primary);font-weight:600">Selecione ou crie um deck</a> ' +
                'antes de salvar o card.</p>';
            var linkGoto = document.getElementById("link-goto-deck");
            if (linkGoto) {
                linkGoto.addEventListener("click", function (e) {
                    e.preventDefault();
                    deckSelect.focus();
                    deckSelect.scrollIntoView({ behavior: "smooth", block: "center" });
                });
            }
        }
    }

    var currentFile      = null;
    var previewResult    = null;
    var currentDeckID    = null;

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
            app.renderTopbar(user);
            contentEl.classList.remove("hidden");
            loadDecks();
        });
    }

    /* ── Decks ───────────────────────────────────── */
    function loadDecks() {
        api.get("/api/decks").then(function (res) {
            if (!res.ok) return;
            return res.json();
        }).then(function (page) {
            var decks = (page && page.items) ? page.items : (page || []);
            var subjects = {};
            for (var i = 0; i < decks.length; i++) {
                var opt = document.createElement("option");
                opt.value = decks[i].id;
                opt.textContent = decks[i].subject
                    ? decks[i].name + " [" + decks[i].subject + "]"
                    : decks[i].name;
                deckSelect.appendChild(opt);
                if (decks[i].subject) subjects[decks[i].subject] = true;
            }
            // Populate autocomplete datalist
            if (subjectDatalist) {
                Object.keys(subjects).sort().forEach(function (s) {
                    var o = document.createElement("option");
                    o.value = s;
                    subjectDatalist.appendChild(o);
                });
            }
        });
    }

    deckSelect.addEventListener("change", function () {
        currentDeckID = deckSelect.value || null;
        if (currentDeckID) {
            var selectedName = deckSelect.options[deckSelect.selectedIndex].text;
            deckManageLink.href = "/deck_manage.html?deckId=" + currentDeckID +
                "&deckName=" + encodeURIComponent(selectedName);
            deckLink.classList.remove("hidden");
        } else {
            deckLink.classList.add("hidden");
        }
        updateManualDeckCtx();
        clearPreview();
    });

    btnNewDeck.addEventListener("click", function () {
        newDeckForm.style.display = "";
        newDeckForm.classList.remove("hidden");
        newDeckName.focus();
    });

    btnCancel.addEventListener("click", function () {
        newDeckForm.classList.add("hidden");
        newDeckName.value = "";
    });

    btnCreate.addEventListener("click", createDeck);
    newDeckName.addEventListener("keydown", function (e) { if (e.key === "Enter") createDeck(); });

    function createDeck() {
        var name = newDeckName.value.trim();
        if (!name) return;
        var subject = (newDeckSubject && newDeckSubject.value.trim()) || null;
        btnCreate.disabled = true;
        api.post("/api/content/decks", { name: name, subject: subject })
            .then(function (res) { return res.json(); })
            .then(function (deck) {
                if (!deck.id) throw new Error("no id");
                var opt = document.createElement("option");
                opt.value = deck.id;
                opt.textContent = deck.name + (deck.subject ? " [" + deck.subject + "]" : "");
                deckSelect.appendChild(opt);
                deckSelect.value = deck.id;
                deckSelect.dispatchEvent(new Event("change"));
                newDeckForm.classList.add("hidden");
                newDeckName.value = "";
                if (newDeckSubject) newDeckSubject.value = "";
                // add new subject to autocomplete if not already there
                if (subject && subjectDatalist) {
                    var exists = false;
                    for (var i = 0; i < subjectDatalist.options.length; i++) {
                        if (subjectDatalist.options[i].value === subject) { exists = true; break; }
                    }
                    if (!exists) {
                        var o = document.createElement("option");
                        o.value = subject;
                        subjectDatalist.appendChild(o);
                    }
                }
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
        var prev = deckSelect.value;
        while (deckSelect.options.length > 1) deckSelect.remove(1);
        api.get("/api/decks").then(function (res) { return res.json(); }).then(function (page) {
            var decks = (page && page.items) ? page.items : (page || []);
            for (var i = 0; i < decks.length; i++) {
                var opt = document.createElement("option");
                opt.value = decks[i].id;
                opt.textContent = decks[i].subject
                    ? decks[i].name + " [" + decks[i].subject + "]"
                    : decks[i].name;
                deckSelect.appendChild(opt);
            }
            if (prev) { deckSelect.value = prev; deckSelect.dispatchEvent(new Event("change")); }
        });
    }

    /* ── Collapsible CSV section ─────────────────── */
    csvToggle.addEventListener("click", function () {
        var open = csvToggle.getAttribute("aria-expanded") === "true";
        csvToggle.setAttribute("aria-expanded", open ? "false" : "true");
        if (open) {
            csvBody.classList.add("hidden");
        } else {
            csvBody.classList.remove("hidden");
        }
    });

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
        var deckID = deckSelect.value;
        if (!deckID) {
            cardError.innerHTML =
                'Nenhum deck selecionado. ' +
                '<a href="#deck-select" style="color:var(--danger);font-weight:600;text-decoration:underline" id="err-deck-link">Selecione ou crie um deck</a> acima.';
            cardError.classList.remove("hidden");
            var errLink = document.getElementById("err-deck-link");
            if (errLink) {
                errLink.addEventListener("click", function (e) {
                    e.preventDefault();
                    deckSelect.focus();
                    deckSelect.scrollIntoView({ behavior: "smooth", block: "center" });
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
            .finally(function () { btnSave.disabled = false; });
    }

    /* ── Helpers ─────────────────────────────────── */
    function trunc(s, n) {
        if (!s) return "";
        return s.length > n ? s.slice(0, n) + "…" : s;
    }

    init();
})();
