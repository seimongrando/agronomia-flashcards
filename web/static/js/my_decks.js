(function () {
    "use strict";

    var spinnerEl  = document.getElementById("spinner");
    var pageEl     = document.getElementById("page-content");
    var deckList   = document.getElementById("deck-list");
    var btnNew     = document.getElementById("btn-new-deck");
    var createForm = document.getElementById("create-form");
    var nameInput  = document.getElementById("deck-name-input");
    var btnCreate  = document.getElementById("btn-create");
    var btnCancel  = document.getElementById("btn-cancel");
    var createErr  = document.getElementById("create-error");

    function init() {
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) { window.location.href = "/?error=unauthorized"; return; }
            // Only students use this page; redirect staff to manage
            app.renderTopbar(user);
            pageEl.classList.remove("hidden");
            loadDecks();
            wireForm();
        });
    }

    function loadDecks() {
        deckList.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Carregando…</p>';
        api.get("/api/my/decks")
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) { renderDecks(data && data.items ? data.items : []); })
            .catch(function () { deckList.innerHTML = '<p style="color:var(--danger)">Erro ao carregar decks.</p>'; });
    }

    function renderDecks(decks) {
        if (decks.length === 0) {
            deckList.innerHTML =
                '<div class="my-deck-empty">' +
                    '<svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="opacity:.25">' +
                    '<rect x="3" y="11" width="18" height="11" rx="2" stroke="currentColor" stroke-width="1.5"/>' +
                    '<path d="M7 11V7a5 5 0 0 1 10 0v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>' +
                    '</svg>' +
                    '<p style="margin:.5rem 0 .2rem;font-weight:600">Nenhum deck pessoal ainda</p>' +
                    '<p class="text-muted" style="font-size:.85rem">Crie seu primeiro deck para começar a estudar!</p>' +
                '</div>';
            return;
        }

        deckList.innerHTML = decks.map(function (d) {
            var isEmpty  = d.total_cards === 0;
            var cardLabel = isEmpty ? '⚠ Sem cards — adicione perguntas' : '📋 ' + d.total_cards + (d.total_cards === 1 ? ' carta' : ' cartas');
            return '<div class="my-deck-item-card">' +
                '<div class="my-deck-item-card__body">' +
                    '<div class="my-deck-item-card__name">' + app.esc(d.name) + '</div>' +
                    '<div class="my-deck-item-card__count' + (isEmpty ? ' my-deck-item-card__count--empty' : '') + '">' +
                        cardLabel +
                    '</div>' +
                '</div>' +
                '<div class="my-deck-item-card__actions">' +
                    '<a href="/my_deck.html?deckId=' + d.id + '&deckName=' + encodeURIComponent(d.name) + '"' +
                       ' class="btn btn-sm btn-primary">Gerenciar cards</a>' +
                    (!isEmpty
                        ? '<a href="/study.html?deckId=' + d.id + '&mode=random&deckName=' + encodeURIComponent(d.name) + '"' +
                          ' class="btn btn-sm btn-outline">Estudar</a>'
                        : '') +
                    '<button class="btn btn-sm btn-ghost btn-del"' +
                        ' data-id="' + d.id + '" data-name="' + app.esc(d.name) + '"' +
                        ' title="Excluir deck" style="color:var(--danger);padding:.3rem .45rem">' +
                        '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                        '<polyline points="3 6 5 6 21 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                        '<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"' +
                        ' stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                        '</svg>' +
                    '</button>' +
                '</div>' +
            '</div>';
        }).join('');

        deckList.querySelectorAll(".btn-del").forEach(function (btn) {
            btn.addEventListener("click", function () {
                if (!confirm('Excluir deck "' + btn.dataset.name + '"?\nTodos os cards serão removidos. Esta ação não pode ser desfeita.')) return;
                api.del("/api/my/decks/" + btn.dataset.id)
                    .then(function (r) {
                        if (!r.ok) throw new Error("Erro ao excluir deck");
                        loadDecks();
                        toast("Deck excluído.", "success");
                    })
                    .catch(function (e) { toast(e.message, "error"); });
            });
        });
    }

    function wireForm() {
        if (btnNew) btnNew.addEventListener("click", function () {
            createForm.classList.remove("hidden");
            hideError();
            if (nameInput) nameInput.focus();
        });

        if (btnCancel) btnCancel.addEventListener("click", function () {
            createForm.classList.add("hidden");
            if (nameInput) nameInput.value = "";
            hideError();
        });

        if (btnCreate) btnCreate.addEventListener("click", createDeck);
        if (nameInput) nameInput.addEventListener("keydown", function (e) {
            if (e.key === "Enter") createDeck();
        });
    }

    function createDeck() {
        hideError();
        var name = (nameInput ? nameInput.value : "").trim();
        if (!name) { showError("Informe o nome do deck."); return; }
        btnCreate.disabled = true;
        api.post("/api/my/decks", { name: name })
            .then(function (r) {
                if (r.status === 409) return r.json().then(function (e) { throw new Error(e.detail || "Já existe um deck com este nome."); });
                if (!r.ok) throw new Error("Erro ao criar deck.");
                return r.json();
            })
            .then(function (deck) {
                // Navigate to card management for this new deck so user adds cards immediately
                window.location.href = "/my_deck.html?deckId=" + deck.id + "&deckName=" + encodeURIComponent(deck.name) + "&created=1";
            })
            .catch(function (e) { showError(e.message); })
            .finally(function () { btnCreate.disabled = false; });
    }

    function showError(msg) {
        if (!createErr) return;
        createErr.textContent = msg;
        createErr.classList.remove("hidden");
    }

    function hideError() {
        if (createErr) createErr.classList.add("hidden");
    }

    init();
})();
