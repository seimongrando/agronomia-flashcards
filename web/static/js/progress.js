(function () {
    "use strict";

    var spinnerEl  = document.getElementById("spinner");
    var contentEl  = document.getElementById("progress-content");

    var statMastered = document.getElementById("stat-mastered");
    var statLearning = document.getElementById("stat-learning");
    var statDue      = document.getElementById("stat-due");
    var statDays     = document.getElementById("stat-days");

    var accuracyPct   = document.getElementById("accuracy-pct");
    var accuracyFill  = document.getElementById("accuracy-fill");
    var accuracyTrack = document.getElementById("accuracy-track");
    var accuracyHint  = document.getElementById("accuracy-hint");

    var deckTableBody = document.getElementById("deck-table-body");
    var noDecks       = document.getElementById("no-decks");

    function init() {
        app.checkAuth().then(function (user) {
            if (!user) { window.location.href = "/"; return; }
            app.renderTopbar(user);
            load();
        });
    }

    function load() {
        api.get("/api/progress")
            .then(function (res) {
                if (!res.ok) throw new Error("failed");
                return res.json();
            })
            .then(function (data) {
                spinnerEl.classList.add("hidden");
                contentEl.classList.remove("hidden");
                render(data);
            })
            .catch(function () {
                spinnerEl.classList.add("hidden");
                contentEl.classList.remove("hidden");
                contentEl.innerHTML = '<p class="alert alert-error">Erro ao carregar progresso.</p>';
            });
    }

    function render(d) {
        statMastered.textContent = d.mastered   || 0;
        statLearning.textContent = d.learning   || 0;
        statDue.textContent      = d.due_today  || 0;
        statDays.textContent     = d.study_days || 0;

        var pct = d.accuracy_7d || 0;
        accuracyPct.textContent = pct + "%";
        accuracyFill.style.width = pct + "%";
        accuracyTrack.setAttribute("aria-valuenow", pct);
        accuracyFill.className = "prog-accuracy__fill" +
            (pct >= 80 ? " prog-accuracy__fill--good" :
             pct >= 50 ? " prog-accuracy__fill--ok" : " prog-accuracy__fill--low");

        var hint = d.total_studied === 0
            ? "Você ainda não estudou nenhum card."
            : pct >= 80 ? "Ótima precisão! Continue assim."
            : pct >= 50 ? "Boa base — foque nas cartas vencidas para melhorar."
            : "Muitos erros recentes — revise os fundamentos dos decks com mais dificuldade.";
        accuracyHint.textContent = hint;

        var decks = d.decks || [];
        if (decks.length === 0) {
            noDecks.classList.remove("hidden");
            document.getElementById("deck-table-wrap").classList.add("hidden");
            return;
        }

        var html = "";
        for (var i = 0; i < decks.length; i++) {
            var dk = decks[i];
            var masteredPct = dk.total_cards > 0 ? Math.round(dk.mastered * 100 / dk.total_cards) : 0;
            html += '<tr>' +
                '<td class="deck-name-cell">' +
                    '<a href="/study.html?deckId=' + app.esc(dk.id) + '&mode=due&deckName=' + encodeURIComponent(dk.name) + '">' +
                        app.esc(dk.name) +
                    '</a>' +
                    '<div class="prog-mini-bar" title="' + masteredPct + '% dominadas">' +
                        '<div class="prog-mini-fill" style="width:' + masteredPct + '%"></div>' +
                    '</div>' +
                '</td>' +
                '<td class="num">' + dk.total_cards + '</td>' +
                '<td class="num prog-cell--mastered">' + dk.mastered + '</td>' +
                '<td class="num">' + dk.learning + '</td>' +
                '<td class="num' + (dk.due_now > 0 ? ' prog-cell--due' : '') + '">' + dk.due_now + '</td>' +
            '</tr>';
        }
        deckTableBody.innerHTML = html;
    }

    init();
})();
