(function () {
    "use strict";

    var spinnerEl  = document.getElementById("spinner");
    var contentEl  = document.getElementById("progress-content");

    var statMastered = document.getElementById("stat-mastered");
    var statLearning = document.getElementById("stat-learning");
    var statDue      = document.getElementById("stat-due");
    var statDays     = document.getElementById("stat-days");
    var statStreak   = document.getElementById("stat-streak");
    var statLongest  = document.getElementById("stat-longest");
    var streakBanner = document.getElementById("streak-banner");

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
        statMastered.textContent = d.mastered      || 0;
        statLearning.textContent = d.learning      || 0;
        statDue.textContent      = d.due_today     || 0;
        statDays.textContent     = d.study_days    || 0;
        statStreak.textContent   = d.study_streak  || 0;
        statLongest.textContent  = d.longest_streak || 0;

        // Streak motivational banner
        var streak = d.study_streak || 0;
        if (streak >= 2) {
            var emoji = streak >= 30 ? "🌳" : streak >= 14 ? "🌿" : streak >= 7 ? "🌱" : "✨";
            var msg = streak === 1
                ? "Bom começo — estude amanhã para iniciar uma sequência!"
                : streak + " dias seguidos de estudo. " + emoji + " Continue assim!";
            streakBanner.textContent = msg;
            streakBanner.classList.remove("hidden");
        }

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
            // Use explicit Number() to guarantee 0 is rendered, never undefined/null/empty
            var total   = Number(dk.total_cards)  || 0;
            var mastered = Number(dk.mastered)     || 0;
            var learning = Number(dk.learning)     || 0;
            var dueNow   = Number(dk.due_now)      || 0;
            var hard     = Number(dk.hard)         || 0;
            var wrong    = Number(dk.wrong)        || 0;
            html += '<tr>' +
                '<td class="deck-name-cell">' +
                    '<a href="/study.html?deckId=' + app.esc(dk.id) + '&mode=due&deckName=' + encodeURIComponent(dk.name) + '">' +
                        app.esc(dk.name) +
                    '</a>' +
                    '<div class="prog-mini-bar" title="' + masteredPct + '% dominadas">' +
                        '<div class="prog-mini-fill" style="width:' + masteredPct + '%"></div>' +
                    '</div>' +
                '</td>' +
                '<td class="num">' + total + '</td>' +
                '<td class="num prog-cell--mastered">' + mastered + '</td>' +
                '<td class="num">' + learning + '</td>' +
                '<td class="num' + (dueNow > 0 ? ' prog-cell--due'   : '') + '">' + dueNow + '</td>' +
                '<td class="num' + (hard   > 0 ? ' prog-cell--hard'  : '') + '">' + hard   + '</td>' +
                '<td class="num' + (wrong  > 0 ? ' prog-cell--wrong' : '') + '">' + wrong  + '</td>' +
            '</tr>';
        }
        deckTableBody.innerHTML = html;
    }

    init();
})();
