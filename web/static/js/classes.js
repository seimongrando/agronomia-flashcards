(function () {
    "use strict";

    var spinnerEl    = document.getElementById("spinner");
    var pageContent  = document.getElementById("page-content");
    var staffView    = document.getElementById("staff-view");
    var studentView  = document.getElementById("student-view");

    // Staff refs
    var btnNewClass    = document.getElementById("btn-new-class");
    var newClassForm   = document.getElementById("new-class-form");
    var newClassName   = document.getElementById("new-class-name");
    var newClassDesc   = document.getElementById("new-class-desc");
    var btnCreateClass = document.getElementById("btn-create-class");
    var btnCancelClass = document.getElementById("btn-cancel-class");
    var staffClassList = document.getElementById("staff-class-list");

    // Student refs
    var inviteInput      = document.getElementById("invite-input");
    var btnJoin          = document.getElementById("btn-join");
    var joinError        = document.getElementById("join-error");
    var studentClassList = document.getElementById("student-class-list");

    var isStaff = false;

    function init() {
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) { window.location.href = "/?error=unauthorized"; return; }
            var roles = user.roles || [];
            isStaff = app.effectiveIsStaff(roles);
            app.renderTopbar(user);
            pageContent.classList.remove("hidden");
            if (isStaff) {
                staffView.classList.remove("hidden");
                loadStaffClasses();
                wireStaffForm();
            } else {
                studentView.classList.remove("hidden");
                loadStudentClasses();
                wireStudentForm();
            }
        });
    }

    /* ══ STAFF ══════════════════════════════════════════════════════════════ */

    function loadStaffClasses() {
        api.get("/api/classes").then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                renderStaffClasses(data && data.items ? data.items : []);
            });
    }

    function renderStaffClasses(classes) {
        if (!staffClassList) return;
        if (classes.length === 0) {
            staffClassList.innerHTML =
                '<p class="text-muted" style="padding:.5rem 0">Nenhuma turma criada ainda. Clique em <strong>+ Nova turma</strong>.</p>';
            return;
        }
        staffClassList.innerHTML = classes.map(function (cl) {
            var badge = cl.is_active
                ? '<span class="badge badge-green" style="font-size:.72rem">Ativa</span>'
                : '<span class="badge badge-inactive" style="font-size:.72rem">Inativa</span>';
            var code = cl.invite_code
                ? '<span class="class-invite-code">' + app.esc(cl.invite_code) + '</span>'
                : '';
            return '<div class="class-card" role="listitem">' +
                '<div class="class-card__head">' +
                    '<div>' +
                        '<div class="class-card__name">' + app.esc(cl.name) + '</div>' +
                        (cl.description ? '<div class="class-card__desc">' + app.esc(cl.description) + '</div>' : '') +
                    '</div>' +
                    '<div style="display:flex;gap:.4rem;align-items:center;flex-wrap:wrap">' + badge + code + '</div>' +
                '</div>' +
                '<div class="class-card__meta">' +
                    '<span>' + cl.deck_count + ' deck(s)</span><span>·</span><span>' + cl.member_count + ' aluno(s)</span>' +
                '</div>' +
                '<div class="class-card__actions">' +
                    '<a href="/class_manage.html?classId=' + cl.id + '" class="btn btn-sm btn-primary">Gerenciar</a>' +
                '</div>' +
            '</div>';
        }).join('');
    }

    function wireStaffForm() {
        if (btnNewClass) btnNewClass.addEventListener("click", function () {
            newClassForm.classList.remove("hidden");
            if (newClassName) newClassName.focus();
        });
        if (btnCancelClass) btnCancelClass.addEventListener("click", function () {
            newClassForm.classList.add("hidden");
            if (newClassName) newClassName.value = "";
            if (newClassDesc) newClassDesc.value = "";
        });
        if (btnCreateClass) btnCreateClass.addEventListener("click", function () {
            var name = (newClassName ? newClassName.value : "").trim();
            if (!name) { toast("Informe o nome da turma.", "error"); return; }
            var desc = (newClassDesc && newClassDesc.value.trim()) ? newClassDesc.value.trim() : null;
            btnCreateClass.disabled = true;
            api.post("/api/classes", { name: name, description: desc })
                .then(function (r) {
                    if (!r.ok) return r.json().then(function (e) { throw new Error(e.detail || "Erro ao criar turma"); });
                    return r.json();
                })
                .then(function () {
                    newClassForm.classList.add("hidden");
                    if (newClassName) newClassName.value = "";
                    if (newClassDesc) newClassDesc.value = "";
                    loadStaffClasses();
                    toast("Turma criada!", "success");
                })
                .catch(function (e) { toast(e.message, "error"); })
                .finally(function () { btnCreateClass.disabled = false; });
        });
    }

    /* ══ STUDENT ════════════════════════════════════════════════════════════ */

    function loadStudentClasses() {
        api.get("/api/classes").then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                renderStudentClasses(data && data.items ? data.items : []);
            });
    }

    function renderStudentClasses(classes) {
        if (!studentClassList) return;
        if (classes.length === 0) {
            studentClassList.innerHTML =
                '<p class="text-muted" style="padding:.5rem 0">Você ainda não está inscrito em nenhuma turma.</p>';
            return;
        }
        studentClassList.innerHTML = classes.map(function (cl) {
            var joinedLabel = cl.joined_at
                ? 'Inscrito em ' + new Date(cl.joined_at).toLocaleDateString("pt-BR") : '';
            return '<div class="class-card" role="listitem">' +
                '<div class="class-card__head">' +
                    '<div>' +
                        '<div class="class-card__name">' + app.esc(cl.name) + '</div>' +
                        (cl.description ? '<div class="class-card__desc">' + app.esc(cl.description) + '</div>' : '') +
                    '</div>' +
                '</div>' +
                '<div class="class-card__meta">' +
                    '<span>' + cl.deck_count + ' deck(s)</span>' +
                    (joinedLabel ? '<span>· ' + joinedLabel + '</span>' : '') +
                '</div>' +
                '<div class="class-card__actions">' +
                    '<a href="/" class="btn btn-sm btn-primary">Ver decks</a>' +
                    '<button class="btn btn-sm btn-ghost btn-leave"' +
                        ' data-id="' + cl.id + '" data-name="' + app.esc(cl.name) + '"' +
                        ' style="color:var(--danger)">Sair da turma</button>' +
                '</div>' +
            '</div>';
        }).join('');

        studentClassList.querySelectorAll(".btn-leave").forEach(function (btn) {
            btn.addEventListener("click", function () {
                if (!confirm('Sair da turma "' + btn.dataset.name + '"?\nVocê poderá se reinscrever com o código de acesso.')) return;
                api.del("/api/classes/" + btn.dataset.id + "/leave")
                    .then(function (r) {
                        if (!r.ok) throw new Error("Erro ao sair da turma");
                        loadStudentClasses();
                        toast("Você saiu da turma.", "success");
                    })
                    .catch(function (e) { toast(e.message, "error"); });
            });
        });
    }

    function wireStudentForm() {
        if (btnJoin) btnJoin.addEventListener("click", joinClass);
        if (inviteInput) inviteInput.addEventListener("keydown", function (e) {
            if (e.key === "Enter") joinClass();
        });
    }

    function joinClass() {
        var code = (inviteInput ? inviteInput.value : "").trim().toUpperCase();
        if (!code) { showJoinError("Digite o código de acesso."); return; }
        if (joinError) joinError.classList.add("hidden");
        btnJoin.disabled = true;
        api.post("/api/classes/join", { invite_code: code })
            .then(function (r) {
                return r.json().then(function (body) {
                    if (!r.ok) throw new Error(body.detail || "Código inválido.");
                    return body;
                });
            })
            .then(function (cl) {
                if (inviteInput) inviteInput.value = "";
                loadStudentClasses();
                toast('Inscrito em "' + cl.name + '"! Os decks já aparecem na tela inicial.', "success");
            })
            .catch(function (e) { showJoinError(e.message); })
            .finally(function () { btnJoin.disabled = false; });
    }

    function showJoinError(msg) {
        if (!joinError) return;
        joinError.textContent = msg;
        joinError.classList.remove("hidden");
    }

    init();
})();
