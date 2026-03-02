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

    // ── Role-preview helpers ──────────────────────────────────────────────────
    // Admins can simulate student or professor view without changing their real
    // server-side role. State lives in sessionStorage (cleared on tab close).
    // Modes are mutually exclusive: entering one always exits the other.
    var STUDENT_MODE_KEY   = "_agro_student_mode";
    var PROFESSOR_MODE_KEY = "_agro_professor_mode";
    var BANNER_DISMISSED_KEY = "_agro_banner_dismissed";

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

        // ── Mode predicates ───────────────────────────────────────────────────
        isStudentMode:   function () { return sessionStorage.getItem(STUDENT_MODE_KEY)   === "1"; },
        isProfessorMode: function () { return sessionStorage.getItem(PROFESSOR_MODE_KEY) === "1"; },

        // ── Mode transitions ──────────────────────────────────────────────────
        enterStudentMode: function () {
            sessionStorage.setItem(STUDENT_MODE_KEY, "1");
            sessionStorage.removeItem(PROFESSOR_MODE_KEY);
            sessionStorage.removeItem(BANNER_DISMISSED_KEY);
            window.location.href = "/";
        },
        exitStudentMode: function () {
            sessionStorage.removeItem(STUDENT_MODE_KEY);
            sessionStorage.removeItem(BANNER_DISMISSED_KEY);
            window.location.href = "/";
        },
        enterProfessorMode: function () {
            sessionStorage.setItem(PROFESSOR_MODE_KEY, "1");
            sessionStorage.removeItem(STUDENT_MODE_KEY);
            sessionStorage.removeItem(BANNER_DISMISSED_KEY);
            window.location.href = "/";
        },
        exitProfessorMode: function () {
            sessionStorage.removeItem(PROFESSOR_MODE_KEY);
            sessionStorage.removeItem(BANNER_DISMISSED_KEY);
            window.location.href = "/";
        },

        // Convenience: returns effective isStaff respecting active preview mode.
        // Professor mode keeps staff = true; student mode forces it to false.
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

            // Real role flags (never affected by preview modes)
            var realIsAdmin     = roles.indexOf("admin") !== -1;
            var realIsProfessor = roles.indexOf("professor") !== -1;
            var realIsStaff     = realIsAdmin || realIsProfessor;

            // Active preview mode
            var studentMode   = realIsStaff && app.isStudentMode();
            var professorMode = realIsAdmin  && !studentMode && app.isProfessorMode();
            var anyPreview    = studentMode || professorMode;

            // Effective role flags
            var effIsStaff = realIsStaff && !studentMode;  // professor mode keeps staff=true
            var effIsAdmin = realIsAdmin  && !anyPreview;   // professor mode hides admin links

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

            // In preview mode show the avatar with a small coloured dot
            var triggerAvatarEl = anyPreview
                ? '<span class="topbar-avatar-wrap">' + avatarEl +
                  '<span class="student-mode-dot student-mode-dot--' +
                  (studentMode ? "student" : "professor") +
                  '" title="' + (studentMode ? "Modo Aluno" : "Modo Professor") + ' ativo"></span></span>'
                : avatarEl;

            var roleBadges = roles.map(function (r) {
                return '<span class="role-badge role-badge--' + app.esc(r) + '">' + app.esc(r) + '</span>';
            }).join(" ");

            // ── Preview mode items shown inside the dropdown ───────────────
            var iconExit =
                '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0">' +
                '<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                '</svg>';
            var iconStudent =
                '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0">' +
                '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2"/>' +
                '<path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                '</svg>';
            var iconProfessor =
                '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="flex-shrink:0">' +
                '<path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6A19.79 19.79 0 0 1 2.12 4.18 2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 16.92z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                '</svg>';

            var studentModeItem = "";
            if (realIsStaff) {
                if (studentMode) {
                    // Currently in student mode → show exit
                    studentModeItem =
                        '<button class="user-menu-item user-menu-item--student-exit" role="menuitem" id="btn-exit-student-mode">' +
                        iconExit + 'Sair do Modo Aluno</button>';
                } else if (professorMode) {
                    // Currently in professor mode → show exit
                    studentModeItem =
                        '<button class="user-menu-item user-menu-item--professor-exit" role="menuitem" id="btn-exit-professor-mode">' +
                        iconExit + 'Sair do Modo Professor</button>';
                } else {
                    // Normal mode → show both entry options (professor entry only for admins)
                    studentModeItem =
                        '<button class="user-menu-item user-menu-item--student-enter" role="menuitem" id="btn-enter-student-mode">' +
                        iconStudent + 'Visualizar como aluno</button>';
                    if (realIsAdmin) {
                        studentModeItem +=
                            '<button class="user-menu-item user-menu-item--professor-enter" role="menuitem" id="btn-enter-professor-mode">' +
                            iconProfessor + 'Visualizar como professor</button>';
                    }
                }
            }

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
                                (studentMode   ? '<span class="role-badge" style="background:#D97706;color:#fff">modo aluno</span>'      : '') +
                                (professorMode ? '<span class="role-badge" style="background:#2563EB;color:#fff">modo professor</span>' : '') +
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

            var btnEnterStudent = document.getElementById("btn-enter-student-mode");
            if (btnEnterStudent) btnEnterStudent.addEventListener("click", app.enterStudentMode);

            var btnExitStudent = document.getElementById("btn-exit-student-mode");
            if (btnExitStudent) btnExitStudent.addEventListener("click", app.exitStudentMode);

            var btnEnterProf = document.getElementById("btn-enter-professor-mode");
            if (btnEnterProf) btnEnterProf.addEventListener("click", app.enterProfessorMode);

            var btnExitProf = document.getElementById("btn-exit-professor-mode");
            if (btnExitProf) btnExitProf.addEventListener("click", app.exitProfessorMode);

            // Highlight active nav link
            var currentPath = window.location.pathname;
            var navLinks = right.querySelectorAll(".nav-link");
            for (var i = 0; i < navLinks.length; i++) {
                var link = navLinks[i];
                var href = link.getAttribute("href");
                if (href === "/" && currentPath === "/") { link.classList.add("nav-link--active"); break; }
                if (href !== "/" && currentPath.startsWith(href)) { link.classList.add("nav-link--active"); break; }
            }

            // ── Preview-mode floating banner (bottom) ─────────────────────
            if (anyPreview && !document.getElementById("student-mode-banner") &&
                !sessionStorage.getItem(BANNER_DISMISSED_KEY)) {

                var isProf   = professorMode;
                var modeLabel = isProf ? "Modo Professor" : "Modo Aluno";
                var modeClass = isProf ? "student-mode-banner--professor" : "";
                var exitFn    = isProf ? "app.exitProfessorMode()" : "app.exitStudentMode()";
                var exitBtnId = isProf ? "btn-banner-exit-prof" : "btn-banner-exit-student";

                var banner = document.createElement("div");
                banner.id = "student-mode-banner";
                banner.className = "student-mode-banner " + modeClass;
                banner.innerHTML =
                    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>' +
                    '<circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2"/>' +
                    '</svg>' +
                    '<span>Você está no <strong>' + modeLabel + '</strong> — visualização simulada</span>' +
                    '<button class="student-mode-banner__exit" id="' + exitBtnId + '">Sair do modo</button>' +
                    '<button class="student-mode-banner__close" id="btn-banner-dismiss" aria-label="Fechar banner">' +
                    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                    '<path d="M18 6 6 18M6 6l12 12" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>' +
                    '</svg></button>';
                document.body.appendChild(banner);

                document.getElementById(exitBtnId).addEventListener("click",
                    isProf ? app.exitProfessorMode : app.exitStudentMode);

                document.getElementById("btn-banner-dismiss").addEventListener("click", function () {
                    banner.remove();
                    sessionStorage.setItem(BANNER_DISMISSED_KEY, "1");
                });
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
