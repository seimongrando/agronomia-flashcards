(function () {
    "use strict";

    var spinnerEl = document.getElementById("spinner");
    var profileEl = document.getElementById("profile");
    var errorEl   = document.getElementById("profile-error");

    var NOTIF_DISMISSED_KEY  = "_agro_notif_dismissed";
    var NOTIF_SUBSCRIBED_KEY = "_agro_notif_subscribed";

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

        // ── Notification settings section ────────────────────────────────────
        html += '<hr class="profile-divider">';
        html += '<div id="notif-section" class="profile-section">' +
                '<div class="profile-section__header">' +
                '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
                '<path d="M18 8A6 6 0 1 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                '<path d="M13.73 21a2 2 0 0 1-3.46 0" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
                '</svg>' +
                '<span>Notificações</span>' +
                '</div>' +
                '<div id="notif-status" class="notif-status">' +
                '<div class="spinner spinner--sm" role="status" aria-label="Verificando…"></div>' +
                '</div>' +
                '</div>';

        html += '<hr class="profile-divider">';
        html += '<button id="btn-do-logout" class="btn btn-danger-outline">Sair da conta</button>';

        profileEl.innerHTML = html;
        profileEl.classList.remove("hidden");

        document.getElementById("btn-do-logout").addEventListener("click", function () {
            var btn = document.getElementById("btn-do-logout");
            btn.disabled = true;
            btn.textContent = "Saindo\u2026";
            api.post("/auth/logout").finally(function () {
                window.location.href = "/";
            });
        });

        // Resolve notification state and render the toggle.
        resolveNotifState().then(renderNotifStatus);
    }

    // ── Notification state resolution ─────────────────────────────────────────

    /**
     * Returns a state object describing the current notification setup.
     * state.status: "unsupported" | "denied" | "subscribed" | "unsubscribed"
     * state.swReg: ServiceWorkerRegistration (when supported)
     * state.sub: PushSubscription (when subscribed)
     */
    function resolveNotifState() {
        if (!("Notification" in window) || !("serviceWorker" in navigator)) {
            return Promise.resolve({ status: "unsupported" });
        }
        if (Notification.permission === "denied") {
            return Promise.resolve({ status: "denied" });
        }

        return navigator.serviceWorker.ready.then(function (reg) {
            return reg.pushManager.getSubscription().then(function (sub) {
                return {
                    status: sub ? "subscribed" : "unsubscribed",
                    swReg: reg,
                    sub: sub
                };
            });
        }).catch(function () {
            return { status: "unsupported" };
        });
    }

    function renderNotifStatus(state) {
        var el = document.getElementById("notif-status");
        if (!el) return;

        if (state.status === "unsupported") {
            el.innerHTML =
                '<span class="notif-badge notif-badge--off">Não suportado</span>' +
                '<p class="notif-hint">Seu dispositivo ou navegador não suporta notificações push.</p>';
            return;
        }

        if (state.status === "denied") {
            el.innerHTML =
                '<span class="notif-badge notif-badge--blocked">Bloqueadas</span>' +
                '<p class="notif-hint">As notificações foram bloqueadas no seu navegador. Para reativar, clique no cadeado ' +
                'na barra de endereço e altere a permissão de notificações para este site.</p>';
            return;
        }

        if (state.status === "subscribed") {
            el.innerHTML =
                '<div class="notif-row">' +
                '<span class="notif-badge notif-badge--on">Ativas</span>' +
                '<span class="notif-hint">Você receberá um lembrete diário quando tiver cards para revisar.</span>' +
                '</div>' +
                '<button id="btn-notif-disable" class="btn btn-outline btn-sm" style="margin-top:.75rem">Desativar lembretes</button>';

            document.getElementById("btn-notif-disable").addEventListener("click", function () {
                disableNotifications(state);
            });
            return;
        }

        // unsubscribed
        el.innerHTML =
            '<div class="notif-row">' +
            '<span class="notif-badge notif-badge--off">Desativadas</span>' +
            '<span class="notif-hint">Ative para receber um lembrete diário dos cards pendentes.</span>' +
            '</div>' +
            '<button id="btn-notif-enable" class="btn btn-primary btn-sm" style="margin-top:.75rem">Ativar lembretes</button>';

        document.getElementById("btn-notif-enable").addEventListener("click", function () {
            enableNotifications(state);
        });
    }

    // ── Enable / Disable ──────────────────────────────────────────────────────

    function enableNotifications(state) {
        var btn = document.getElementById("btn-notif-enable");
        if (btn) { btn.disabled = true; btn.textContent = "Ativando\u2026"; }

        Notification.requestPermission().then(function (perm) {
            if (perm !== "granted") {
                // User denied in the browser dialog.
                localStorage.setItem(NOTIF_DISMISSED_KEY, "1");
                resolveNotifState().then(renderNotifStatus);
                return;
            }
            // Fetch VAPID public key from server.
            return api.get("/api/push/key").then(function (res) {
                if (!res.ok) throw new Error("push not configured");
                return res.json();
            }).then(function (data) {
                var appServerKey = urlBase64ToUint8Array(data.public_key);
                return state.swReg.pushManager.subscribe({
                    userVisibleOnly: true,
                    applicationServerKey: appServerKey
                });
            }).then(function (sub) {
                var json = sub.toJSON();
                return api.post("/api/push/subscribe", {
                    endpoint: json.endpoint,
                    keys: { p256dh: json.keys.p256dh, auth: json.keys.auth }
                });
            }).then(function (res) {
                if (!res.ok) throw new Error("subscribe failed");
                localStorage.setItem(NOTIF_SUBSCRIBED_KEY, "1");
                localStorage.removeItem(NOTIF_DISMISSED_KEY);
                toast("Lembretes ativados! Você receberá notificações diárias.", "ok");
                resolveNotifState().then(renderNotifStatus);
            });
        }).catch(function () {
            toast("Não foi possível ativar as notificações.", "error");
            resolveNotifState().then(renderNotifStatus);
        });
    }

    function disableNotifications(state) {
        var btn = document.getElementById("btn-notif-disable");
        if (btn) { btn.disabled = true; btn.textContent = "Desativando\u2026"; }

        var endpoint = state.sub ? state.sub.endpoint : null;

        var unsubPromise = state.sub ? state.sub.unsubscribe() : Promise.resolve();
        unsubPromise.then(function () {
            if (!endpoint) return Promise.resolve();
            // Notify server to remove the subscription.
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
            toast("Lembretes desativados.", "ok");
            resolveNotifState().then(renderNotifStatus);
        }).catch(function () {
            toast("Erro ao desativar notificações.", "error");
            resolveNotifState().then(renderNotifStatus);
        });
    }

    // ── Helper ────────────────────────────────────────────────────────────────

    function urlBase64ToUint8Array(base64) {
        var padding = "=".repeat((4 - (base64.length % 4)) % 4);
        var b64     = (base64 + padding).replace(/-/g, "+").replace(/_/g, "/");
        var raw     = atob(b64);
        var arr     = new Uint8Array(raw.length);
        for (var i = 0; i < raw.length; i++) { arr[i] = raw.charCodeAt(i); }
        return arr;
    }

    // ── Init ──────────────────────────────────────────────────────────────────

    app.checkAuth()
        .then(function (data) {
            spinnerEl.classList.add("hidden");
            if (!data) { window.location.href = "/"; return; }
            renderProfile(data);
        })
        .catch(function () {
            showError("Erro ao carregar perfil. Tente recarregar a p\u00e1gina.");
        });
})();
