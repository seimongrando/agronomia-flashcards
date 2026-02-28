'use strict';

(function () {
    var spinner    = document.getElementById('spinner');
    var content    = document.getElementById('stats-content');
    var overviewEl = document.getElementById('overview-grid');
    var deckTbody  = document.getElementById('deck-tbody');
    var noDeckMsg  = document.getElementById('no-decks-msg');
    var hardList   = document.getElementById('hard-cards-list');
    var noHardMsg  = document.getElementById('no-hard-msg');

    // ── Auth check ───────────────────────────────────────────────────────────
    // app.checkAuth() returns the parsed JSON ({user, roles}) or null.
    app.checkAuth().then(function (me) {
        if (!me || !me.user) {
            window.location.href = '/?error=auth';
            return;
        }

        var roles = Array.isArray(me.roles) ? me.roles : [];
        if (roles.indexOf('professor') === -1 && roles.indexOf('admin') === -1) {
            window.location.href = '/';
            return;
        }

        if (window.app && window.app.renderTopbar) {
            window.app.renderTopbar(me);
        }

        // ── Fetch stats ──────────────────────────────────────────────────────
        window.api.get('/api/admin/stats')
            .then(function (res) {
                if (!res.ok) throw new Error('stats ' + res.status);
                return res.json();
            })
            .then(function (stats) {
                spinner.classList.add('hidden');
                content.classList.remove('hidden');
                renderOverview(stats);
                renderDeckTable(stats.decks || []);
                renderHardCards(stats.hardest_cards || []);
            })
            .catch(function () {
                spinner.innerHTML = '<p class="text-muted" style="padding:1rem 0">Erro ao carregar dados do painel.</p>';
            });

    }).catch(function () {
        window.location.href = '/';
    });

    // ── Renderers ────────────────────────────────────────────────────────────

    function renderOverview(stats) {
        var items = [
            { value: stats.total_decks,     label: 'Decks',         sub: (stats.active_decks || 0) + ' ativos' },
            { value: stats.total_cards,     label: 'Cartas',        sub: 'total de conteúdo'                   },
            { value: stats.active_students, label: 'Alunos Ativos', sub: 'últimos 30 dias'                     },
            { value: stats.total_reviews,   label: 'Revisões',      sub: 'acumulado'                           },
        ];
        overviewEl.innerHTML = items.map(function (o) {
            return '<div class="prog-stat" role="listitem">' +
                '<span class="prog-stat__value">' + (o.value || 0) + '</span>' +
                '<span class="prog-stat__label">' + o.label + '</span>' +
                '<span class="prog-stat__sub">'   + o.sub   + '</span>' +
                '</div>';
        }).join('');
    }

    function renderDeckTable(decks) {
        if (decks.length === 0) {
            noDeckMsg.classList.remove('hidden');
            return;
        }
        deckTbody.innerHTML = decks.map(function (d) {
            var statusBadge = d.is_active
                ? '<span class="badge badge-green" style="font-size:.75rem">Ativo</span>'
                : '<span class="badge badge-inactive" style="font-size:.75rem">Inativo</span>';

            var accuracyBar = d.total_reviews > 0
                ? '<div style="display:flex;align-items:center;gap:.4rem">' +
                    '<div style="flex:1;height:6px;background:var(--gray-100);border-radius:3px;overflow:hidden">' +
                    '<div style="width:' + d.avg_accuracy + '%;height:100%;background:' + accuracyColor(d.avg_accuracy) + ';border-radius:3px"></div>' +
                    '</div>' +
                    '<span style="font-size:.8rem;color:var(--gray-500);white-space:nowrap">' + d.avg_accuracy + '%</span>' +
                    '</div>'
                : '<span style="color:var(--gray-400);font-size:.85rem">—</span>';

            return '<tr>' +
                '<td><a href="/deck_manage.html?deckId=' + d.id + '&deckName=' + encodeURIComponent(d.name) +
                    '" class="link-green" style="font-weight:500">' + esc(d.name) + '</a></td>' +
                '<td style="color:var(--gray-500);font-size:.85rem">' + (d.subject ? esc(d.subject) : '—') + '</td>' +
                '<td>' + statusBadge + '</td>' +
                '<td class="num">' + d.total_cards + '</td>' +
                '<td class="num">' + d.students_studying + '</td>' +
                '<td class="num">' + d.total_reviews + '</td>' +
                '<td style="min-width:110px">' + accuracyBar + '</td>' +
                '<td><a href="/api/content/decks/' + d.id + '/export.csv" class="btn btn-outline btn-sm" ' +
                    'style="white-space:nowrap" title="Baixar CSV">↓ CSV</a></td>' +
                '</tr>';
        }).join('');
    }

    function renderHardCards(hard) {
        if (hard.length === 0) {
            noHardMsg.classList.remove('hidden');
            return;
        }
        var typeMap = {
            conceito:  'badge-conceito',
            processo:  'badge-processo',
            aplicacao: 'badge-aplicacao',
            comparacao:'badge-comparacao',
        };
        hardList.innerHTML = hard.map(function (c, i) {
            var type    = c.type || '';
            var typeCls = typeMap[type] || '';
            return '<div class="hard-card-row">' +
                '<div class="hard-card-rank">' + (i + 1) + '</div>' +
                '<div class="hard-card-body">' +
                    '<div class="hard-card-q">' + esc(trunc(c.question, 120)) + '</div>' +
                    '<div class="hard-card-meta">' +
                        '<span class="badge ' + typeCls + '" style="font-size:.72rem;text-transform:uppercase">' + type + '</span>' +
                        '<span class="hard-card-deck">' + esc(c.deck_name) + '</span>' +
                        '<span style="color:var(--gray-400);font-size:.8rem">' + c.total_reviews + ' revisões</span>' +
                    '</div>' +
                '</div>' +
                '<div class="hard-card-accuracy" style="color:' + accuracyColor(c.accuracy) + '">' +
                    c.accuracy + '%' +
                '</div>' +
            '</div>';
        }).join('');
    }

    // ── Helpers ──────────────────────────────────────────────────────────────

    function accuracyColor(pct) {
        if (pct >= 70) return 'var(--green-main)';
        if (pct >= 40) return '#F57C00';
        return '#c62828';
    }

    function esc(s) {
        return String(s || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function trunc(s, n) {
        return s && s.length > n ? s.slice(0, n) + '…' : s;
    }
})();
