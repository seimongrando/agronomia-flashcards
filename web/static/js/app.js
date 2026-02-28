(function () {
    "use strict";

    var CSRF_HEADER = "X-Requested-With";
    var CSRF_VALUE  = "XMLHttpRequest";

    function request(method, url, body) {
        var opts = {
            method: method,
            credentials: "same-origin",
            headers: {}
        };
        opts.headers[CSRF_HEADER] = CSRF_VALUE;
        if (body !== undefined) {
            opts.headers["Content-Type"] = "application/json";
            opts.body = JSON.stringify(body);
        }
        return fetch(url, opts);
    }

    window.api = {
        get:    function (url)       { return request("GET",    url); },
        post:   function (url, body) { return request("POST",   url, body); },
        put:    function (url, body) { return request("PUT",    url, body); },
        patch:  function (url, body) { return request("PATCH",  url, body); },
        del:    function (url)       { return request("DELETE", url); },
        upload: function (url, file) {
            var fd = new FormData();
            fd.append("file", file);
            return fetch(url, {
                method: "POST",
                credentials: "same-origin",
                headers: { "X-Requested-With": "XMLHttpRequest" },
                body: fd
            });
        }
    };

    window.toast = function (msg, kind) {
        var el = document.createElement("div");
        el.className = "toast" + (kind ? " toast-" + kind : "");
        el.textContent = msg;
        document.body.appendChild(el);
        setTimeout(function () { el.remove(); }, 3000);
    };

    // ── Logout ──────────────────────────────────────────────────────────────
    function logout() {
        api.post("/auth/logout").finally(function () {
            window.location.href = "/";
        });
    }

    // ── Topbar rendering ─────────────────────────────────────────────────────
    window.app = {
        checkAuth: function () {
            return api.get("/api/me").then(function (res) {
                if (!res.ok) return null;
                return res.json();
            }).catch(function () { return null; });
        },

        esc: function (s) {
            var el = document.createElement("span");
            el.textContent = String(s || "");
            return el.innerHTML;
        },

        /**
         * Renders the topbar right slot.
         * @param {Object|null} me  - response from /api/me ({user, roles})
         * @param {Object}      opts - { backHref, centerText, noNav }
         */
        renderTopbar: function (me, opts) {
            opts = opts || {};

            // Back link (study page)
            if (opts.backHref) {
                var back = document.getElementById("topbar-back");
                if (back) { back.href = opts.backHref; back.classList.remove("hidden"); }
            }

            // Center slot (deck title in study)
            if (opts.centerText) {
                var center = document.getElementById("topbar-center");
                if (center) center.textContent = opts.centerText;
            }

            var right = document.getElementById("topbar-right");
            if (!right) return;

            // ── Not logged in ──────────────────────────────────────────────
            if (!me || !me.user) {
                right.innerHTML =
                    '<a href="/auth/google" class="btn btn-sm btn-primary" aria-label="Entrar com Google">' +
                    '<svg width="14" height="14" viewBox="0 0 48 48" aria-hidden="true" style="flex-shrink:0">' +
                    '<path fill="#fff" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>' +
                    '<path fill="#fff" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>' +
                    '<path fill="#fff" d="M10.53 28.59a14.5 14.5 0 0 1 0-9.18l-7.97-6.19A23.998 23.998 0 0 0 0 24c0 3.77.87 7.35 2.56 10.78l7.97-6.19z"/>' +
                    '<path fill="#fff" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>' +
                    '</svg>Entrar</a>';
                return;
            }

            var user  = me.user;
            var roles = Array.isArray(me.roles) ? me.roles : [];
            var isAdmin     = roles.indexOf("admin") !== -1;
            var isProfessor = roles.indexOf("professor") !== -1;
            var isStaff     = isAdmin || isProfessor;

            var name  = user.name  || "Usuário";
            var email = user.email || "";
            var pic   = user.picture_url || "";

            // ── Nav links (role-based, suppressed in noNav mode e.g. study) ──
            var nav = "";
            if (!opts.noNav) {
                nav = '<nav class="topbar-nav" aria-label="Navegação principal">';
                nav += '<a href="/" class="nav-link">Início</a>';
                nav += '<a href="/progress.html" class="nav-link">Progresso</a>';
                if (isStaff) {
                    nav += '<a href="/teach.html" class="nav-link">Gerenciar</a>';
                }
                if (isAdmin) {
                    nav += '<a href="/admin_users.html" class="nav-link">Usuários</a>';
                }
                nav += '</nav>';
            }

            // ── User dropdown ──────────────────────────────────────────────
            var initials = name.split(" ").slice(0, 2).map(function (w) { return w[0]; }).join("").toUpperCase();
            var avatarHtml = pic
                ? '<img src="' + app.esc(pic) + '" alt="" class="topbar-avatar">'
                : '<span class="topbar-initials">' + app.esc(initials) + '</span>';

            var roleBadges = roles.map(function (r) {
                return '<span class="role-badge role-badge--' + app.esc(r) + '">' + app.esc(r) + '</span>';
            }).join(" ");

            var dropdown =
                '<div class="user-dropdown" id="user-dropdown">' +
                    '<button class="user-trigger" id="user-trigger" aria-haspopup="true" aria-expanded="false" aria-label="Menu do usuário">' +
                        avatarHtml +
                        '<span class="user-trigger-name">' + app.esc(name.split(" ")[0]) + '</span>' +
                        '<svg class="user-chevron" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">' +
                            '<path fill="currentColor" d="M6 8L1 3h10z"/>' +
                        '</svg>' +
                    '</button>' +
                    '<div class="user-menu" id="user-menu" role="menu" aria-hidden="true">' +
                        '<div class="user-menu-header">' +
                            '<strong class="user-menu-name">' + app.esc(name) + '</strong>' +
                            '<span class="user-menu-email">' + app.esc(email) + '</span>' +
                            '<div class="user-menu-roles">' + roleBadges + '</div>' +
                        '</div>' +
                        '<div class="user-menu-divider"></div>' +
                        (isStaff
                            ? '<a href="/teach.html" class="user-menu-item" role="menuitem">Gerenciar conteúdo</a>'
                            : '') +
                        (isAdmin
                            ? '<a href="/admin_users.html" class="user-menu-item" role="menuitem">Gerenciar usuários</a>'
                            : '') +
                        '<a href="/me.html" class="user-menu-item" role="menuitem">Meu perfil</a>' +
                        '<div class="user-menu-divider"></div>' +
                        '<button class="user-menu-item user-menu-logout" role="menuitem" id="btn-logout">Sair</button>' +
                    '</div>' +
                '</div>';

            right.innerHTML = nav + dropdown;

            // ── Dropdown toggle ────────────────────────────────────────────
            var trigger = document.getElementById("user-trigger");
            var menu    = document.getElementById("user-menu");

            function openMenu() {
                menu.setAttribute("aria-hidden", "false");
                trigger.setAttribute("aria-expanded", "true");
                menu.classList.add("open");
            }
            function closeMenu() {
                menu.setAttribute("aria-hidden", "true");
                trigger.setAttribute("aria-expanded", "false");
                menu.classList.remove("open");
            }

            trigger.addEventListener("click", function (e) {
                e.stopPropagation();
                menu.classList.contains("open") ? closeMenu() : openMenu();
            });

            document.addEventListener("click", function handler(e) {
                var dd = document.getElementById("user-dropdown");
                if (dd && !dd.contains(e.target)) {
                    closeMenu();
                }
            });

            trigger.addEventListener("keydown", function (e) {
                if (e.key === "Escape") { closeMenu(); trigger.focus(); }
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    menu.classList.contains("open") ? closeMenu() : openMenu();
                }
            });

            var btnLogout = document.getElementById("btn-logout");
            if (btnLogout) {
                btnLogout.addEventListener("click", logout);
            }

            // Highlight active nav link
            var currentPath = window.location.pathname;
            var navLinks = right.querySelectorAll(".nav-link");
            for (var i = 0; i < navLinks.length; i++) {
                var link = navLinks[i];
                var href = link.getAttribute("href");
                if (href === currentPath || (href !== "/" && currentPath.startsWith(href))) {
                    link.classList.add("nav-link--active");
                    break;
                }
                if (href === "/" && currentPath === "/") {
                    link.classList.add("nav-link--active");
                }
            }
        }
    };
})();
