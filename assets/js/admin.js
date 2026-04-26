(function() {
  const apiBase = (window.HOLO_CONFIG?.apiBase || "/api/v1").replace(/\/$/, "");
  const router = window.__ESTELAR_ROUTER__ || null;
  const publicPages = ["home", "rules", "contacts"];
  const publicPageSet = new Set(publicPages);
  const unlockPattern = ["ArrowRight", "ArrowRight", "ArrowLeft", "ArrowLeft"];
  const unlockStorageKey = "estelar-admin-unlocked";
  const oauthPendingKey = "estelar-admin-oauth-pending";
  const returnPageKey = "estelar-admin-return-page";
  const state = {
    meta: null,
    supabase: null,
    session: null,
    identity: null,
    dashboard: null,
    orders: [],
    catalog: [],
    promos: [],
    settings: null,
    selectedOrderID: "",
    selectedCatalogSlug: "",
    selectedPromoCode: "",
    unlocked: false,
    unlockBuffer: [],
    adminOpening: false,
    orderLoading: false,
    publicPageBeforeAdmin: "home",
    activeView: "overview",
    integrations: null,
    payments: null,
    paymentProviders: [],
    integrationsLoaded: false,
    integrationsLoading: false
  };

  const els = {
    intro: document.querySelector("#admin_intro"),
    introText: document.querySelector("#admin_intro_text"),
    introStatus: document.querySelector("#admin_intro_status"),
    gate: document.querySelector("#admin_gate"),
    gateText: document.querySelector("#admin_gate_text"),
    gateStatus: document.querySelector("#admin_status"),
    loginButton: document.querySelector("#admin_discord_login"),
    logoutButton: document.querySelector("#admin_logout_button"),
    shell: document.querySelector("#admin_shell"),
    metrics: document.querySelector("#admin_metrics"),
    analyticsSummary: document.querySelector("#admin_analytics_summary"),
    analyticsTrend: document.querySelector("#admin_analytics_trend"),
    pageStats: document.querySelector("#admin_page_stats"),
    recentOrders: document.querySelector("#admin_recent_orders"),
    orderKpis: document.querySelector("#admin_order_kpis"),
    orderList: document.querySelector("#admin_order_list"),
    orderDetail: document.querySelector("#admin_order_detail"),
    orderStatusFilter: document.querySelector("#admin_order_status_filter"),
    orderSearch: document.querySelector("#admin_order_search"),
    catalogList: document.querySelector("#admin_catalog_list"),
    catalogSearch: document.querySelector("#admin_catalog_search"),
    catalogForm: document.querySelector("#admin_catalog_form"),
    promoList: document.querySelector("#admin_promo_list"),
    promoForm: document.querySelector("#admin_promo_form"),
    settingsForm: document.querySelector("#admin_settings_form"),
    contactsPreview: document.querySelector("#admin_contacts_preview"),
    viewTitle: document.querySelector("#admin_view_title"),
    viewSubtitle: document.querySelector("#admin_view_subtitle"),
    refreshButton: document.querySelector("#admin_refresh_button"),
    statusChip: document.querySelector("#admin_status_chip"),
    identityName: document.querySelector("#admin_identity_name"),
    identityMeta: document.querySelector("#admin_identity_meta"),
    logoutButtonShell: document.querySelector("#admin_logout_shell"),
    authModal: document.querySelector("#admin_auth_modal"),
    authModalClose: document.querySelector("#admin_auth_close"),
    authModalStatus: document.querySelector("#admin_modal_status"),
    authModalHint: document.querySelector("#admin_modal_hint"),
    authModalLogin: document.querySelector("#admin_modal_discord_login")
  };

  function qs(selector, root = document) {
    return root.querySelector(selector);
  }

  function qsa(selector, root = document) {
    return Array.from(root.querySelectorAll(selector));
  }

  function escapeHtml(value) {
    return String(value ?? "").replace(/[&<>"']/g, (char) => {
      const entities = {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      };
      return entities[char] || char;
    });
  }

  function createSafeAuthStorage() {
    const memory = new Map();
    const candidates = [];

    try {
      if (window.sessionStorage) {
        candidates.push(window.sessionStorage);
      }
    } catch (_error) {
      // Storage is unavailable in this browser context.
    }

    try {
      if (window.localStorage) {
        candidates.push(window.localStorage);
      }
    } catch (_error) {
      // Storage is unavailable in this browser context.
    }

    const workingStorage = candidates.find((storage) => {
      try {
        const probeKey = "__estelar_admin_probe__";
        storage.setItem(probeKey, "1");
        storage.removeItem(probeKey);
        return true;
      } catch (_error) {
        return false;
      }
    });

    if (workingStorage) {
      return {
        getItem(key) {
          try {
            return workingStorage.getItem(key);
          } catch (_error) {
            return memory.has(key) ? memory.get(key) : null;
          }
        },
        setItem(key, value) {
          try {
            workingStorage.setItem(key, value);
            return;
          } catch (_error) {
            memory.set(key, value);
          }
        },
        removeItem(key) {
          try {
            workingStorage.removeItem(key);
          } catch (_error) {
            // Ignore broken browser storage and clean fallback below.
          }
          memory.delete(key);
        }
      };
    }

    return {
      getItem(key) {
        return memory.has(key) ? memory.get(key) : null;
      },
      setItem(key, value) {
        memory.set(key, value);
      },
      removeItem(key) {
        memory.delete(key);
      }
    };
  }

  function formatPrice(value) {
    return `${Number(value || 0).toLocaleString("ru-RU")} руб.`;
  }

  function formatDate(value) {
    if (!value) {
      return "—";
    }

    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return "—";
    }

    return date.toLocaleString("ru-RU", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit"
    });
  }

  function formatQuantity(order) {
    const amount = Number(order?.quantity || 1).toLocaleString("ru-RU");
    const label = order?.unitLabel || "ед.";
    return `${amount} ${label}`;
  }

  function formatRelativeDate(value) {
    if (!value) {
      return "—";
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return "—";
    }
    const now = new Date();
    const startOfDay = (d) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
    const dayDiff = Math.round((startOfDay(now) - startOfDay(date)) / (24 * 60 * 60 * 1000));
    const time = date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
    if (dayDiff === 0) {
      return `сегодня ${time}`;
    }
    if (dayDiff === 1) {
      return `вчера ${time}`;
    }
    if (dayDiff > 1 && dayDiff < 7) {
      return `${dayDiff} дн. назад`;
    }
    return date.toLocaleDateString("ru-RU", { day: "2-digit", month: "short", year: dayDiff > 365 ? "numeric" : undefined });
  }

  function getCatalogImageBySlug(slug) {
    if (!slug) {
      return "";
    }
    const item = state.catalog.find((entry) => entry?.slug === slug);
    return item?.image || "";
  }

  function shortOrderId(id) {
    if (!id) {
      return "";
    }
    const tail = String(id).split("-").pop();
    return tail ? `#${tail}` : String(id);
  }

  let authModalCloseTimer = 0;

  function isDirectAdminRoute() {
    const pathname = String(window.location.pathname || "/").replace(/\/+$/, "") || "/";
    return pathname === "/admin";
  }

  function openAnimatedAuthModal() {
    if (!els.authModal) {
      return;
    }

    window.clearTimeout(authModalCloseTimer);
    els.authModal.hidden = false;
    els.authModal.classList.remove("is-closing");
    requestAnimationFrame(() => {
      els.authModal.classList.add("is-open");
    });
  }

  function closeAnimatedAuthModal(onClosed) {
    if (!els.authModal) {
      return;
    }

    els.authModal.classList.remove("is-open");
    els.authModal.classList.add("is-closing");
    window.clearTimeout(authModalCloseTimer);
    authModalCloseTimer = window.setTimeout(() => {
      els.authModal.hidden = true;
      els.authModal.classList.remove("is-closing");
      onClosed?.();
    }, 220);
  }

  function getTitleForPage(page) {
    const projectName = state.meta?.projectName || "ESTELAR.SU";
    switch (page) {
      case "rules":
        return `Правила | ${projectName}`;
      case "contacts":
        return `Контакты | ${projectName}`;
      case "admin":
        return `Admin | ${projectName}`;
      default:
        return `Главная | ${projectName}`;
    }
  }

  function getStatusMeta(status) {
    switch (String(status || "").trim()) {
      case "issued":
        return { label: "Выдано", tone: "success" };
      case "review":
        return { label: "На проверке", tone: "warning" };
      case "rejected":
        return { label: "Отклонено", tone: "error" };
      default:
        return { label: "Ожидает", tone: "pending" };
    }
  }

  function currentHashPage() {
    const page = router?.getPageFromLocation?.() || "home";
    return publicPageSet.has(page) ? page : "home";
  }

  function publicPathForPage(page) {
    if (router?.pathForPage) {
      return router.pathForPage(page);
    }

    return page === "home" ? "/" : `/${page}`;
  }

  function navigateToPage(page, replace = false) {
    if (router?.showPage) {
      router.showPage(page, { replace });
      return;
    }

    activatePage(page);
  }

  function currentVisiblePublicPage() {
    const page = document.querySelector(".page-section.active-page")?.dataset.page || currentHashPage();
    return publicPageSet.has(page) ? page : "home";
  }

  function rememberReturnPage() {
    const page = currentVisiblePublicPage();
    state.publicPageBeforeAdmin = page;
    sessionStorage.setItem(returnPageKey, page);
    return page;
  }

  function getReturnPage() {
    const stored = sessionStorage.getItem(returnPageKey) || state.publicPageBeforeAdmin || currentHashPage();
    return publicPageSet.has(stored) ? stored : "home";
  }

  function hasOAuthPending() {
    return sessionStorage.getItem(oauthPendingKey) === "1";
  }

  function setOAuthPending() {
    rememberReturnPage();
    sessionStorage.setItem(oauthPendingKey, "1");
  }

  function clearOAuthPending() {
    sessionStorage.removeItem(oauthPendingKey);
    sessionStorage.removeItem(returnPageKey);
  }

  function hasOAuthCallbackParams() {
    const url = new URL(window.location.href);
    const hash = url.hash.startsWith("#") ? url.hash.slice(1) : url.hash;
    const hashParams = new URLSearchParams(hash);
    const keys = [
      "code",
      "error",
      "error_description",
      "access_token",
      "refresh_token",
      "provider_token",
      "provider_refresh_token",
      "token_hash"
    ];

    return keys.some((key) => url.searchParams.has(key) || hashParams.has(key));
  }

  function sanitizeLegacyAdminURL() {
    const url = new URL(window.location.href);
    let changed = false;

    if (url.searchParams.has("admin")) {
      url.searchParams.delete("admin");
      changed = true;
    }

    const legacyPage = String(url.hash || "").replace(/^#/, "").trim();
    if (legacyPage) {
      if (legacyPage === "admin" || isDirectAdminRoute()) {
        url.pathname = "/admin";
        url.hash = "";
        changed = true;
      } else if (publicPageSet.has(legacyPage)) {
        url.pathname = publicPathForPage(legacyPage);
        url.hash = "";
        changed = true;
      }
    }

    if (!changed) {
      return;
    }

    const query = url.searchParams.toString();
    const nextURL = `${url.pathname}${query ? `?${query}` : ""}`;
    window.history.replaceState(null, "", nextURL);
  }

  function activatePage(page) {
    const normalized = page === "admin" ? "admin" : publicPageSet.has(page) ? page : "home";
    qsa(".page-section").forEach((section) => {
      section.classList.toggle("active-page", section.dataset.page === normalized);
    });

    qsa("[data-page-link]").forEach((link) => {
      const isActive = normalized !== "admin" && link.dataset.pageLink === normalized;
      link.classList.toggle("active", isActive);
    });

    document.title = getTitleForPage(normalized);
    document.body.dataset.page = normalized;
  }

  function isVisible(element) {
    return Boolean(element) && !element.hidden;
  }

  function syncBodyLock() {
    const storeModal = qs("#item_modal");
    const shouldLock = isVisible(storeModal) || isVisible(els.authModal);
    document.body.classList.toggle("modal-open", shouldLock);
    document.body.style.overflow = shouldLock ? "hidden" : "";
  }

  function setStatusBox(element, message, type = "info") {
    if (!element) {
      return;
    }

    const baseClass = element.className
      .split(" ")
      .filter((token) => token && !["success", "warning", "error", "ready"].includes(token))
      .join(" ");

    element.className = `${baseClass}${type && type !== "info" ? ` ${type}` : ""}`.trim();
    element.textContent = message;
  }

  function setIntroStatus(message, type = "warning") {
    setStatusBox(els.introStatus, message, type);
  }

  function setGateStatus(message, type = "warning") {
    setStatusBox(els.gateStatus, message, type);
  }

  function setModalStatus(message, type = "info") {
    if (!els.authModalStatus) {
      return;
    }

    if (!message) {
      els.authModalStatus.hidden = true;
      els.authModalStatus.textContent = "";
      els.authModalStatus.className = "form_status";
      return;
    }

    els.authModalStatus.hidden = false;
    els.authModalStatus.className = `form_status${type && type !== "info" ? ` ${type}` : ""}`.trim();
    els.authModalStatus.textContent = message;
  }

  function updateAuthAction() {
    if (!els.authModalLogin) {
      return;
    }

    els.authModalLogin.textContent = state.session ? "Открыть панель" : "Войти через Discord";
    if (els.authModalHint) {
      els.authModalHint.textContent = state.session
        ? "Сессия Discord уже найдена. Панель откроется без повторного входа."
        : "После успешного входа панель откроется автоматически.";
    }
  }

  function openAuthModal(message, type = "info") {
    if (!els.authModal) {
      return;
    }

    updateAuthAction();
    setModalStatus(message, type);
    openAnimatedAuthModal();
    syncBodyLock();
  }

  function closeAuthModal() {
    if (!els.authModal) {
      return;
    }

    closeAnimatedAuthModal(() => {
      setModalStatus("");
      syncBodyLock();
      clearOAuthPending();
    });
  }

  function hideAdminShell(restorePublicPage = false) {
    if (els.shell) {
      els.shell.hidden = true;
    }
    if (els.intro) {
      els.intro.style.display = "flex";
    }
    if (els.gate) {
      els.gate.style.display = "block";
    }
    if (restorePublicPage) {
      navigateToPage(getReturnPage(), true);
    }
  }

  function showAdminShell() {
    state.publicPageBeforeAdmin = currentVisiblePublicPage();
    if (els.intro) {
      els.intro.style.display = "none";
    }
    if (els.gate) {
      els.gate.style.display = "none";
    }
    if (els.shell) {
      els.shell.hidden = false;
    }
    navigateToPage("admin", true);
    setAdminView(state.activeView || "overview");
    renderAdminIdentity();
  }

  function setLoginButtonsLoading(loading) {
    [els.loginButton, els.authModalLogin].forEach((button) => {
      if (!button) {
        return;
      }
      button.disabled = loading;
    });

    if (loading) {
      if (els.loginButton) {
        els.loginButton.textContent = "Переход...";
      }
      if (els.authModalLogin) {
        els.authModalLogin.textContent = "Переход...";
      }
      return;
    }

    if (els.loginButton) {
      els.loginButton.textContent = state.session ? "Открыть панель" : "Войти через Discord";
    }
    updateAuthAction();
  }

  function syncAuthButtons() {
    const hasSession = Boolean(state.session);
    if (els.logoutButton) {
      els.logoutButton.hidden = !hasSession;
    }
    if (els.logoutButtonShell) {
      els.logoutButtonShell.hidden = !hasSession;
    }
    if (els.loginButton) {
      els.loginButton.textContent = hasSession ? "Открыть панель" : "Войти через Discord";
    }
    updateAuthAction();
    renderAdminIdentity();
  }

  function getAdminViewMeta(view) {
    switch (view) {
      case "orders":
        return {
          title: "Заказы",
          subtitle: "Очередь заявок, быстрый поиск и смена статуса."
        };
      case "catalog":
        return {
          title: "Каталог",
          subtitle: "Редактирование названий, цен, описаний и активности товаров."
        };
      case "promos":
        return {
          title: "Промокоды",
          subtitle: "Скидки, лимиты и актуальный статус каждого кода."
        };
      case "contacts":
        return {
          title: "Контакты",
          subtitle: "Ссылки кнопок, Telegram, Discord и почта публичной страницы."
        };
      case "integrations":
        return {
          title: "API интеграции",
          subtitle: "Webhook'и для автоматической выдачи привилегий, кейсов и валюты."
        };
      case "payments":
        return {
          title: "Оплата",
          subtitle: "Выбор платёжной системы и ключи провайдера."
        };
      default:
        return {
          title: "Аналитика",
          subtitle: "Просмотры сайта, переходы по страницам и сводка по магазину."
        };
    }
  }

  function setAdminView(view) {
    const allowedViews = new Set(["overview", "orders", "catalog", "promos", "contacts", "integrations", "payments"]);
    const normalized = allowedViews.has(view) ? view : "overview";
    state.activeView = normalized;

    qsa("[data-admin-view-nav]").forEach((button) => {
      button.classList.toggle("active", button.dataset.adminViewNav === normalized);
    });

    qsa("[data-admin-view-panel]").forEach((panel) => {
      panel.classList.toggle("active", panel.dataset.adminViewPanel === normalized);
    });

    const copy = getAdminViewMeta(normalized);
    if (els.viewTitle) {
      els.viewTitle.textContent = copy.title;
    }
    if (els.viewSubtitle) {
      els.viewSubtitle.textContent = copy.subtitle || "";
      els.viewSubtitle.hidden = !copy.subtitle;
    }

    if ((normalized === "integrations" || normalized === "payments") && state.session && !state.integrationsLoading) {
      void loadIntegrations();
    }
  }

  function renderAdminIdentity() {
    const adminName = state.identity?.name || state.identity?.email || state.identity?.id || "Администратор";
    if (els.identityName) {
      els.identityName.textContent = adminName;
    }
    if (els.identityMeta) {
      const provider = state.identity?.provider || "discord";
      els.identityMeta.textContent = state.session ? `Discord-сессия активна • ${provider}` : "Вход в Discord ещё не выполнен";
    }
    if (els.statusChip) {
      els.statusChip.textContent = state.session ? "Доступ открыт" : "Ожидает вход";
    }
  }

  function buildOAuthRedirectURL() {
    const currentOriginPath = `${window.location.origin}${window.location.pathname}`;

    try {
      const url = new URL(currentOriginPath);
      url.searchParams.delete("admin");
      url.searchParams.delete("code");
      url.searchParams.delete("error");
      url.searchParams.delete("error_description");
      url.hash = "";
      return url.toString();
    } catch (_error) {
      return currentOriginPath;
    }
  }

  function wait(delay) {
    return new Promise((resolve) => {
      window.setTimeout(resolve, delay);
    });
  }

  function restoreUnlockedState() {
    state.unlocked = sessionStorage.getItem(unlockStorageKey) === "1";
    if (state.unlocked) {
      setIntroStatus("Шаг 1 выполнен. Комбинация уже принята в этой сессии.", "ready");
      setGateStatus("Шаг 2: войдите через Discord или откройте панель, если сессия уже есть.", "warning");
      return;
    }

    setIntroStatus("Ожидает разблокировку", "warning");
    setGateStatus("Шаг 1: введите скрытую комбинацию, чтобы открыть вход администратора.", "warning");
  }

  function markUnlocked() {
    state.unlocked = true;
    sessionStorage.setItem(unlockStorageKey, "1");
    setIntroStatus("Шаг 1 выполнен. Скрытая комбинация принята.", "ready");
    setGateStatus("Шаг 2: подтвердите вход через Discord.", "warning");
    openAuthModal(
      state.session
        ? "Комбинация принята. Discord-сессия уже найдена, панель можно открыть сразу."
        : "Комбинация принята. Теперь подтвердите вход через Discord.",
      state.session ? "success" : "warning"
    );
  }

  async function fetchJSON(path, options = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000); // 5 сек таймаут

    try {
      const response = await fetch(`${apiBase}${path}`, {
        ...options,
        signal: controller.signal,
        headers: {
          Accept: "application/json",
          ...(options.headers || {})
        }
      });

      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        const error = new Error(payload.error || "Не удалось выполнить запрос.");
        error.status = response.status;
        throw error;
      }

      return payload;
    } catch (error) {
      if (error.name === 'AbortError') {
        throw new Error("Запрос истёк по таймауту (5 сек). API не отвечает.");
      }
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }

  async function authorizedFetch(path, options = {}) {
    const token = state.session?.access_token;
    if (!token) {
      const error = new Error("Сессия Discord не найдена. Войдите заново.");
      error.status = 401;
      throw error;
    }

    return fetchJSON(path, {
      ...options,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(options.headers || {})
      }
    });
  }

  async function loadMeta() {
    const payload = await fetchJSON("/meta");
    state.meta = payload;
    if (els.introText && payload.manualFulfillmentMode) {
      els.introText.textContent = "Панель управляет очередью ручной выдачи, каталогом товаров и промокодами через Supabase и Fiber API.";
    }
    return payload;
  }

  function initSupabase() {
    if (state.supabase || !state.meta) {
      return;
    }

    const createClient = window.supabase?.createClient;
    if (typeof createClient !== "function") {
      setGateStatus("Supabase SDK не загрузился. Проверьте подключение CDN.", "error");
      return;
    }

    if (!state.meta.supabaseUrl || !state.meta.supabasePublicKey) {
      setGateStatus("Supabase Auth еще не настроен для админки.", "error");
      return;
    }

    state.supabase = createClient(state.meta.supabaseUrl, state.meta.supabasePublicKey, {
      auth: {
        storageKey: "estelar-admin-auth",
        storage: createSafeAuthStorage(),
        persistSession: true,
        autoRefreshToken: true,
        detectSessionInUrl: true,
        flowType: "pkce"
      }
    });

    state.supabase.auth.onAuthStateChange((_event, session) => {
      state.session = session || null;
      syncAuthButtons();
      renderAdminIdentity();

      if (!session) {
        state.identity = null;
        renderAdminIdentity();
        return;
      }

      if (hasOAuthPending()) {
        void openAdminIfNeeded();
      }
    });
  }

  async function restoreSession() {
    if (!state.supabase) {
      return;
    }

    const { data, error } = await state.supabase.auth.getSession();
    if (error) {
      throw error;
    }

    state.session = data.session || null;
    syncAuthButtons();
    renderAdminIdentity();
    return state.session;
  }

  async function waitForAuthSession(timeoutMs = 3000) {
    const startedAt = Date.now();
    while (Date.now() - startedAt < timeoutMs) {
      const session = await restoreSession();
      if (session) {
        return true;
      }

      await wait(150);
    }

    return Boolean(state.session);
  }

  async function signInWithDiscord() {
    if (!state.unlocked) {
      openAuthModal("Сначала введите скрытую комбинацию: → → ← ←", "warning");
      return;
    }

    if (state.session) {
      await openAdminIfNeeded();
      return;
    }

    if (!state.supabase) {
      openAuthModal("Supabase Auth еще не настроен. Проверьте ключи и URL проекта.", "error");
      return;
    }

    try {
      setLoginButtonsLoading(true);
      setOAuthPending();
      sanitizeLegacyAdminURL();
      openAuthModal("Перенаправляем в Discord для входа администратора...", "warning");

      const result = await state.supabase.auth.signInWithOAuth({
        provider: "discord",
        options: {
          redirectTo: buildOAuthRedirectURL()
        }
      });

      if (result?.error) {
        throw result.error;
      }
    } catch (error) {
      clearOAuthPending();
      setModalStatus(error.message || "Не удалось начать вход через Discord.", "error");
      setGateStatus(error.message || "Не удалось начать вход через Discord.", "error");
      setLoginButtonsLoading(false);
    }
  }

  async function signOutAdmin() {
    if (state.supabase) {
      await state.supabase.auth.signOut().catch(() => {});
    }

    clearOAuthPending();
    state.session = null;
    state.identity = null;
    state.dashboard = null;
    state.orders = [];
    state.selectedOrderID = "";
    state.activeView = "overview";
    hideAdminShell(true);
    renderMetrics();
    renderRecentOrders();
    renderOrders();
    renderOrderDetail();
    renderAdminIdentity();
    closeAuthModal();
    setGateStatus("Вы вышли из панели. Для повторного входа снова используйте Discord.", "warning");
    syncAuthButtons();
  }

  async function verifyAdminSession() {
    const payload = await authorizedFetch("/admin/session");
    state.identity = payload.identity || null;
    return state.identity;
  }

  function renderMetrics() {
    if (!els.metrics) {
      return;
    }

    if (!state.dashboard) {
      els.metrics.innerHTML = "";
      return;
    }

    const analytics = state.dashboard.analytics || {};
    const metrics = [
      { label: "Сегодня", value: analytics.todayViews || 0 },
      { label: "Уники", value: analytics.todayUnique || 0 },
      { label: "7 дней", value: analytics.last7DaysViews || 0 },
      { label: "В очереди", value: state.dashboard.pendingOrders || 0 },
      { label: "Товаров", value: state.dashboard.activeProducts || 0 },
      { label: "Промо", value: state.dashboard.activePromos || 0 }
    ];

    els.metrics.innerHTML = metrics.map((metric) => `
      <div class="metric_card dash-metric-card">
        <p class="metric_value">${escapeHtml(metric.value)}</p>
        <p class="metric_label">${escapeHtml(metric.label)}</p>
      </div>
    `).join("");
  }

  function renderAnalytics() {
    const analytics = state.dashboard?.analytics;
    if (!els.analyticsSummary || !els.analyticsTrend || !els.pageStats) {
      return;
    }

    if (!analytics) {
      els.analyticsSummary.innerHTML = "";
      els.analyticsTrend.innerHTML = "";
      els.pageStats.innerHTML = '<div class="admin_empty compact">Данных пока нет.</div>';
      return;
    }

    const summaryItems = [
      {
        label: "Сегодня",
        value: analytics.todayViews || 0,
        note: `${Number(analytics.todayUnique || 0).toLocaleString("ru-RU")} уникальных`
      },
      {
        label: "7 дней",
        value: analytics.last7DaysViews || 0,
        note: `${Number(analytics.last7DaysUnique || 0).toLocaleString("ru-RU")} уникальных`
      },
      {
        label: "Всего",
        value: analytics.totalViews || 0,
        note: `${Number(analytics.totalUnique || 0).toLocaleString("ru-RU")} за весь период`
      }
    ];

    els.analyticsSummary.innerHTML = summaryItems.map((item) => `
      <div class="admin_stat_tile">
        <p class="label">${escapeHtml(item.label)}</p>
        <p class="value">${escapeHtml(Number(item.value || 0).toLocaleString("ru-RU"))}</p>
        <p class="note">${escapeHtml(item.note)}</p>
      </div>
    `).join("");

    const trend = Array.isArray(analytics.trend) ? analytics.trend : [];
    const maxViews = Math.max(...trend.map((point) => Number(point.views || 0)), 1);
    els.analyticsTrend.innerHTML = trend.map((point) => {
      const height = Math.max(Math.round((Number(point.views || 0) / maxViews) * 100), Number(point.views || 0) > 0 ? 14 : 6);
      return `
        <div class="admin_trend_bar">
          <div class="admin_trend_meta">
            <strong>${escapeHtml(Number(point.views || 0).toLocaleString("ru-RU"))}</strong>
            <span>${escapeHtml(Number(point.unique || 0).toLocaleString("ru-RU"))} уники</span>
          </div>
          <div class="admin_trend_track">
            <span style="height:${height}%"></span>
          </div>
          <p>${escapeHtml(point.label || point.date || "")}</p>
        </div>
      `;
    }).join("");

    const pages = Array.isArray(analytics.pages) ? analytics.pages : [];
    if (!pages.length) {
      els.pageStats.innerHTML = '<div class="admin_empty compact">Переходов по страницам пока нет.</div>';
      return;
    }

    const maxPageViews = Math.max(...pages.map((page) => Number(page.views || 0)), 1);
    els.pageStats.innerHTML = pages.map((page) => `
      <div class="admin_page_stat_row">
        <div class="copy">
          <p class="label">${escapeHtml(page.label || page.page)}</p>
          <p class="meta">${escapeHtml(Number(page.unique || 0).toLocaleString("ru-RU"))} уникальных</p>
        </div>
        <div class="bar">
          <span style="width:${Math.max(Math.round((Number(page.views || 0) / maxPageViews) * 100), Number(page.views || 0) > 0 ? 10 : 0)}%"></span>
        </div>
        <p class="value">${escapeHtml(Number(page.views || 0).toLocaleString("ru-RU"))}</p>
      </div>
    `).join("");
  }

  async function refreshAdminData() {
    if (!state.session || !els.refreshButton) {
      return;
    }

    const initialLabel = els.refreshButton.textContent;
    els.refreshButton.disabled = true;
    els.refreshButton.textContent = "Обновляем...";

    if (els.statusChip) {
      els.statusChip.textContent = "Синхронизация";
    }

    try {
      await loadDashboard();
      await loadOrders();
      renderOrderDetail();
      setGateStatus("Данные админ-панели обновлены.", "ready");
      setAdminView(state.activeView);
    } catch (error) {
      setGateStatus(error.message || "Не удалось обновить данные панели.", "error");
      if (els.statusChip) {
        els.statusChip.textContent = "Ошибка синхронизации";
      }
    } finally {
      els.refreshButton.disabled = false;
      els.refreshButton.textContent = initialLabel;
      renderAdminIdentity();
    }
  }

  function renderRecentOrders() {
    if (!els.recentOrders) {
      return;
    }

    const recentOrders = Array.isArray(state.dashboard?.recentOrders) ? state.dashboard.recentOrders : [];
    if (!recentOrders.length) {
      els.recentOrders.innerHTML = '<div class="admin_empty compact">Новых заявок пока нет.</div>';
      return;
    }

    els.recentOrders.innerHTML = recentOrders.slice(0, 8).map((order) => {
      const status = getStatusMeta(order.status);
      return `
        <button class="admin_order_card" data-order-id="${escapeHtml(order.id)}" data-open-orders-view="1" type="button">
          <div class="top">
            <p class="name">${escapeHtml(order.productName)}</p>
            <span class="status ${escapeHtml(order.status)} ${escapeHtml(status.tone)}">${escapeHtml(order.statusLabel || status.label)}</span>
          </div>
          <p class="meta">${escapeHtml(order.nickname)} • ${escapeHtml(formatPrice(order.finalPrice))}</p>
          <p class="sub">${escapeHtml(order.id)} • ${escapeHtml(formatDate(order.createdAt))}</p>
        </button>
      `;
    }).join("");
  }

  function fillSettingsForm(settings) {
    if (!els.settingsForm || !settings) {
      return;
    }

    qs("#admin_settings_discord_url", els.settingsForm).value = settings.discordUrl || "";
    qs("#admin_settings_discord_button_label", els.settingsForm).value = settings.discordButtonLabel || "";
    qs("#admin_settings_vk_url", els.settingsForm).value = settings.vkUrl || "";
    qs("#admin_settings_vk_button_label", els.settingsForm).value = settings.vkButtonLabel || "";
    qs("#admin_settings_support_email", els.settingsForm).value = settings.supportEmail || "";
  }

  function renderContactsPreview() {
    if (!els.contactsPreview) {
      return;
    }

    const settings = state.settings;
    if (!settings) {
      els.contactsPreview.innerHTML = '<div class="admin_empty compact">Контакты ещё не загружены.</div>';
      return;
    }

    els.contactsPreview.innerHTML = `
      <div class="admin_contact_preview_card">
        <div class="admin_contact_preview_item">
          <p class="label">Discord</p>
          <p class="link">${escapeHtml(settings.discordUrl || "#")}</p>
          <p class="meta">${escapeHtml(settings.discordButtonLabel || "Подключиться")}</p>
        </div>
        <div class="admin_contact_preview_item">
          <p class="label">Telegram</p>
          <p class="link">${escapeHtml(settings.vkUrl || "#")}</p>
          <p class="meta">${escapeHtml(settings.vkButtonLabel || "Перейти")}</p>
        </div>
        <div class="admin_contact_preview_item">
          <p class="label">Почта</p>
          <p class="link">${escapeHtml(settings.supportEmail || "support@estelar.shop")}</p>
        </div>
      </div>
    `;
  }

  function renderOrderKpis() {
    if (!els.orderKpis) {
      return;
    }

    const counts = {
      total: state.orders.length,
      pending: 0,
      review: 0,
      issued: 0,
      rejected: 0
    };

    state.orders.forEach((order) => {
      if (counts[order.status] !== undefined) {
        counts[order.status] += 1;
      }
    });

    const tiles = [
      { label: "Всего", value: counts.total },
      { label: "Ожидают", value: counts.pending },
      { label: "Проверка", value: counts.review },
      { label: "Выданы", value: counts.issued },
      { label: "Отклонены", value: counts.rejected }
    ];

    els.orderKpis.innerHTML = tiles.map((tile) => `
      <div class="admin_kpi_chip">
        <span>${escapeHtml(tile.label)}</span>
        <strong>${escapeHtml(tile.value)}</strong>
      </div>
    `).join("");
  }

  function renderOrders() {
    if (!els.orderList) {
      return;
    }

    if (!state.orders.length) {
      els.orderList.innerHTML = '<div class="admin_empty table_empty">Заявок по текущему фильтру пока нет.</div>';
      return;
    }

    els.orderList.innerHTML = state.orders.map((order) => {
      const status = getStatusMeta(order.status);
      const image = getCatalogImageBySlug(order.productSlug);
      const compactStatus = compactStatusLabel(order.status, order.statusLabel || status.label);
      const thumb = image
        ? `<span class="admin_order_v2_thumb" style="background-image:url('${escapeHtml(image)}')" aria-hidden="true"></span>`
        : `<span class="admin_order_v2_thumb admin_order_v2_thumb_fallback" aria-hidden="true">${escapeHtml(initialFor(order.productName))}</span>`;
      return `
        <button class="admin_order_v2${order.id === state.selectedOrderID ? " active" : ""}" data-order-id="${escapeHtml(order.id)}" type="button">
          ${thumb}
          <span class="admin_order_v2_main">
            <span class="admin_order_v2_title">${escapeHtml(order.productName)}</span>
            <span class="admin_order_v2_meta">${escapeHtml(order.nickname)} • ${escapeHtml(shortOrderId(order.id))}</span>
          </span>
          <span class="admin_order_v2_side">
            <span class="admin_order_v2_price">${escapeHtml(formatPrice(order.finalPrice))}</span>
            <span class="admin_order_v2_date">${escapeHtml(formatRelativeDate(order.createdAt))}</span>
          </span>
          <span class="status status_pill ${escapeHtml(order.status)} ${escapeHtml(status.tone)}">${escapeHtml(compactStatus)}</span>
        </button>
      `;
    }).join("");
  }

  function compactStatusLabel(status, fallback) {
    switch (status) {
      case "issued":
        return "Выдан";
      case "review":
        return "На проверке";
      case "rejected":
        return "Отклонён";
      case "pending":
        return "Ожидает";
      default:
        return fallback || "Ожидает";
    }
  }

  function initialFor(text) {
    const trimmed = String(text || "").trim();
    return trimmed ? trimmed.charAt(0).toUpperCase() : "?";
  }

  function getSelectedOrder() {
    return state.orders.find((order) => order.id === state.selectedOrderID) || null;
  }

  function renderOrderDetail() {
    if (!els.orderDetail) {
      return;
    }

    const order = getSelectedOrder();
    if (!order) {
      els.orderDetail.className = "admin_order_detail empty";
      els.orderDetail.textContent = "Выберите заявку слева, чтобы посмотреть детали и отметить выдачу.";
      return;
    }

    const quantityLine = order.variablePrice || Number(order.quantity || 1) > 1
      ? `<p>Количество: <span>${escapeHtml(formatQuantity(order))}</span></p>`
      : "";
    const unitLine = order.variablePrice
      ? `<p>Цена за единицу: <span>${escapeHtml(formatPrice(order.unitPrice))}</span></p>`
      : "";

    const image = getCatalogImageBySlug(order.productSlug);
    const heroThumb = image
      ? `<span class="admin_order_focus_thumb" style="background-image:url('${escapeHtml(image)}')" aria-hidden="true"></span>`
      : `<span class="admin_order_focus_thumb admin_order_focus_thumb_fallback" aria-hidden="true">${escapeHtml(initialFor(order.productName))}</span>`;
    const statusMeta = getStatusMeta(order.status);

    els.orderDetail.className = "admin_order_detail";
    els.orderDetail.innerHTML = `
      <div class="admin_order_focus">
        <div class="admin_order_focus_hero">
          ${heroThumb}
          <div class="admin_order_focus_hero_copy">
            <p class="eyebrow"><code>${escapeHtml(order.id)}</code></p>
            <h3>${escapeHtml(order.productName)}</h3>
            <p class="meta">Игрок <strong>${escapeHtml(order.nickname)}</strong> • ${escapeHtml(order.categoryLabel)} • ${escapeHtml(order.periodLabel)}</p>
          </div>
          <div class="admin_order_focus_hero_side">
            <span class="status status_pill ${escapeHtml(order.status)} ${escapeHtml(statusMeta.tone)}">${escapeHtml(compactStatusLabel(order.status, order.statusLabel))}</span>
            <strong class="admin_order_focus_price">${escapeHtml(formatPrice(order.finalPrice))}</strong>
          </div>
        </div>

        <div class="admin_info_grid admin_info_grid_compact">
          ${quantityLine ? `<div class="admin_info_item"><span>Количество</span><strong>${escapeHtml(formatQuantity(order))}</strong></div>` : ""}
          ${unitLine ? `<div class="admin_info_item"><span>За единицу</span><strong>${escapeHtml(formatPrice(order.unitPrice))}</strong></div>` : ""}
          <div class="admin_info_item">
            <span>Промокод</span>
            <strong>${escapeHtml(order.promo?.code || "—")}</strong>
          </div>
          <div class="admin_info_item">
            <span>Обработал</span>
            <strong>${escapeHtml(order.handledBy || "—")}</strong>
          </div>
          <div class="admin_info_item">
            <span>Создан</span>
            <strong>${escapeHtml(formatRelativeDate(order.createdAt))}</strong>
          </div>
          <div class="admin_info_item">
            <span>Обновлен</span>
            <strong>${escapeHtml(formatRelativeDate(order.updatedAt))}</strong>
          </div>
        </div>
      </div>
      <form class="admin_form compact admin_order_editor" id="admin_order_update_form">
        <div class="admin_order_editor_grid">
          <label class="admin_field">
            <span>Статус</span>
            <select id="admin_order_update_status">
              <option value="pending"${order.status === "pending" ? " selected" : ""}>Ожидает</option>
              <option value="review"${order.status === "review" ? " selected" : ""}>На проверке</option>
              <option value="issued"${order.status === "issued" ? " selected" : ""}>Выдано</option>
              <option value="rejected"${order.status === "rejected" ? " selected" : ""}>Отклонено</option>
            </select>
          </label>
        </div>
        <label class="admin_field admin_field_full">
          <span>Комментарий</span>
          <textarea id="admin_order_update_note" placeholder="Внутренняя заметка">${escapeHtml(order.adminNote || "")}</textarea>
        </label>
        <button type="submit">Сохранить</button>
      </form>
    `;

    qs("#admin_order_update_form", els.orderDetail)?.addEventListener("submit", updateOrderStatus);
  }

  function getFilteredCatalog() {
    const query = els.catalogSearch?.value.trim().toLowerCase() || "";
    if (!query) {
      return state.catalog;
    }

    return state.catalog.filter((item) => {
      const haystack = [item.name, item.slug, item.categoryLabel].join(" ").toLowerCase();
      return haystack.includes(query);
    });
  }

  function renderCatalogList() {
    if (!els.catalogList) {
      return;
    }

    const items = getFilteredCatalog();
    if (!items.length) {
      els.catalogList.innerHTML = '<div class="admin_empty table_empty">Ничего не найдено.</div>';
      return;
    }

    els.catalogList.innerHTML = items.map((item) => {
      const thumb = item.image
        ? `<span class="admin_order_v2_thumb" style="background-image:url('${escapeHtml(item.image)}')" aria-hidden="true"></span>`
        : `<span class="admin_order_v2_thumb admin_order_v2_thumb_fallback" aria-hidden="true">${escapeHtml(initialFor(item.name))}</span>`;
      const stateClass = item.isActive ? "issued success" : "pending warning";
      const stateLabel = item.isActive ? "Активен" : "Скрыт";
      return `
        <button class="admin_order_v2${item.slug === state.selectedCatalogSlug ? " active" : ""}" data-catalog-slug="${escapeHtml(item.slug)}" type="button">
          ${thumb}
          <span class="admin_order_v2_main">
            <span class="admin_order_v2_title">${escapeHtml(item.name)}</span>
            <span class="admin_order_v2_meta">${escapeHtml(item.categoryLabel)} • <code>${escapeHtml(item.slug)}</code></span>
          </span>
          <span class="admin_order_v2_side">
            <span class="admin_order_v2_price">${escapeHtml(formatPrice(item.price))}</span>
          </span>
          <span class="status status_pill ${escapeHtml(stateClass)}">${escapeHtml(stateLabel)}</span>
        </button>
      `;
    }).join("");
  }

  function renderPromoList() {
    if (!els.promoList) {
      return;
    }

    if (!state.promos.length) {
      els.promoList.innerHTML = '<div class="admin_empty">Промокодов пока нет.</div>';
      return;
    }

    els.promoList.innerHTML = state.promos.map((promo) => `
      <button class="admin_promo_row admin_order_v2 admin_promo_card${promo.code === state.selectedPromoCode ? " active" : ""}" data-promo-code="${escapeHtml(promo.code)}" type="button">
        <span class="admin_order_v2_thumb admin_promo_badge" aria-hidden="true">−${escapeHtml(promo.discountPercent)}%</span>
        <span class="admin_order_v2_main">
          <span class="admin_order_v2_title"><code>${escapeHtml(promo.code)}</code></span>
          <span class="admin_order_v2_meta">Использований ${escapeHtml(promo.timesUsed)} • Лимит ${escapeHtml(promo.usageLimit ?? "без лимита")}</span>
        </span>
        <span class="status status_pill ${promo.isActive ? "issued success" : "pending warning"}">${promo.isActive ? "Активен" : "Отключен"}</span>
      </button>
    `).join("");
  }

  function fillCatalogForm(item) {
    if (!els.catalogForm || !item) {
      return;
    }

    qs("#admin_catalog_slug", els.catalogForm).value = item.slug || "";
    qs("#admin_catalog_name", els.catalogForm).value = item.name || "";
    qs("#admin_catalog_category", els.catalogForm).value = item.category || "privilege";
    qs("#admin_catalog_price", els.catalogForm).value = item.price ?? 0;
    qs("#admin_catalog_image", els.catalogForm).value = item.image || "";
    qs("#admin_catalog_sort", els.catalogForm).value = item.sort ?? 1;
    qs("#admin_catalog_summary", els.catalogForm).value = item.summary || "";
    qs("#admin_catalog_highlights", els.catalogForm).value = Array.isArray(item.highlights) ? item.highlights.join("\n") : "";
    qs("#admin_catalog_active", els.catalogForm).checked = Boolean(item.isActive);
  }

  function fillPromoForm(promo) {
    if (!els.promoForm || !promo) {
      return;
    }

    qs("#admin_promo_code", els.promoForm).value = promo.code || "";
    qs("#admin_promo_discount", els.promoForm).value = promo.discountPercent ?? 15;
    qs("#admin_promo_limit", els.promoForm).value = promo.usageLimit ?? "";
    qs("#admin_promo_active", els.promoForm).checked = Boolean(promo.isActive);
  }

  async function loadDashboard() {
    const dashboard = await authorizedFetch("/admin/dashboard");
    state.dashboard = dashboard;
    state.identity = dashboard.identity || state.identity;
    state.catalog = Array.isArray(dashboard.catalog) ? dashboard.catalog : [];
    state.promos = Array.isArray(dashboard.promos) ? dashboard.promos : [];
    state.settings = dashboard.settings || state.settings;
    if (!state.selectedCatalogSlug || !state.catalog.some((item) => item.slug === state.selectedCatalogSlug)) {
      state.selectedCatalogSlug = state.catalog[0]?.slug || "";
    }
    if (!state.selectedPromoCode || !state.promos.some((promo) => promo.code === state.selectedPromoCode)) {
      state.selectedPromoCode = state.promos[0]?.code || "";
    }
    renderMetrics();
    renderAnalytics();
    renderRecentOrders();
    renderCatalogList();
    renderPromoList();
    fillCatalogForm(state.catalog.find((item) => item.slug === state.selectedCatalogSlug));
    fillPromoForm(state.promos.find((promo) => promo.code === state.selectedPromoCode));
    fillSettingsForm(state.settings);
    renderContactsPreview();
    renderAdminIdentity();
  }

  async function loadOrders() {
    if (state.orderLoading) {
      return;
    }

    state.orderLoading = true;
    try {
      const params = new URLSearchParams();
      const status = els.orderStatusFilter?.value.trim() || "";
      const query = els.orderSearch?.value.trim() || "";
      if (status) {
        params.set("status", status);
      }
      if (query) {
        params.set("query", query);
      }
      params.set("limit", "200");

      const payload = await authorizedFetch(`/admin/orders?${params.toString()}`);
      state.orders = Array.isArray(payload.orders) ? payload.orders : [];
      if (!state.orders.some((order) => order.id === state.selectedOrderID)) {
        state.selectedOrderID = state.orders[0]?.id || "";
      }
      renderOrderKpis();
      renderOrders();
      renderOrderDetail();
    } finally {
      state.orderLoading = false;
    }
  }

  async function updateOrderStatus(event) {
    event.preventDefault();

    const order = getSelectedOrder();
    if (!order) {
      return;
    }

    const status = qs("#admin_order_update_status", els.orderDetail)?.value || "pending";
    const adminNote = qs("#admin_order_update_note", els.orderDetail)?.value.trim() || "";

    try {
      const payload = await authorizedFetch(`/admin/orders/${encodeURIComponent(order.id)}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          status,
          adminNote
        })
      });

      const updated = payload.order;
      state.orders = state.orders.map((entry) => entry.id === updated.id ? updated : entry);
      state.selectedOrderID = updated.id;
      renderOrderKpis();
      renderOrders();
      renderOrderDetail();
      await loadDashboard();
      setGateStatus(`Заявка ${updated.id} обновлена.`, "success");
    } catch (error) {
      setGateStatus(error.message || "Не удалось обновить заявку.", "error");
    }
  }

  async function saveCatalogItem(event) {
    event.preventDefault();
    if (!els.catalogForm) {
      return;
    }

    const slug = qs("#admin_catalog_slug", els.catalogForm)?.value.trim() || "";
    const highlights = (qs("#admin_catalog_highlights", els.catalogForm)?.value || "")
      .split("\n")
      .map((value) => value.trim())
      .filter(Boolean);
    const category = qs("#admin_catalog_category", els.catalogForm)?.value || "privilege";
    const existingItem = state.catalog.find((item) => item.slug === slug);
    const categoryLabel = existingItem?.categoryLabel
      || (slug === "donate-currency" ? "Валюта" : category === "case" ? "Кейс" : "Привилегия");

    try {
      await authorizedFetch("/admin/catalog", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          slug,
          name: qs("#admin_catalog_name", els.catalogForm)?.value.trim() || "",
          category,
          categoryLabel,
          price: Number(qs("#admin_catalog_price", els.catalogForm)?.value || 0),
          image: qs("#admin_catalog_image", els.catalogForm)?.value.trim() || "",
          sort: Number(qs("#admin_catalog_sort", els.catalogForm)?.value || 1),
          summary: qs("#admin_catalog_summary", els.catalogForm)?.value.trim() || "",
          highlights,
          isActive: Boolean(qs("#admin_catalog_active", els.catalogForm)?.checked)
        })
      });

      state.selectedCatalogSlug = slug;
      await loadDashboard();
      setGateStatus(`Товар ${slug} сохранен.`, "success");
    } catch (error) {
      setGateStatus(error.message || "Не удалось сохранить товар.", "error");
    }
  }

  async function savePromo(event) {
    event.preventDefault();
    if (!els.promoForm) {
      return;
    }

    const code = qs("#admin_promo_code", els.promoForm)?.value.trim().toUpperCase() || "";
    const rawLimit = qs("#admin_promo_limit", els.promoForm)?.value.trim() || "";

    try {
      await authorizedFetch("/admin/promos", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          code,
          discountPercent: Number(qs("#admin_promo_discount", els.promoForm)?.value || 15),
          usageLimit: rawLimit ? Number(rawLimit) : null,
          isActive: Boolean(qs("#admin_promo_active", els.promoForm)?.checked)
        })
      });

      state.selectedPromoCode = code;
      await loadDashboard();
      setGateStatus(`Промокод ${code} сохранен.`, "success");
    } catch (error) {
      setGateStatus(error.message || "Не удалось сохранить промокод.", "error");
    }
  }

  async function saveSettings(event) {
    event.preventDefault();
    if (!els.settingsForm) {
      return;
    }

    try {
      const payload = await authorizedFetch("/admin/settings", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          contactTitle: state.settings?.contactTitle || "",
          discordUrl: qs("#admin_settings_discord_url", els.settingsForm)?.value.trim() || "",
          discordButtonLabel: qs("#admin_settings_discord_button_label", els.settingsForm)?.value.trim() || "",
          discordText: state.settings?.discordText || "",
          vkUrl: qs("#admin_settings_vk_url", els.settingsForm)?.value.trim() || "",
          vkButtonLabel: qs("#admin_settings_vk_button_label", els.settingsForm)?.value.trim() || "",
          vkText: state.settings?.vkText || "",
          supportEmail: qs("#admin_settings_support_email", els.settingsForm)?.value.trim() || "",
          supportText: state.settings?.supportText || ""
        })
      });

      state.settings = payload.settings || state.settings;
      fillSettingsForm(state.settings);
      renderContactsPreview();
      window.dispatchEvent(new CustomEvent("estelar:settings-updated", {
        detail: {
          settings: state.settings
        }
      }));
      setGateStatus("Контакты сохранены и уже обновлены на сайте.", "success");
    } catch (error) {
      setGateStatus(error.message || "Не удалось сохранить контакты.", "error");
    }
  }

  async function openAdminIfNeeded() {
    if (state.adminOpening) {
      return;
    }

    const isOAuthFlow = hasOAuthCallbackParams() || hasOAuthPending();
    const directAdminRoute = isDirectAdminRoute();

    if (!state.unlocked && !isOAuthFlow && !directAdminRoute) {
      openAuthModal("Сначала введите скрытую комбинацию: → → ← ←", "warning");
      return;
    }

    if (!state.session) {
      openAuthModal("Войдите через Discord, чтобы открыть админ-панель.", "warning");
      return;
    }

    state.adminOpening = true;
    try {
      setGateStatus("Проверяем права администратора...", "warning");
      await verifyAdminSession();
      await loadDashboard();
      await loadOrders();
      showAdminShell();

      const adminName = state.identity?.name || state.identity?.email || state.identity?.id || "администратор";
      setIntroStatus(`Доступ открыт для ${adminName}.`, "ready");
      setGateStatus("Панель готова к работе.", "success");
      clearOAuthPending();
      sanitizeLegacyAdminURL();
      closeAuthModal();
    } catch (error) {
      hideAdminShell(false);
      const message = error.message || "Не удалось открыть админ-панель.";
      setIntroStatus(message, "error");
      setGateStatus(message, "error");
      openAuthModal(message, "error");
      clearOAuthPending();
      if ((error.status === 401 || error.status === 403) && state.supabase) {
        await state.supabase.auth.signOut().catch(() => {});
      }
    } finally {
      state.adminOpening = false;
      setLoginButtonsLoading(false);
      syncAuthButtons();
    }
  }

  function selectOrder(orderID) {
    state.selectedOrderID = orderID;
    renderOrders();
    renderOrderDetail();
  }

  function attachEvents() {
    els.loginButton?.addEventListener("click", () => {
      if (state.session) {
        openAdminIfNeeded();
        return;
      }
      signInWithDiscord();
    });

    els.authModalLogin?.addEventListener("click", () => {
      if (state.session) {
        openAdminIfNeeded();
        return;
      }
      signInWithDiscord();
    });

    els.logoutButton?.addEventListener("click", signOutAdmin);
    els.logoutButtonShell?.addEventListener("click", signOutAdmin);
    els.refreshButton?.addEventListener("click", refreshAdminData);
    els.catalogForm?.addEventListener("submit", saveCatalogItem);
    els.promoForm?.addEventListener("submit", savePromo);
    els.settingsForm?.addEventListener("submit", saveSettings);
    els.authModalClose?.addEventListener("click", closeAuthModal);

    qsa("[data-admin-view-nav]").forEach((button) => {
      button.addEventListener("click", () => {
        setAdminView(button.dataset.adminViewNav || "overview");
      });
    });

    attachIntegrationEvents();

    els.authModal?.addEventListener("click", (event) => {
      if (event.target === els.authModal) {
        closeAuthModal();
      }
    });

    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && isVisible(els.authModal)) {
        closeAuthModal();
      }

      if (!["ArrowRight", "ArrowLeft"].includes(event.key)) {
        state.unlockBuffer = [];
        return;
      }

      state.unlockBuffer.push(event.key);
      if (state.unlockBuffer.length > unlockPattern.length) {
        state.unlockBuffer.shift();
      }

      if (unlockPattern.every((key, index) => state.unlockBuffer[index] === key)) {
        state.unlockBuffer = [];
        markUnlocked();
      }
    });

    els.orderList?.addEventListener("click", (event) => {
      const button = event.target.closest("[data-order-id]");
      if (!button) {
        return;
      }
      selectOrder(button.dataset.orderId || "");
    });

    els.recentOrders?.addEventListener("click", (event) => {
      const button = event.target.closest("[data-order-id]");
      if (!button) {
        return;
      }

      state.selectedOrderID = button.dataset.orderId || "";
      setAdminView("orders");
      renderOrders();
      renderOrderDetail();
    });

    els.catalogList?.addEventListener("click", (event) => {
      const button = event.target.closest("[data-catalog-slug]");
      if (!button) {
        return;
      }

      state.selectedCatalogSlug = button.dataset.catalogSlug || "";
      const item = state.catalog.find((entry) => entry.slug === state.selectedCatalogSlug);
      renderCatalogList();
      fillCatalogForm(item);
    });

    els.promoList?.addEventListener("click", (event) => {
      const button = event.target.closest("[data-promo-code]");
      if (!button) {
        return;
      }

      state.selectedPromoCode = button.dataset.promoCode || "";
      const promo = state.promos.find((entry) => entry.code === state.selectedPromoCode);
      renderPromoList();
      fillPromoForm(promo);
    });

    els.orderStatusFilter?.addEventListener("change", loadOrders);
    els.orderSearch?.addEventListener("input", () => {
      window.clearTimeout(els.orderSearch._searchTimer);
      els.orderSearch._searchTimer = window.setTimeout(loadOrders, 250);
    });
    els.catalogSearch?.addEventListener("input", () => {
      renderCatalogList();
    });

    window.addEventListener("estelar:pagechange", (event) => {
        const page = event.detail?.page;
        if (page === "admin") {
          if (state.session) {
          if (els.shell?.hidden) {
              void openAdminIfNeeded();
            }
            return;
        }

        hideAdminShell(false);
        activatePage("admin");
        openAuthModal("Войдите через Discord, чтобы открыть админ-панель.", "warning");
        return;
      }

      if (page && publicPageSet.has(page)) {
        hideAdminShell(false);
      }
    });
  }

  const INTEGRATION_CATEGORIES = [
    {
      key: "privilegesEndpoint",
      category: "privilege",
      title: "Привилегии",
      icon: "🛡",
      description: "Webhook вызывается при выдаче VIP/Premium/Deluxe и пр. — плагин ставит привилегию игроку."
    },
    {
      key: "casesEndpoint",
      category: "case",
      title: "Кейсы",
      icon: "📦",
      description: "Webhook вызывается при выдаче кейсов — плагин начисляет ключи или открывает кейс."
    },
    {
      key: "currencyEndpoint",
      category: "currency",
      title: "Донат-валюта",
      icon: "💰",
      description: "Webhook вызывается при покупке внутриигровой валюты — плагин зачисляет монеты."
    }
  ];

  function authorizedJSON(method, path, body) {
    return authorizedFetch(`${apiBase}${path}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined
    });
  }

  async function loadIntegrations() {
    if (state.integrationsLoading) {
      return;
    }
    state.integrationsLoading = true;
    try {
      const response = await authorizedFetch(`${apiBase}/admin/integrations`);
      if (!response.ok) {
        throw new Error(`Не удалось загрузить интеграции (${response.status}).`);
      }
      const data = await response.json();
      state.integrations = data.integrations || {};
      state.payments = data.payments || { provider: "manual" };
      state.paymentProviders = Array.isArray(data.providers) ? data.providers : [];
      state.integrationsLoaded = true;
      renderIntegrations();
      renderPayments();
    } catch (error) {
      setAdminFormStatus(els.integrationsStatus, error.message || "Ошибка загрузки.", "error");
    } finally {
      state.integrationsLoading = false;
    }
  }

  function setAdminFormStatus(element, message, tone) {
    if (!element) {
      return;
    }
    if (!message) {
      element.hidden = true;
      element.textContent = "";
      element.className = "form_status";
      return;
    }
    element.hidden = false;
    element.textContent = message;
    element.className = `form_status ${tone || "ready"}`;
  }

  function renderIntegrations() {
    const grid = document.querySelector("#admin_integrations_grid");
    if (!grid) {
      return;
    }
    const cfg = state.integrations || {};
    grid.innerHTML = INTEGRATION_CATEGORIES.map((meta) => {
      const endpoint = cfg[meta.key] || {};
      const enabled = Boolean(endpoint.enabled);
      return `
        <article class="dash-card admin_integration_card" data-integration-key="${escapeHtml(meta.key)}" data-integration-category="${escapeHtml(meta.category)}">
          <header class="admin_integration_card_head">
            <span class="admin_integration_icon" aria-hidden="true">${meta.icon}</span>
            <div>
              <div class="admin_integration_title">${escapeHtml(meta.title)}</div>
              <div class="admin_integration_sub">${escapeHtml(meta.description)}</div>
            </div>
            <label class="admin_integration_toggle">
              <input type="checkbox" data-integration-enabled ${enabled ? "checked" : ""} />
              <span>Включить</span>
            </label>
          </header>
          <div class="admin_integration_body">
            <label class="admin_field admin_field_full">
              <span>URL webhook</span>
              <input type="url" data-integration-url placeholder="https://your-mc-server.example/api/issue" value="${escapeHtml(endpoint.url || "")}" />
            </label>
            <label class="admin_field admin_field_full">
              <span>Bearer-токен (опционально)</span>
              <input type="password" data-integration-token placeholder="${endpoint.token ? "•••••••• — оставьте пустым, чтобы не менять" : "Заголовок Authorization: Bearer …"}" autocomplete="off" />
            </label>
            <div class="admin_integration_actions">
              <button class="dash-btn dash-btn_ghost" type="button" data-integration-test>Тестовый запрос</button>
              <span class="form_status admin_integration_test_status" data-integration-test-status hidden></span>
            </div>
          </div>
        </article>
      `;
    }).join("");

    const secret = document.querySelector("#admin_integration_secret");
    if (secret) {
      if (cfg.webhookSecret) {
        secret.placeholder = "•••••••• — оставьте пустым, чтобы не менять";
        secret.value = "";
      } else {
        secret.placeholder = "Сгенерируйте длинную строку 32+ символов";
      }
    }
  }

  function readIntegrationsFromDOM() {
    const grid = document.querySelector("#admin_integrations_grid");
    if (!grid) {
      return null;
    }
    const out = { webhookSecret: document.querySelector("#admin_integration_secret")?.value || "" };
    INTEGRATION_CATEGORIES.forEach((meta) => {
      const card = grid.querySelector(`[data-integration-key="${meta.key}"]`);
      if (!card) {
        return;
      }
      out[meta.key] = {
        enabled: Boolean(card.querySelector("[data-integration-enabled]")?.checked),
        url: card.querySelector("[data-integration-url]")?.value?.trim() || "",
        token: card.querySelector("[data-integration-token]")?.value || ""
      };
    });
    return out;
  }

  async function saveIntegrations() {
    const button = document.querySelector("#admin_integrations_save");
    const status = document.querySelector("#admin_integrations_status");
    const payload = readIntegrationsFromDOM();
    if (!payload) {
      return;
    }
    if (button) {
      button.disabled = true;
      button.textContent = "Сохраняем...";
    }
    setAdminFormStatus(status, "", "");
    try {
      const response = await authorizedJSON("POST", "/admin/integrations", payload);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data?.error || `Ошибка сохранения (${response.status}).`);
      }
      state.integrations = data.integrations || state.integrations;
      renderIntegrations();
      setAdminFormStatus(status, "Настройки сохранены.", "success");
    } catch (error) {
      setAdminFormStatus(status, error.message || "Не удалось сохранить.", "error");
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = "Сохранить настройки";
      }
    }
  }

  async function testIntegration(category, button, statusEl) {
    if (!category) {
      return;
    }
    button.disabled = true;
    const initialLabel = button.textContent;
    button.textContent = "Запрос...";
    setAdminFormStatus(statusEl, "", "");
    try {
      const response = await authorizedJSON("POST", "/admin/integrations/test", { category });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data?.error || `Ошибка теста (${response.status}).`);
      }
      const result = data.result || {};
      setAdminFormStatus(statusEl, result.message || (result.ok ? "OK" : "Нет ответа."), result.ok ? "success" : "warning");
    } catch (error) {
      setAdminFormStatus(statusEl, error.message || "Сбой теста.", "error");
    } finally {
      button.disabled = false;
      button.textContent = initialLabel;
    }
  }

  function renderPayments() {
    const providersBox = document.querySelector("#admin_payment_providers");
    if (!providersBox) {
      return;
    }
    const providers = state.paymentProviders || [];
    const current = state.payments?.provider || "manual";
    providersBox.innerHTML = providers.map((provider) => {
      const checked = provider.id === current ? "checked" : "";
      const ready = provider.ready ? "" : "<span class=\"admin_payment_card_badge\">Скоро</span>";
      return `
        <label class="admin_payment_card${provider.ready ? "" : " is-stub"}">
          <input type="radio" name="admin_payment_provider" value="${escapeHtml(provider.id)}" ${checked} />
          <span class="admin_payment_card_inner">
            <span class="admin_payment_card_title">${escapeHtml(provider.label)}${ready}</span>
            <span class="admin_payment_card_desc">${escapeHtml(provider.description)}</span>
          </span>
        </label>
      `;
    }).join("");

    const payments = state.payments || {};
    const map = {
      "#admin_payment_shop": payments.shopId || "",
      "#admin_payment_public": payments.publicKey || "",
      "#admin_payment_return": payments.returnUrl || "",
      "#admin_payment_webhook": payments.webhookUrl || "",
      "#admin_payment_description": payments.description || ""
    };
    Object.entries(map).forEach(([selector, value]) => {
      const el = document.querySelector(selector);
      if (el) {
        el.value = value;
      }
    });
    const secretField = document.querySelector("#admin_payment_secret");
    if (secretField) {
      secretField.value = "";
      secretField.placeholder = payments.secretKey ? "•••••••• — оставьте пустым, чтобы не менять" : "Секретный ключ провайдера";
    }
    const test = document.querySelector("#admin_payment_test");
    if (test) {
      test.checked = Boolean(payments.testMode);
    }
  }

  async function savePayments(event) {
    event?.preventDefault?.();
    const status = document.querySelector("#admin_payment_status");
    const provider = document.querySelector("input[name=\"admin_payment_provider\"]:checked")?.value || "manual";
    const payload = {
      provider,
      testMode: Boolean(document.querySelector("#admin_payment_test")?.checked),
      shopId: document.querySelector("#admin_payment_shop")?.value || "",
      publicKey: document.querySelector("#admin_payment_public")?.value || "",
      secretKey: document.querySelector("#admin_payment_secret")?.value || "",
      returnUrl: document.querySelector("#admin_payment_return")?.value || "",
      webhookUrl: document.querySelector("#admin_payment_webhook")?.value || "",
      description: document.querySelector("#admin_payment_description")?.value || ""
    };
    setAdminFormStatus(status, "", "");
    try {
      const response = await authorizedJSON("POST", "/admin/payments", payload);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data?.error || `Ошибка сохранения (${response.status}).`);
      }
      state.payments = data.payments || state.payments;
      renderPayments();
      setAdminFormStatus(status, "Настройки оплаты сохранены.", "success");
    } catch (error) {
      setAdminFormStatus(status, error.message || "Не удалось сохранить.", "error");
    }
  }

  function attachIntegrationEvents() {
    document.querySelector("#admin_integrations_save")?.addEventListener("click", saveIntegrations);
    document.querySelector("#admin_integrations_grid")?.addEventListener("click", (event) => {
      const button = event.target.closest("[data-integration-test]");
      if (!button) {
        return;
      }
      const card = button.closest("[data-integration-category]");
      const statusEl = card?.querySelector("[data-integration-test-status]");
      const category = card?.dataset?.integrationCategory;
      void testIntegration(category, button, statusEl);
    });
    document.querySelector("#admin_payments_form")?.addEventListener("submit", savePayments);
    document.querySelector("#admin_payment_providers")?.addEventListener("change", () => {
      // Если выбрали ручную выдачу — поля становятся опциональными.
    });
  }

  async function init() {
    if (!els.gate || !els.shell) {
      return;
    }

    const hasOAuthCallback = hasOAuthCallbackParams();
    const hadOAuthPending = hasOAuthPending();
    const directAdminRoute = isDirectAdminRoute();

    if (hasOAuthCallback) {
      setOAuthPending();
    }

    if (els.authModal) {
      els.authModal.hidden = true;
    }

    document.body.style.overflow = "";

    restoreUnlockedState();
    attachEvents();
    sanitizeLegacyAdminURL();
    hideAdminShell(false);
    if (directAdminRoute) {
      activatePage("admin");
    }

    try {
      await loadMeta();
      initSupabase();

      const shouldResumeOAuth = hasOAuthCallback || hadOAuthPending;
      const hasSession = shouldResumeOAuth
        ? await waitForAuthSession(3000)
        : Boolean(await restoreSession());

      if (hasSession && (hasOAuthPending() || directAdminRoute)) {
        await openAdminIfNeeded();
      } else if (shouldResumeOAuth) {
        clearOAuthPending();
        openAuthModal("Не удалось завершить вход через Discord. Похоже, браузер не сохранил OAuth-сессию. Попробуйте еще раз и при необходимости откройте сайт без конфликтующих расширений.", "error");
      } else if (directAdminRoute) {
        openAuthModal("Войдите через Discord, чтобы открыть админ-панель.", "warning");
      }
    } catch (error) {
      setIntroStatus(error.message || "Не удалось инициализировать админ-панель.", "error");
      setGateStatus(error.message || "Не удалось инициализировать админ-панель.", "error");
      clearOAuthPending();
    } finally {
      syncAuthButtons();
      syncBodyLock();
    }
  }

  init();
})();
