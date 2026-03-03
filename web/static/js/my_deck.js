(function () {
    "use strict";

    var params  = new URLSearchParams(window.location.search);
    var deckId  = params.get("deckId");

    var spinnerEl    = document.getElementById("spinner");
    var deniedEl     = document.getElementById("access-denied");
    var pageContent  = document.getElementById("page-content");
    var deckTitle    = document.getElementById("deck-title");
    var cardCountTxt = document.getElementById("card-count-text");
    var btnStudy     = document.getElementById("btn-study");

    var inpQuestion  = document.getElementById("inp-question");
    var inpAnswer    = document.getElementById("inp-answer");
    var typePicker   = document.getElementById("type-picker");
    var btnSave      = document.getElementById("btn-save-card");
    var btnCancel    = document.getElementById("btn-cancel-edit");
    var formTitle    = document.getElementById("form-title");
    var formError    = document.getElementById("form-error");
    var cardListEl   = document.getElementById("card-list");

    var editingCardId = null; // null = adding new; string = editing existing

    var typeLabels = {
        conceito:   "Conceito",
        processo:   "Processo",
        aplicacao:  "Aplicação",
        comparacao: "Comparação"
    };

    var typeColors = {
        conceito:   { bg: "#E3F2FD", color: "#1565C0", border: "#90CAF9" },
        processo:   { bg: "#FFF8E1", color: "#E65100", border: "#FFCC80" },
        aplicacao:  { bg: "#E8F5E9", color: "#1B5E20", border: "#A5D6A7" },
        comparacao: { bg: "#F3E5F5", color: "#6A1B9A", border: "#CE93D8" }
    };

    function init() {
        if (!deckId) { window.location.href = "/classes.html"; return; }

        app.checkAuth().then(function (user) {
            if (!user) { window.location.href = "/"; return; }
            app.renderTopbar(user, { backHref: "/my_decks.html" });
            var back = document.getElementById("topbar-back");
            if (back) back.classList.remove("hidden");

            loadCards();
            wireForm();
        });
    }

    function loadCards() {
        api.get("/api/my/decks/" + deckId + "/cards")
            .then(function (r) {
                if (!r.ok) {
                    spinnerEl.classList.add("hidden");
                    if (deniedEl) deniedEl.classList.remove("hidden");
                    return null;
                }
                return r.json();
            })
            .then(function (data) {
                if (!data) return;
                spinnerEl.classList.add("hidden");
                pageContent.classList.remove("hidden");

                var items = data.items || [];
                renderCards(items);

                // Set deck title from first card's deck_id — fetch deck name via cards page
                // (we already know deckId; title comes from classes.html link params)
                var deckName = params.get("deckName");
                if (deckTitle && deckName) deckTitle.textContent = decodeURIComponent(deckName);
                if (btnStudy) btnStudy.href = "/study.html?deckId=" + deckId + "&mode=random";
            })
            .catch(function () {
                spinnerEl.classList.add("hidden");
                if (deniedEl) deniedEl.classList.remove("hidden");
            });
    }

    function renderCards(cards) {
        if (cardCountTxt) cardCountTxt.textContent = cards.length + " card(s)";
        if (!cardListEl) return;

        if (cards.length === 0) {
            cardListEl.innerHTML = '<p class="text-muted" style="padding:.5rem 0">Nenhum card ainda. Adicione o primeiro acima!</p>';
            return;
        }

        cardListEl.innerHTML = cards.map(function (c) {
            var type    = c.type || "conceito";
            var typeLbl = typeLabels[type] || type;
            return '<div class="my-card-item" data-id="' + c.id + '" data-type="' + type + '">' +
                '<div class="my-card-item__top">' +
                    '<span class="my-card-item__type badge-type" data-type="' + type + '" ' +
                        'style="color:#fff;font-size:.7rem;font-weight:700;padding:.2rem .6rem;border-radius:999px;background:var(--type-' + type + ')">' +
                        typeLbl +
                    '</span>' +
                    '<div class="my-card-item__actions">' +
                        '<button class="btn btn-ghost btn-sm btn-edit-card" data-id="' + c.id + '"' +
                            ' title="Editar card">' +
                            '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                            '<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                            '<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                            '</svg>' +
                        '</button>' +
                        '<button class="btn btn-ghost btn-sm btn-del-card" data-id="' + c.id + '"' +
                            ' title="Excluir card" style="color:var(--danger)">' +
                            '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                            '<path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                            '</svg>' +
                        '</button>' +
                    '</div>' +
                '</div>' +
                '<div class="my-card-item__q">' + app.esc(c.question) + '</div>' +
                '<div class="my-card-item__a">' + app.esc(c.answer) + '</div>' +
            '</div>';
        }).join('');

        // Edit buttons — populate form
        cardListEl.querySelectorAll(".btn-edit-card").forEach(function (btn) {
            btn.addEventListener("click", function () {
                var row  = btn.closest(".my-card-item");
                var id   = btn.dataset.id;
                var q    = row.querySelector(".my-card-item__q").textContent;
                var a    = row.querySelector(".my-card-item__a").textContent;
                var type = row.dataset.type || "conceito";
                startEdit(id, q, a, type);
            });
        });

        // Delete buttons
        cardListEl.querySelectorAll(".btn-del-card").forEach(function (btn) {
            btn.addEventListener("click", function () {
                if (!confirm("Excluir este card?")) return;
                api.del("/api/my/cards/" + btn.dataset.id)
                    .then(function (r) {
                        if (!r.ok) throw new Error("Erro ao excluir card");
                        toast("Card excluído.", "success");
                        var row = btn.closest(".my-card-item");
                        if (row) row.remove();
                        updateCount();
                        // If editing this card, reset form
                        if (editingCardId === btn.dataset.id) resetForm();
                    })
                    .catch(function (e) { toast(e.message, "error"); });
            });
        });
    }

    function updateCount() {
        var count = cardListEl ? cardListEl.querySelectorAll(".my-card-item").length : 0;
        if (cardCountTxt) cardCountTxt.textContent = count + " card(s)";
    }

    function getSelectedType() {
        if (!typePicker) return "conceito";
        var checked = typePicker.querySelector('input[name="my-card-type"]:checked');
        return checked ? checked.value : "conceito";
    }

    function setSelectedType(type) {
        if (!typePicker) return;
        var radio = typePicker.querySelector('input[value="' + type + '"]');
        if (radio) radio.checked = true;
    }

    function startEdit(id, question, answer, type) {
        editingCardId = id;
        if (inpQuestion) { inpQuestion.value = question; inpQuestion.dispatchEvent(new Event("input")); }
        if (inpAnswer)   { inpAnswer.value   = answer;   inpAnswer.dispatchEvent(new Event("input")); }
        setSelectedType(type);
        if (formTitle)   formTitle.textContent = "Editar card";
        if (btnSave)     btnSave.textContent   = "Salvar alterações";
        if (btnCancel)   btnCancel.classList.remove("hidden");
        hideError();
        var section = document.getElementById("add-card-section");
        if (section) section.scrollIntoView({ behavior: "smooth", block: "start" });
    }

    function resetForm() {
        editingCardId = null;
        if (inpQuestion) { inpQuestion.value = ""; inpQuestion.dispatchEvent(new Event("input")); }
        if (inpAnswer)   { inpAnswer.value   = ""; inpAnswer.dispatchEvent(new Event("input")); }
        setSelectedType("conceito");
        if (formTitle)   formTitle.textContent = "Adicionar card";
        if (btnSave)     btnSave.textContent   = "Salvar card";
        if (btnCancel)   btnCancel.classList.add("hidden");
        hideError();
    }

    function showError(msg) {
        if (formError) {
            formError.textContent = msg;
            formError.classList.remove("hidden");
        }
    }

    function hideError() {
        if (formError) formError.classList.add("hidden");
    }

    function wireForm() {
        if (btnCancel) btnCancel.addEventListener("click", resetForm);

        if (btnSave) btnSave.addEventListener("click", function () {
            var question = inpQuestion ? inpQuestion.value.trim() : "";
            var answer   = inpAnswer   ? inpAnswer.value.trim()   : "";
            var type     = getSelectedType();

            hideError();
            if (!question) { showError("A pergunta é obrigatória."); return; }
            if (!answer)   { showError("A resposta é obrigatória."); return; }

            btnSave.disabled = true;

            var payload = { question: question, answer: answer, type: type };

            var req;
            if (editingCardId) {
                req = api.put("/api/my/cards/" + editingCardId, payload)
                    .then(function (r) {
                        if (!r.ok) return r.json().then(function (e) { throw new Error(e.detail || "Erro ao salvar"); });
                        toast("Card atualizado.", "success");
                        resetForm();
                        loadCards();
                    });
            } else {
                req = api.post("/api/my/decks/" + deckId + "/cards", payload)
                    .then(function (r) {
                        if (!r.ok) return r.json().then(function (e) { throw new Error(e.detail || "Erro ao criar card"); });
                        toast("Card adicionado!", "success");
                        resetForm();
                        loadCards();
                    });
            }

            req.catch(function (e) { showError(e.message); })
               .finally(function () { btnSave.disabled = false; });
        });
    }

    init();
})();
