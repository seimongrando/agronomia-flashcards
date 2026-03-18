(function () {
    "use strict";

    var PAGE_SIZE = 20;

    var params    = new URLSearchParams(window.location.search);
    var deckId    = params.get("deckId");
    var deckName  = params.get("deckName") ? decodeURIComponent(params.get("deckName")) : null;

    var spinnerEl    = document.getElementById("spinner");
    var deckContent  = document.getElementById("deck-content");
    var deckTitle    = document.getElementById("deck-title");
    var deckSubtitle = document.getElementById("deck-subtitle");
    var searchInput  = document.getElementById("search-input");
    var cardListEl   = document.getElementById("card-list");
    var emptyEl      = document.getElementById("empty-cards");
    var deleteZoneEl = document.getElementById("delete-zone");
    var btnDeleteDeck = document.getElementById("btn-delete-deck");
    var paginationEl = document.getElementById("pagination");

    var deckStatusBadge    = document.getElementById("deck-status-badge");
    var btnExportCSV       = document.getElementById("btn-export-csv");
    var btnRenameDeck      = document.getElementById("btn-rename-deck");
    var renameForm         = document.getElementById("rename-form");
    var renameNameEl       = document.getElementById("rename-name");
    var renameSubjectEl    = document.getElementById("rename-subject");
    var renameSubjectDl    = document.getElementById("rename-subject-suggestions");
    var renameDescEl       = document.getElementById("rename-desc");
    var renameActiveEl     = document.getElementById("rename-active");
    var toggleActiveLbl    = document.getElementById("toggle-active-lbl");
    var renameExpiresEl    = document.getElementById("rename-expires");
    var btnClearExpiry     = document.getElementById("btn-clear-expiry");
    var renameError        = document.getElementById("rename-error");
    var btnRenameSave      = document.getElementById("btn-rename-save");
    var btnRenameCancel    = document.getElementById("btn-rename-cancel");

    // Track current deck state for patch operations
    var currentDeck = null;

    var editModal      = document.getElementById("edit-modal");
    var modalClose     = document.getElementById("modal-close");
    var btnModalCancel = document.getElementById("btn-modal-cancel");
    var btnModalSave   = document.getElementById("btn-modal-save");
    var btnDeleteCard  = document.getElementById("btn-delete-card");
    var editQuestion   = document.getElementById("edit-question");
    var editAnswer     = document.getElementById("edit-answer");
    var editTopic      = document.getElementById("edit-topic");
    var editSource     = document.getElementById("edit-source");
    var editError      = document.getElementById("edit-error");

    // ── Bulk selection ────────────────────────────────────────────────────────
    var btnSelectMode  = document.getElementById("btn-select-mode");
    var bulkBar        = document.getElementById("bulk-bar");
    var chkSelectAll   = document.getElementById("chk-select-all");
    var bulkCount      = document.getElementById("bulk-count");
    var btnBulkDelete  = document.getElementById("btn-bulk-delete");
    var btnBulkCancel  = document.getElementById("btn-bulk-cancel");

    var selectionMode  = false;   // are checkboxes visible?
    var selectedIDs    = {};      // id → true for checked cards

    function getEditType() {
        var radios = document.querySelectorAll('input[name="edit-type"]');
        for (var i = 0; i < radios.length; i++) {
            if (radios[i].checked) return radios[i].value;
        }
        return "conceito";
    }

    function setEditType(val) {
        var radios = document.querySelectorAll('input[name="edit-type"]');
        for (var i = 0; i < radios.length; i++) {
            radios[i].checked = (radios[i].value === val);
        }
    }

    var allCards    = [];
    var filtered    = [];
    var currentPage = 1;
    var editingID   = null;
    var currentUserID = null;
    var isAdmin       = false;
    var canEdit       = false; // set after deck loads

    function applyReadOnly() {
        // Hide all mutating controls when professor doesn't own the deck
        var hide = [btnRenameDeck, btnDeleteDeck, deleteZoneEl];
        hide.forEach(function (el) { if (el) el.style.display = "none"; });
        var notice = document.createElement("div");
        notice.className = "alert";
        notice.style.cssText = "background:#FFF3E0;border-color:#FFCC80;color:#E65100;margin-bottom:1rem";
        notice.textContent = "Este deck pertence a outro professor. Você pode visualizar os cards, mas não pode editá-los ou excluí-los.";
        if (deckContent) deckContent.insertBefore(notice, deckContent.firstChild);
    }

    /* ── Init ────────────────────────────────────── */
    function init() {
        if (!deckId) { window.location.href = "/teach.html"; return; }

        // Show deck name in topbar center immediately from URL param
        var topbarDeckName = document.getElementById("topbar-deck-name");
        if (topbarDeckName && deckName) { topbarDeckName.textContent = deckName; }

        app.checkAuth().then(function (user) {
            if (!user) { window.location.href = "/"; return; }
            var roles = user.roles || [];
            var ok = roles.indexOf("professor") >= 0 || roles.indexOf("admin") >= 0;
            if (!ok) { window.location.href = "/"; return; }
            isAdmin = roles.indexOf("admin") >= 0;
            currentUserID = user.user ? user.user.id : null;
            app.renderTopbar(user, { noNav: true });
            loadCards();
        });
    }

    /* ── Load cards ──────────────────────────────── */
    function loadCards() {
        // Fetch deck details and cards in parallel
        Promise.all([
            api.get("/api/content/decks/" + deckId).then(function (r) { return r.ok ? r.json() : null; }),
            api.get("/api/content/cards?deckId=" + deckId + "&limit=100")
                .then(function (res) { if (!res.ok) throw new Error(); return res.json(); })
        ])
            .then(function (results) {
                var deck = results[0];
                var page = results[1];
                spinnerEl.classList.add("hidden");
                deckContent.classList.remove("hidden");

                if (deck) {
                    currentDeck = deck;
                    renderStatusBadge(deck);
                    // Ownership: admins can edit any deck; professors only their own
                    canEdit = isAdmin || (deck.created_by && deck.created_by === currentUserID);
                    if (!canEdit) { applyReadOnly(); }
                }

                // Wire CSV export button
                if (btnExportCSV) {
                    btnExportCSV.href = '/api/content/decks/' + deckId + '/export.csv';
                }

                // Defensively handle both {items:[]} and unexpected shapes.
                // Go serializes nil slice as null; guard against that.
                var items = (page && Array.isArray(page.items)) ? page.items : [];
                allCards = items;

                deckTitle.textContent = deckName || "Deck";
                deckSubtitle.textContent = allCards.length + (allCards.length === 1 ? " card" : " cards");

                applyFilter();
            })
            .catch(function () {
                spinnerEl.classList.add("hidden");
                deckContent.classList.remove("hidden");
                deckTitle.textContent = deckName || "Deck";
                deckSubtitle.textContent = "";
                cardListEl.innerHTML =
                    '<div class="alert alert-error" style="margin:.5rem 0">' +
                    'Erro ao carregar cards. Verifique se o deck existe e se as migrações foram aplicadas.' +
                    '</div>';
            });
    }

    /* ── Search ──────────────────────────────────── */
    searchInput.addEventListener("input", debounce(function () {
        currentPage = 1;
        applyFilter();
    }, 250));

    function applyFilter() {
        var q = searchInput.value.toLowerCase().trim();
        if (!q) {
            filtered = allCards.slice();
        } else {
            // answer is not in the list payload; search question and topic only
            filtered = allCards.filter(function (c) {
                return c.question.toLowerCase().indexOf(q) >= 0 ||
                       (c.topic && c.topic.toLowerCase().indexOf(q) >= 0);
            });
        }
        renderPage();
    }

    /* ── Render ──────────────────────────────────── */
    function renderPage() {
        var total  = filtered.length;
        var pages  = Math.max(1, Math.ceil(total / PAGE_SIZE));
        currentPage = Math.min(currentPage, pages);
        var start  = (currentPage - 1) * PAGE_SIZE;
        var slice  = filtered.slice(start, start + PAGE_SIZE);

        if (total === 0) {
            cardListEl.innerHTML = "";
            emptyEl.classList.remove("hidden");
            // Show delete option only when deck truly has no cards (not just search filter)
            if (deleteZoneEl) {
                var noCards = allCards.length === 0;
                deleteZoneEl.classList.toggle("hidden", !noCards);
            }
            paginationEl.innerHTML = "";
            return;
        }
        emptyEl.classList.add("hidden");
        if (deleteZoneEl) deleteZoneEl.classList.add("hidden");

        var html = "";
        for (var i = 0; i < slice.length; i++) {
            html += renderCardItem(slice[i]);
        }
        cardListEl.innerHTML = html;

        /* edit buttons */
        var btns = cardListEl.querySelectorAll(".btn-edit");
        for (var j = 0; j < btns.length; j++) {
            btns[j].addEventListener("click", function () {
                openEditModal(this.getAttribute("data-id"));
            });
        }

        /* selection checkboxes */
        var chks = cardListEl.querySelectorAll(".card-item-chk");
        for (var k = 0; k < chks.length; k++) {
            chks[k].addEventListener("change", function () {
                var id = this.getAttribute("data-id");
                if (this.checked) {
                    selectedIDs[id] = true;
                } else {
                    delete selectedIDs[id];
                }
                updateBulkBar();
            });
        }

        renderPagination(pages);
    }

    function renderCardItem(c) {
        // answer intentionally omitted from list (data minimisation).
        // It is fetched when the edit modal opens.
        var checked = selectedIDs[c.id] ? " checked" : "";
        return '<div class="card-item' + (selectionMode ? " card-item--selectable" : "") + '"' +
                ' data-id="' + app.esc(c.id) + '">' +
            (selectionMode
                ? '<label class="card-item-check" aria-label="Selecionar card">' +
                    '<input type="checkbox" class="card-item-chk" data-id="' + app.esc(c.id) + '"' + checked + '>' +
                  '</label>'
                : '') +
            '<div class="card-item-body">' +
                '<div class="card-item-q">' + app.esc(c.question) + '</div>' +
                (c.topic ? '<div class="card-item-a">' + app.esc(c.topic) + '</div>' : '') +
                '<div class="card-item-meta">' +
                    '<span class="badge badge-type" data-type="' + app.esc(c.type) + '">' + app.esc(c.type) + '</span>' +
                    (c.topic ? '<span class="badge badge-topic">' + app.esc(c.topic) + '</span>' : '') +
                '</div>' +
            '</div>' +
            '<div class="card-item-actions">' +
                (!selectionMode && canEdit
                    ? '<button class="btn btn-outline btn-sm btn-edit" data-id="' + app.esc(c.id) + '"' +
                      ' aria-label="Editar card">Editar</button>'
                    : '') +
            '</div>' +
        '</div>';
    }

    /* ── Bulk selection helpers ──────────────────── */
    function enterSelectionMode() {
        selectionMode = true;
        selectedIDs = {};
        if (btnSelectMode) {
            btnSelectMode.classList.add("btn-primary");
            btnSelectMode.classList.remove("btn-outline");
        }
        renderPage();
        if (bulkBar) bulkBar.classList.remove("hidden");
        updateBulkBar();
    }

    function exitSelectionMode() {
        selectionMode = false;
        selectedIDs = {};
        if (btnSelectMode) {
            btnSelectMode.classList.remove("btn-primary");
            btnSelectMode.classList.add("btn-outline");
        }
        if (bulkBar) bulkBar.classList.add("hidden");
        if (chkSelectAll) chkSelectAll.checked = false;
        renderPage();
    }

    function updateBulkBar() {
        var count = Object.keys(selectedIDs).length;
        if (bulkCount) {
            bulkCount.textContent = count === 0
                ? "0 selecionados"
                : count === 1 ? "1 selecionado" : count + " selecionados";
        }
        if (btnBulkDelete) btnBulkDelete.disabled = count === 0;

        // Keep "select all" checkbox in sync.
        if (chkSelectAll && filtered.length > 0) {
            var pageIDs = getCurrentPageIDs();
            var allChecked = pageIDs.length > 0 && pageIDs.every(function (id) { return selectedIDs[id]; });
            chkSelectAll.indeterminate = !allChecked && count > 0;
            chkSelectAll.checked = allChecked;
        }
    }

    function getCurrentPageIDs() {
        var start = (currentPage - 1) * PAGE_SIZE;
        var slice = filtered.slice(start, start + PAGE_SIZE);
        return slice.map(function (c) { return c.id; });
    }

    if (btnSelectMode) {
        btnSelectMode.addEventListener("click", function () {
            if (selectionMode) exitSelectionMode();
            else enterSelectionMode();
        });
    }

    if (chkSelectAll) {
        chkSelectAll.addEventListener("change", function () {
            var ids = getCurrentPageIDs();
            ids.forEach(function (id) {
                if (chkSelectAll.checked) selectedIDs[id] = true;
                else delete selectedIDs[id];
            });
            // Sync individual checkboxes in the rendered list.
            cardListEl.querySelectorAll(".card-item-chk").forEach(function (chk) {
                chk.checked = !!selectedIDs[chk.getAttribute("data-id")];
            });
            updateBulkBar();
        });
    }

    if (btnBulkCancel) {
        btnBulkCancel.addEventListener("click", exitSelectionMode);
    }

    if (btnBulkDelete) {
        btnBulkDelete.addEventListener("click", function () {
            var ids = Object.keys(selectedIDs);
            if (!ids.length) return;
            var label = ids.length === 1 ? "1 card" : ids.length + " cards";
            if (!confirm("Excluir " + label + " permanentemente? Essa ação não pode ser desfeita.")) return;

            btnBulkDelete.disabled = true;
            btnBulkDelete.textContent = "Excluindo…";

            // api.del doesn't support a body; use fetch directly.
            fetch("/api/content/decks/" + deckId + "/cards", {
                method: "DELETE",
                credentials: "same-origin",
                headers: {
                    "Content-Type": "application/json",
                    "X-Requested-With": "XMLHttpRequest"
                },
                body: JSON.stringify({ ids: ids })
            })
                .then(function (res) {
                    if (!res.ok) throw new Error("falha");
                    toast(label + " excluído(s) com sucesso.", "success");
                    exitSelectionMode();
                    loadCards(); // reload full list
                })
                .catch(function () {
                    toast("Erro ao excluir cards selecionados.", "error");
                    btnBulkDelete.disabled = false;
                    btnBulkDelete.textContent = "Excluir selecionados";
                });
        });
    }

    function renderPagination(pages) {
        if (pages <= 1) { paginationEl.innerHTML = ""; return; }
        var html = "";
        for (var i = 1; i <= pages; i++) {
            html += '<button class="page-btn' + (i === currentPage ? " active" : "") + '" data-page="' + i + '">' + i + '</button>';
        }
        paginationEl.innerHTML = html;
        var pageBtns = paginationEl.querySelectorAll(".page-btn");
        for (var j = 0; j < pageBtns.length; j++) {
            pageBtns[j].addEventListener("click", function () {
                currentPage = parseInt(this.getAttribute("data-page"), 10);
                renderPage();
                window.scrollTo(0, 0);
            });
        }
    }

    /* ── Edit modal ──────────────────────────────── */
    function openEditModal(id) {
        // The list items don't carry answer/source (data minimisation).
        // Fetch the full card from the detail endpoint before opening the modal.
        editingID = null;
        editError.classList.add("hidden");
        editModal.classList.remove("hidden");

        // Show a temporary loading state inside the modal
        btnModalSave.disabled = true;
        editQuestion.value = "Carregando…";
        editAnswer.value   = "";
        editTopic.value    = "";
        editSource.value   = "";

        api.get("/api/content/cards/" + id)
            .then(function (res) {
                if (!res.ok) throw new Error("not found");
                return res.json();
            })
            .then(function (card) {
                editingID = card.id;
                setEditType(card.type || "conceito");
                editQuestion.value = card.question || "";
                editAnswer.value   = card.answer   || "";
                editTopic.value    = card.topic    || "";
                editSource.value   = card.source   || "";
                btnModalSave.disabled = false;
                // Refresh character counters now that values are populated.
                [editQuestion, editAnswer, editTopic, editSource].forEach(function (el) {
                    el.dispatchEvent(new Event("input"));
                });
            })
            .catch(function () {
                editError.textContent = "Erro ao carregar card.";
                editError.classList.remove("hidden");
                editQuestion.value = "";
            });
    }

    function closeModal() {
        editModal.classList.add("hidden");
        editingID = null;
    }

    modalClose.addEventListener("click", closeModal);
    btnModalCancel.addEventListener("click", closeModal);
    editModal.addEventListener("click", function (e) {
        if (e.target === editModal) closeModal();
    });

    btnModalSave.addEventListener("click", function () {
        if (!editingID) return;  // still loading
        var q = editQuestion.value.trim();
        var a = editAnswer.value.trim();
        if (!q || !a) {
            editError.textContent = "Pergunta e resposta são obrigatórias.";
            editError.classList.remove("hidden"); return;
        }
        editError.classList.add("hidden");
        btnModalSave.disabled = true;

        var body = {
            type:     getEditType(),
            question: q,
            answer:   a
        };
        var t = editTopic.value.trim();
        var s = editSource.value.trim();
        if (t) body.topic  = t;
        if (s) body.source = s;

        api.put("/api/content/cards/" + editingID, body)
            .then(function (res) {
                if (!res.ok) return res.json().then(function (e) { throw new Error(e.detail || "erro"); });
                // Update the in-memory list item (no answer stored locally)
                var card = findCard(editingID);
                if (card) {
                    card.type     = getEditType();
                    card.question = body.question;
                    card.topic    = body.topic || null;
                }
                closeModal();
                applyFilter();
                toast("Card atualizado!", "ok");
            })
            .catch(function (err) {
                editError.textContent = err.message || "Erro ao salvar.";
                editError.classList.remove("hidden");
            })
            .finally(function () { btnModalSave.disabled = false; });
    });

    btnDeleteCard.addEventListener("click", function () {
        if (!editingID) return;
        if (!confirm("Excluir este card? Essa ação não pode ser desfeita.")) return;
        btnDeleteCard.disabled = true;

        api.del("/api/content/cards/" + editingID)
            .then(function (res) {
                if (!res.ok) throw new Error();
                allCards = allCards.filter(function (c) { return c.id !== editingID; });
                deckSubtitle.textContent = allCards.length + (allCards.length === 1 ? " card" : " cards");
                closeModal();
                applyFilter();
                toast("Card excluído.", "ok");
            })
            .catch(function () { toast("Erro ao excluir card.", "error"); })
            .finally(function () { btnDeleteCard.disabled = false; });
    });

    /* ── Toggle label update ─────────────────────── */
    if (renameActiveEl) {
        renameActiveEl.addEventListener("change", function () {
            toggleActiveLbl.textContent = renameActiveEl.checked ? "Deck ativo" : "Deck inativo";
        });
    }
    if (btnClearExpiry) {
        btnClearExpiry.addEventListener("click", function () {
            renameExpiresEl.value = "";
        });
    }

    /* ── Edit deck ───────────────────────────────── */
    btnRenameDeck.addEventListener("click", function () {
        renameError.classList.add("hidden");
        renameNameEl.value      = deckName || "";
        renameSubjectEl.value   = "";
        renameDescEl.value      = "";
        renameActiveEl.checked  = true;
        renameExpiresEl.value   = "";
        toggleActiveLbl.textContent = "Deck ativo";

        // Fetch current deck info to prefill all fields
        api.get("/api/content/decks/" + deckId)
            .then(function (res) { return res.ok ? res.json() : null; })
            .then(function (deck) {
                if (!deck) return;
                currentDeck = deck;
                renameNameEl.value     = deck.name        || deckName || "";
                renameSubjectEl.value  = deck.subject     || "";
                renameDescEl.value     = deck.description || "";
                renameActiveEl.checked = deck.is_active !== false;
                toggleActiveLbl.textContent = renameActiveEl.checked ? "Deck ativo" : "Deck inativo";
                if (deck.expires_at) {
                    // datetime-local expects "YYYY-MM-DDTHH:mm" in local time
                    var d = new Date(deck.expires_at);
                    var pad = function (n) { return String(n).padStart(2, "0"); };
                    renameExpiresEl.value =
                        d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) +
                        "T" + pad(d.getHours()) + ":" + pad(d.getMinutes());
                }
                renderStatusBadge(deck);
            })
            .catch(function () { /* non-critical; keep deckName prefill */ });

        // Populate subjects datalist
        if (renameSubjectDl && renameSubjectDl.options.length === 0) {
            api.get("/api/decks?limit=100")
                .then(function (r) { return r.ok ? r.json() : null; })
                .then(function (page) {
                    if (!page) return;
                    var items = page.items || page || [];
                    var seen = {};
                    items.forEach(function (d) {
                        if (d.subject && !seen[d.subject]) {
                            seen[d.subject] = true;
                            var o = document.createElement("option");
                            o.value = d.subject;
                            renameSubjectDl.appendChild(o);
                        }
                    });
                })
                .catch(function () {});
        }

        renameForm.classList.remove("hidden");
        renameNameEl.focus();
    });

    btnRenameCancel.addEventListener("click", function () {
        renameForm.classList.add("hidden");
    });

    btnRenameSave.addEventListener("click", function () {
        var newName = renameNameEl.value.trim();
        if (!newName) {
            renameError.textContent = "O nome do deck não pode estar vazio.";
            renameError.classList.remove("hidden");
            return;
        }
        renameError.classList.add("hidden");
        btnRenameSave.disabled = true;

        // PUT for name / subject / description
        var putBody = { name: newName };
        var subj = renameSubjectEl.value.trim();
        putBody.subject = subj || null;
        var desc = renameDescEl.value.trim();
        if (desc) putBody.description = desc;

        // PATCH for is_active / expires_at
        var patchBody = { is_active: renameActiveEl.checked };
        var expiresVal = renameExpiresEl.value;
        if (expiresVal) {
            // datetime-local gives local time; convert to ISO string with timezone
            patchBody.expires_at = new Date(expiresVal).toISOString();
        } else {
            patchBody.expires_at = ""; // clear
        }

        Promise.all([
            api.put("/api/content/decks/" + deckId, putBody)
                .then(function (r) { return r.ok ? r.json() : r.json().then(function (e) { throw new Error(e.detail || "erro ao salvar"); }); }),
            api.patch("/api/content/decks/" + deckId, patchBody)
                .then(function (r) { return r.ok ? r.json() : r.json().then(function (e) { throw new Error(e.detail || "erro ao salvar"); }); })
        ])
        .then(function (results) {
            var deck = results[1]; // PATCH returns updated deck with is_active/expires_at
            deck.name = results[0].name; // merge name from PUT
            currentDeck = deck;
            deckName = deck.name;
            deckTitle.textContent = deck.name;
            var topbarDeckName = document.getElementById("topbar-deck-name");
            if (topbarDeckName) topbarDeckName.textContent = deck.name;
            renderStatusBadge(deck);
            renameForm.classList.add("hidden");
            // Keep the URL in sync with the new deck name so a refresh loads correctly.
            var url = new URL(window.location.href);
            url.searchParams.set("deckName", deck.name);
            window.history.replaceState(null, "", url.toString());
            toast("Deck atualizado!", "ok");
        })
        .catch(function (err) {
            renameError.textContent = err.message || "Erro ao salvar.";
            renameError.classList.remove("hidden");
        })
        .finally(function () { btnRenameSave.disabled = false; });
    });

    /* ── Delete deck ─────────────────────────────── */
    if (btnDeleteDeck) {
        btnDeleteDeck.addEventListener("click", function () {
            if (!confirm('Tem certeza que deseja excluir o deck "' + (deckName || "este deck") + '"?\nEsta ação não pode ser desfeita.')) return;
            btnDeleteDeck.disabled = true;
            btnDeleteDeck.textContent = "Excluindo…";
            api.del("/api/content/decks/" + deckId)
                .then(function (res) {
                    if (!res.ok) return res.json().then(function (e) { throw new Error(e.detail || "erro"); });
                    toast("Deck excluído com sucesso!", "ok");
                    setTimeout(function () { window.location.href = "/teach.html"; }, 800);
                })
                .catch(function (err) {
                    toast(err.message || "Erro ao excluir deck.", "error");
                    btnDeleteDeck.disabled = false;
                    btnDeleteDeck.textContent = "Excluir deck";
                });
        });
    }

    /* ── Status badge + publish button ──────────── */
    var btnPublishDeck = document.getElementById("btn-publish-deck");

    function renderStatusBadge(deck) {
        if (!deckStatusBadge) return;
        var isActive   = deck.is_active !== false;
        var isExpired  = deck.expires_at && new Date(deck.expires_at) <= new Date();
        if (!isActive) {
            deckStatusBadge.innerHTML = '<span class="badge badge-draft">Rascunho</span>';
        } else if (isExpired) {
            deckStatusBadge.innerHTML = '<span class="badge badge-expired">Expirado</span>';
        } else if (deck.expires_at) {
            var d = new Date(deck.expires_at);
            deckStatusBadge.innerHTML = '<span class="badge badge-expiring" title="Expira em ' +
                d.toLocaleDateString("pt-BR") + '">Expira ' + d.toLocaleDateString("pt-BR") + '</span>';
        } else {
            deckStatusBadge.innerHTML = '<span class="badge badge-green">Publicado</span>';
        }

        // Show/update publish toggle button
        if (btnPublishDeck && !isExpired) {
            if (isActive) {
                btnPublishDeck.className = "btn btn-sm btn-outline";
                btnPublishDeck.style.display = "inline-flex";
                btnPublishDeck.innerHTML =
                    '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M17 1l4 4-4 4M3 11V9a4 4 0 0 1 4-4h11M7 23l-4-4 4-4M21 13v2a4 4 0 0 1-4 4H6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                    'Despublicar';
                btnPublishDeck.dataset.publish = '0';
            } else {
                btnPublishDeck.className = "btn btn-sm btn-primary";
                btnPublishDeck.style.display = "inline-flex";
                btnPublishDeck.innerHTML =
                    '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M5 12l5 5L20 7" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
                    'Publicar';
                btnPublishDeck.dataset.publish = '1';
            }
        }
    }

    if (btnPublishDeck) {
        btnPublishDeck.addEventListener('click', function () {
            if (!currentDeck) return;
            var publish = btnPublishDeck.dataset.publish === '1';
            btnPublishDeck.disabled = true;
            api.patch('/api/content/decks/' + currentDeck.id, { is_active: publish })
                .then(function (res) { return res.ok ? res.json() : Promise.reject(); })
                .then(function (updated) {
                    currentDeck = updated;
                    renderStatusBadge(updated);
                    toast(publish ? 'Deck publicado! Alunos já podem acessá-lo.' : 'Deck movido para rascunho.', 'ok');
                })
                .catch(function () {
                    toast('Erro ao atualizar deck.', 'error');
                })
                .finally(function () { btnPublishDeck.disabled = false; });
        });
    }

    /* ── Helpers ─────────────────────────────────── */
    function findCard(id) {
        for (var i = 0; i < allCards.length; i++) {
            if (allCards[i].id === id) return allCards[i];
        }
        return null;
    }

    function debounce(fn, ms) {
        var t;
        return function () {
            clearTimeout(t);
            t = setTimeout(fn, ms);
        };
    }

    init();
})();
