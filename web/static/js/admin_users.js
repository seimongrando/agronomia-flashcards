(function () {
    "use strict";

    var spinnerEl      = document.getElementById("spinner");
    var deniedEl       = document.getElementById("access-denied");
    var contentEl      = document.getElementById("admin-content");
    var searchInput    = document.getElementById("search-input");
    var btnSearch      = document.getElementById("btn-search");
    var spinnerSearch  = document.getElementById("spinner-search");
    var userListEl     = document.getElementById("user-list");
    var emptyEl        = document.getElementById("empty-users");

    /* ── Init ────────────────────────────────────── */
    function init() {
        app.checkAuth().then(function (user) {
            spinnerEl.classList.add("hidden");
            if (!user) { window.location.href = "/"; return; }
            var roles = user.roles || [];
            if (roles.indexOf("admin") < 0) {
                app.renderTopbar(user);
                deniedEl.classList.remove("hidden");
                return;
            }
            app.renderTopbar(user);
            contentEl.classList.remove("hidden");
            loadUsers("");
        });
    }

    /* ── Search ──────────────────────────────────── */
    btnSearch.addEventListener("click", function () {
        loadUsers(searchInput.value.trim());
    });
    searchInput.addEventListener("keydown", function (e) {
        if (e.key === "Enter") loadUsers(searchInput.value.trim());
    });

    /* ── Load users ──────────────────────────────── */
    function loadUsers(query) {
        spinnerSearch.classList.remove("hidden");
        userListEl.innerHTML = "";
        emptyEl.classList.add("hidden");

        // API now accepts ?q= (new) instead of ?query= (legacy both accepted)
        var qs = query ? "?q=" + encodeURIComponent(query) : "";
        api.get("/api/admin/users" + qs)
            .then(function (res) {
                if (!res.ok) throw new Error();
                return res.json();
            })
            .then(function (page) {
                spinnerSearch.classList.add("hidden");
                // API returns {items: [...], next_cursor: "..." | null}
                var users = (page && page.items) ? page.items : (page || []);
                if (!users || users.length === 0) {
                    emptyEl.classList.remove("hidden");
                    return;
                }
                renderUsers(users);
            })
            .catch(function () {
                spinnerSearch.classList.add("hidden");
                userListEl.innerHTML = '<p class="alert alert-error">Erro ao carregar usuários.</p>';
            });
    }

    /* ── Render users ────────────────────────────── */
    function renderUsers(users) {
        var html = "";
        for (var i = 0; i < users.length; i++) {
            html += renderUserItem(users[i]);
        }
        userListEl.innerHTML = html;

        /* wire toggle buttons */
        var toggles = userListEl.querySelectorAll(".role-toggle");
        for (var j = 0; j < toggles.length; j++) {
            toggles[j].addEventListener("click", handleRoleToggle);
        }
    }

    function renderUserItem(u) {
        var roles = u.roles || [];
        var isProfessor = roles.indexOf("professor") >= 0;
        var isAdmin     = roles.indexOf("admin") >= 0;

        var rolesHtml = "";
        rolesHtml += '<button class="role-toggle ' + (isProfessor ? "active" : "inactive") + '" ' +
            'data-user-id="' + app.esc(u.id) + '" data-role="professor" data-active="' + isProfessor + '">' +
            'Professor</button>';
        if (isAdmin) {
            rolesHtml += '<span class="role-badge">admin</span>';
        }

        return '<div class="user-item" id="user-' + app.esc(u.id) + '">' +
            '<div class="user-info">' +
                '<div class="user-name">' + app.esc(u.name || "—") + '</div>' +
                '<div class="user-email">' + app.esc(u.email) + '</div>' +
            '</div>' +
            '<div class="user-roles">' + rolesHtml + '</div>' +
        '</div>';
    }

    /* ── Role toggle ─────────────────────────────── */
    function handleRoleToggle() {
        var btn    = this;
        var userID = btn.getAttribute("data-user-id");
        var role   = btn.getAttribute("data-role");
        var active = btn.getAttribute("data-active") === "true";

        btn.disabled = true;

        var body = active
            ? { remove: [role] }
            : { add:    [role] };

        api.post("/api/admin/users/" + userID + "/roles", body)
            .then(function (res) {
                return res.json().then(function (data) {
                    return { ok: res.ok, data: data };
                });
            })
            .then(function (r) {
                if (!r.ok) {
                    var msg = (r.data && r.data.detail) ? r.data.detail : "Erro ao alterar role.";
                    toast(msg, "error");
                    btn.disabled = false;
                    return;
                }
                var newRoles = r.data.roles || [];
                var nowActive = newRoles.indexOf(role) >= 0;
                btn.setAttribute("data-active", nowActive ? "true" : "false");
                btn.className = "role-toggle " + (nowActive ? "active" : "inactive");
                toast(nowActive ? "Role adicionada." : "Role removida.", "ok");
            })
            .catch(function () {
                toast("Erro ao alterar role.", "error");
            })
            .finally(function () { btn.disabled = false; });
    }

    init();
})();
