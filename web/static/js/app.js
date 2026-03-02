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

    // ── Student-mode helpers ─────────────────────────────────────────────────
    // Admins/professors can activate "student view" to experience the app as a
    // student. The mode is stored in sessionStorage and cleared on exit or when
    // the browser tab is closed. The real server-side role is never changed.
    var STUDENT_MODE_KEY = "_agro_student_mode";

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

        // Returns true when a staff user has activated student-preview mode.
        isStudentMode: function () {
            return sessionStorage.getItem(STUDENT_MODE_KEY) === "1";
        },

        // Activate student-preview mode and reload so all pages re-render.
        enterStudentMode: function () {
            sessionStorage.setItem(STUDENT_MODE_KEY, "1");
            window.location.href = "/";
        },

        // Deactivate student-preview mode and return to admin home.
        exitStudentMode: function () {
            sessionStorage.removeItem(STUDENT_MODE_KEY);
            window.location.href = "/";
        },

        // Convenience: returns effective isStaff respecting student mode.
        // Use this in page scripts instead of checking roles directly.
        effectiveIsStaff: function (roles) {
            if (app.isStudentMode()) return false;
            roles = Array.isArray(roles) ? roles : [];
            return roles.indexOf("professor") !== -1 || roles.indexOf("admin") !== -1;
        },

        /**
         * Renders the topbar right slot.
         * @param {Object|null} me   - response from /api/me ({user, roles})
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

            // Real role flags (never affected by student mode)
            var realIsAdmin     = roles.indexOf("admin") !== -1;
            var realIsProfessor = roles.indexOf("professor") !== -1;
            var realIsStaff     = realIsAdmin || realIsProfessor;

            // Effective role flags (flipped to student when mode is active)
            var studentMode  = realIsStaff && app.isStudentMode();
            var effIsStaff   = realIsStaff && !studentMode;
            var effIsAdmin   = realIsAdmin  && !studentMode;

            var name  = user.name  || "Usuário";
            var email = user.email || "";
            var pic   = user.picture_url || "";

            // ── Nav links (role-based, suppressed in noNav mode e.g. study) ──
            var nav = "";
            if (!opts.noNav) {
                nav = '<nav class="topbar-nav" aria-label="Navegação principal">';
                nav += '<a href="/" class="nav-link">Início</a>';
                nav += '<a href="/classes.html" class="nav-link">Turmas</a>';
                if (!effIsStaff) {
                    nav += '<a href="/my_decks.html" class="nav-link">Meus Cards</a>';
                }
                nav += '<a href="/progress.html" class="nav-link">Progresso</a>';
                if (effIsStaff) {
                    nav += '<a href="/teach.html" class="nav-link">Gerenciar</a>';
                    nav += '<a href="/professor_stats.html" class="nav-link">Painel</a>';
                }
                if (effIsAdmin) {
                    nav += '<a href="/admin_users.html" class="nav-link">Usuários</a>';
                }
                nav += '</nav>';
            }

            // ── User dropdown ──────────────────────────────────────────────
            var initials = name.split(" ").slice(0, 2).map(function (w) { return w[0]; }).join("").toUpperCase();
            var avatarEl = pic
                ? '<img src="' + app.esc(pic) + '" alt="" class="topbar-avatar">'
                : '<span class="topbar-initials">' + app.esc(initials) + '</span>';

            // In student mode show the avatar with a small badge to signal the mode
            var triggerAvatarEl = studentMode
                ? '<span class="topbar-avatar-wrap">' + avatarEl +
                  '<span class="student-mode-dot" title="Modo Aluno ativo"></span></span>'
                : avatarEl;

            var roleBadges = roles.map(function (r) {
                return '<span class="role-badge role-badge--' + app.esc(r) + '">' + app.esc(r) + '</span>';
            }).join(" ");

            // Student-mode toggle item shown inside the dropdown
            var studentModeItem = realIsStaff
                ? (studentMode
                    ? '<button class="user-menu-item user-menu-item--student-exit" role="menuitem" id="btn-exit-student-mode">' +
                      '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0">' +
                      '<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                      '</svg>Sair do Modo Aluno</button>'
                    : '<button class="user-menu-item user-menu-item--student-enter" role="menuitem" id="btn-enter-student-mode">' +
                      '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0">' +
                      '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                      '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2"/>' +
                      '<path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                      '</svg>Visualizar como aluno</button>')
                : '';

            var dropdown =
                '<div class="user-dropdown" id="user-dropdown">' +
                    '<button class="user-trigger' + (studentMode ? ' user-trigger--student-mode' : '') + '" id="user-trigger" aria-haspopup="true" aria-expanded="false" aria-label="Menu do usuário">' +
                        triggerAvatarEl +
                        '<span class="user-trigger-name">' + app.esc(name.split(" ")[0]) + '</span>' +
                        '<svg class="user-chevron" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">' +
                            '<path fill="currentColor" d="M6 8L1 3h10z"/>' +
                        '</svg>' +
                    '</button>' +
                    '<div class="user-menu" id="user-menu" role="menu" aria-hidden="true">' +
                        '<div class="user-menu-header">' +
                            '<strong class="user-menu-name">' + app.esc(name) + '</strong>' +
                            '<span class="user-menu-email">' + app.esc(email) + '</span>' +
                            '<div class="user-menu-roles">' + roleBadges +
                                (studentMode ? '<span class="role-badge" style="background:#D97706;color:#fff">modo aluno</span>' : '') +
                            '</div>' +
                        '</div>' +
                        '<div class="user-menu-divider"></div>' +
                        '<a href="/classes.html" class="user-menu-item" role="menuitem">Turmas</a>' +
                        (effIsStaff ? '<a href="/teach.html" class="user-menu-item" role="menuitem">Gerenciar conteúdo</a>' : '') +
                        (effIsAdmin ? '<a href="/admin_users.html" class="user-menu-item" role="menuitem">Gerenciar usuários</a>' : '') +
                        '<a href="/me.html" class="user-menu-item" role="menuitem">Meu perfil</a>' +
                        (studentModeItem ? '<div class="user-menu-divider"></div>' + studentModeItem : '') +
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
                if (dd && !dd.contains(e.target)) { closeMenu(); }
            });

            trigger.addEventListener("keydown", function (e) {
                if (e.key === "Escape") { closeMenu(); trigger.focus(); }
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    menu.classList.contains("open") ? closeMenu() : openMenu();
                }
            });

            var btnLogout = document.getElementById("btn-logout");
            if (btnLogout) btnLogout.addEventListener("click", logout);

            var btnEnter = document.getElementById("btn-enter-student-mode");
            if (btnEnter) btnEnter.addEventListener("click", app.enterStudentMode);

            var btnExit = document.getElementById("btn-exit-student-mode");
            if (btnExit) btnExit.addEventListener("click", app.exitStudentMode);

            // Highlight active nav link
            var currentPath = window.location.pathname;
            var navLinks = right.querySelectorAll(".nav-link");
            for (var i = 0; i < navLinks.length; i++) {
                var link = navLinks[i];
                var href = link.getAttribute("href");
                if (href === "/" && currentPath === "/") { link.classList.add("nav-link--active"); break; }
                if (href !== "/" && currentPath.startsWith(href)) { link.classList.add("nav-link--active"); break; }
            }

            // ── Student-mode floating banner ───────────────────────────────
            if (studentMode && !document.getElementById("student-mode-banner")) {
                var banner = document.createElement("div");
                banner.id = "student-mode-banner";
                banner.className = "student-mode-banner";
                banner.innerHTML =
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2"/>' +
                    '</svg>' +
                    '<span>Você está no <strong>Modo Aluno</strong> — visualização como estudante</span>' +
                    '<button class="student-mode-banner__exit" id="btn-banner-exit-student">Voltar ao admin</button>';
                document.body.appendChild(banner);

                document.getElementById("btn-banner-exit-student")
                    .addEventListener("click", app.exitStudentMode);
            }
        }
    };
    // ── Service Worker registration ──────────────────────────────────────────
    if ('serviceWorker' in navigator) {
        window.addEventListener('load', function () {
            navigator.serviceWorker.register('/sw.js', { scope: '/' })
                .then(function (reg) {
                    // After authentication is confirmed, show the notification prompt
                    // (once per device, only to students — staff rarely need reminders).
                    api.get("/api/me").then(function (res) {
                        if (!res.ok) return;
                        return res.json();
                    }).then(function (me) {
                        if (!me) return;
                        // Only prompt students (not staff in their real role).
                        var isStaff = app.effectiveIsStaff(me.roles || []);
                        if (!isStaff) {
                            showNotificationPromptIfNeeded(reg);
                        }
                    }).catch(function () { /* non-critical */ });
                })
                .catch(function () { /* SW optional — silently ignore */ });
        });
    }

    // ── Push Notification helpers ─────────────────────────────────────────────

    var NOTIF_DISMISSED_KEY = "_agro_notif_dismissed";
    var NOTIF_SUBSCRIBED_KEY = "_agro_notif_subscribed";

    function showNotificationPromptIfNeeded(swReg) {
        // Don't show if already subscribed or previously dismissed.
        if (localStorage.getItem(NOTIF_DISMISSED_KEY)) return;
        if (localStorage.getItem(NOTIF_SUBSCRIBED_KEY)) return;
        if (!("Notification" in window))                return;
        if (Notification.permission === "granted")     { ensureSubscribed(swReg); return; }
        if (Notification.permission === "denied")      return;

        // Show a subtle in-page prompt instead of immediately calling requestPermission()
        // (browsers require a user gesture for permission dialogs).
        var prompt = document.createElement("div");
        prompt.className = "notif-prompt";
        prompt.setAttribute("role", "dialog");
        prompt.setAttribute("aria-label", "Ativar lembretes de estudo");
        prompt.innerHTML =
            '<span class="notif-prompt__icon" aria-hidden="true">🌱</span>' +
            '<span class="notif-prompt__text">' +
            'Quer receber um lembrete diário quando tiver cards para revisar?' +
            '</span>' +
            '<div class="notif-prompt__actions">' +
            '<button class="btn btn-primary" id="_notif-yes">Ativar</button>' +
            '<button class="btn btn-ghost"   id="_notif-no">Agora não</button>' +
            '</div>';
        document.body.appendChild(prompt);

        document.getElementById("_notif-yes").addEventListener("click", function () {
            prompt.remove();
            Notification.requestPermission().then(function (perm) {
                if (perm === "granted") {
                    ensureSubscribed(swReg);
                } else {
                    localStorage.setItem(NOTIF_DISMISSED_KEY, "1");
                }
            });
        });
        document.getElementById("_notif-no").addEventListener("click", function () {
            prompt.remove();
            localStorage.setItem(NOTIF_DISMISSED_KEY, "1");
        });
    }

    function ensureSubscribed(swReg) {
        // Fetch the server's VAPID public key.
        api.get("/api/push/key").then(function (res) {
            if (!res.ok) return null;
            return res.json();
        }).then(function (data) {
            if (!data || !data.public_key) return;
            var appServerKey = urlBase64ToUint8Array(data.public_key);

            return swReg.pushManager.subscribe({
                userVisibleOnly: true,
                applicationServerKey: appServerKey
            }).then(function (sub) {
                // Send subscription to our server.
                var json = sub.toJSON();
                return api.post("/api/push/subscribe", {
                    endpoint: json.endpoint,
                    keys: { p256dh: json.keys.p256dh, auth: json.keys.auth }
                }).then(function (res) {
                    if (res.ok) {
                        localStorage.setItem(NOTIF_SUBSCRIBED_KEY, "1");
                        toast("Lembretes ativados! Você receberá notificações diárias.", "ok");
                    }
                });
            });
        }).catch(function () { /* push optional */ });
    }

    // Convert VAPID base64url public key to Uint8Array required by pushManager.subscribe().
    function urlBase64ToUint8Array(base64String) {
        var padding = "=".repeat((4 - (base64String.length % 4)) % 4);
        var base64  = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
        var raw     = atob(base64);
        var arr     = new Uint8Array(raw.length);
        for (var i = 0; i < raw.length; i++) { arr[i] = raw.charCodeAt(i); }
        return arr;
    }

    // Expose unsubscribe for the profile menu (optional).
    window.app.unsubscribeNotifications = function () {
        if (!("serviceWorker" in navigator)) return;
        navigator.serviceWorker.ready.then(function (reg) {
            return reg.pushManager.getSubscription();
        }).then(function (sub) {
            if (!sub) return;
            var endpoint = sub.endpoint;
            return sub.unsubscribe().then(function () {
                // DELETE with body requires fetch directly (api.del has no body param).
                return fetch("/api/push/subscribe", {
                    method: "DELETE",
                    credentials: "same-origin",
                    headers: {
                        "Content-Type": "application/json",
                        "X-Requested-With": "XMLHttpRequest"
                    },
                    body: JSON.stringify({ endpoint: endpoint })
                });
            }).then(function () {
                localStorage.removeItem(NOTIF_SUBSCRIBED_KEY);
                localStorage.removeItem(NOTIF_DISMISSED_KEY);
                toast("Notificações desativadas.", "ok");
            });
        }).catch(function () { /* non-critical */ });
    };
})();
