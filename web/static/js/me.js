(function () {
    "use strict";

    var spinnerEl = document.getElementById("spinner");
    var profileEl = document.getElementById("profile");
    var errorEl   = document.getElementById("profile-error");

    function showError(msg) {
        spinnerEl.classList.add("hidden");
        errorEl.textContent = msg || "Não foi possível carregar o perfil.";
        errorEl.classList.remove("hidden");
    }

    function renderProfile(data) {
        if (!data || !data.user) { window.location.href = "/"; return; }

        app.renderTopbar(data);

        var user  = data.user;
        var roles = Array.isArray(data.roles) ? data.roles : [];
        var pic   = user.picture_url || "";

        var html = "";
        if (pic) {
            html += '<img src="' + app.esc(pic) + '" alt="Foto de perfil" class="profile-avatar">';
        } else {
            var words    = (user.name || "U").trim().split(/\s+/);
            var initials = words.slice(0, 2).map(function (w) { return w[0] || ""; }).join("").toUpperCase();
            html += '<div class="profile-avatar profile-avatar--initials">' + app.esc(initials) + '</div>';
        }

        html += '<div class="profile-name">'  + app.esc(user.name  || "") + '</div>';
        html += '<div class="profile-email">' + app.esc(user.email || "") + '</div>';

        if (roles.length > 0) {
            html += '<div class="profile-roles">';
            for (var i = 0; i < roles.length; i++) {
                html += '<span class="role-badge role-badge--' + app.esc(roles[i]) + '">' + app.esc(roles[i]) + '</span>';
            }
            html += '</div>';
        }

        html += '<hr style="border:none;border-top:1px solid var(--border);margin:1.25rem 0">';
        html += '<button id="btn-do-logout" class="btn btn-danger-outline">Sair da conta</button>';

        profileEl.innerHTML = html;
        profileEl.classList.remove("hidden");

        var logoutBtn = document.getElementById("btn-do-logout");
        if (logoutBtn) {
            logoutBtn.addEventListener("click", function () {
                logoutBtn.disabled = true;
                logoutBtn.textContent = "Saindo\u2026";
                api.post("/auth/logout").finally(function () {
                    window.location.href = "/";
                });
            });
        }
    }

    app.checkAuth()
        .then(function (data) {
            spinnerEl.classList.add("hidden");
            if (!data) {
                window.location.href = "/";
                return;
            }
            renderProfile(data);
        })
        .catch(function () {
            showError("Erro ao carregar perfil. Tente recarregar a p\u00e1gina.");
        });
})();
