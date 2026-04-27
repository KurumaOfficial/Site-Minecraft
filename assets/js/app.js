/* Автор: Kuruma */
﻿(function() {
  const defaultProjectName = "ESTELAR.SU";
  const apiBase = (window.HOLO_CONFIG?.apiBase || "/api/v1").replace(/\/$/, "");
  const paymentProviders = [
    { code: "cards", label: "Банковские карты", note: "Подключение ЮKassa / CloudPayments", enabled: false },
    { code: "sbp", label: "СБП", note: "Быстрая оплата по QR", enabled: false },
    { code: "wallet", label: "Электронный кошелек", note: "Резервный платёжный метод", enabled: false }
  ];
  const publicPages = ["home", "rules", "contacts"];
  const pages = [...publicPages, "admin"];
  const pageRoutes = {
    home: "/",
    rules: "/rules",
    contacts: "/contacts",
    admin: "/admin"
  };
  const state = {
    meta: null,
    catalog: [],
    selectedSlug: "",
    modalItem: null,
    quoteTimer: 0,
    lastQuote: null,
    modalCloseTimer: 0,
    lastTrackedPage: "",
    lastTrackedAt: 0
  };
  const titles = createTitles(defaultProjectName);

  const modal = document.querySelector("#item_modal");
  const buyForm = document.querySelector("#buy_form");
  const donateInfo = document.querySelector("#donate-info");
  const paymentMethod = document.querySelector("#payment-method");
  const additionalInfo = document.querySelector("#additional_info");
  const modalName = document.querySelector(".buy_item .name span");
  const modalPrice = document.querySelector("#modal_price span");
  const modalPromoInfo = document.querySelector(".check_info");
  const promoDiscount = document.querySelector(".check_info span");
  const promoInput = document.querySelector('input[name="promo"]');
  const nicknameInput = document.querySelector("#input_nick");
  const quantityWrap = document.querySelector("#quantity_wrap");
  const quantityInput = document.querySelector("#input_quantity");
  const quantityHint = document.querySelector("#quantity_hint");
  const modalDualRow = document.querySelector("#modal_dual_row");
  const submitButton = document.querySelector(".buy_item .action button");
  const formStatus = document.querySelector("#form_status");
  const lookupInput = document.querySelector("#order_lookup_input");
  const lookupButton = document.querySelector("#order_lookup_button");
  const lookupResult = document.querySelector("#order_lookup_result");
  const supportEmail = document.querySelector("#support_email");
  const contactsTitle = document.querySelector("#contacts_title");
  const discordText = document.querySelector("#contact_discord_text");
  const discordLinkLabel = document.querySelector("#discord_link_label");
  const vkText = document.querySelector("#contact_vk_text");
  const vkLinkLabel = document.querySelector("#vk_link_label");
  const supportText = document.querySelector("#contact_support_text");

  function qs(selector, root = document) {
    return root.querySelector(selector);
  }

  function qsa(selector, root = document) {
    return Array.from(root.querySelectorAll(selector));
  }

  function createTitles(projectName) {
    return {
      home: `Главная | ${projectName}`,
      rules: `Правила | ${projectName}`,
      contacts: `Контакты | ${projectName}`,
      admin: `Admin | ${projectName}`
    };
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

  async function parseJSONResponse(response, fallbackMessage) {
    const raw = await response.text();
    if (!raw) {
      return {};
    }

    try {
      return JSON.parse(raw);
    } catch (_error) {
      const error = new Error(fallbackMessage || "Backend вернул некорректный ответ.");
      error.status = response.status;
      throw error;
    }
  }

  function formatPrice(value) {
    return `${Number(value || 0).toLocaleString("ru-RU")} руб.`;
  }

  function formatQuantity(value, unitLabel) {
    const amount = Number(value || 0).toLocaleString("ru-RU");
    return unitLabel ? `${amount} ${unitLabel}` : amount;
  }

  function createPriceLabel(item, period) {
    const unitPrice = Number(period?.price || item?.price || 0);
    if (item?.variablePrice) {
      const minimum = Math.max(Number(item.minQuantity || 1), 1);
      return `от ${formatPrice(unitPrice * minimum)}`;
    }
    return formatPrice(unitPrice);
  }

  function createQuantityHint(item, period) {
    if (!item?.variablePrice) {
      return "";
    }

    const minimum = Math.max(Number(item.minQuantity || 1), 1);
    const maximum = Math.max(Number(item.maxQuantity || 0), minimum);
    const step = Math.max(Number(item.quantityStep || 1), 1);
    const label = item.unitLabel || "единиц";
    const unitPrice = Number(period?.price || item.price || 0);

    return `Минимум ${minimum} ${label}, максимум ${maximum}. Шаг ${step}. Цена за 1 единицу: ${formatPrice(unitPrice)}.`;
  }

  function parsePositiveInt(value, fallback) {
    const parsed = Number.parseInt(String(value ?? "").trim(), 10);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return fallback;
    }
    return parsed;
  }

  function setBodyPage(page) {
    document.body.dataset.page = page;
  }

  function getVisitorID() {
    const storageKey = "estelar-visitor-id";
    const generateID = () => {
      if (window.crypto?.randomUUID) {
        return window.crypto.randomUUID();
      }
      return `estelar-${Date.now()}-${Math.random().toString(16).slice(2, 12)}`;
    };

    try {
      const existing = window.localStorage?.getItem(storageKey);
      if (existing) {
        return existing;
      }

      const next = generateID();
      window.localStorage?.setItem(storageKey, next);
      return next;
    } catch (_error) {
      if (!window.__ESTELAR_VISITOR_ID__) {
        window.__ESTELAR_VISITOR_ID__ = generateID();
      }
      return window.__ESTELAR_VISITOR_ID__;
    }
  }

  function trackPageVisit(page) {
    if (!publicPages.includes(page)) {
      return;
    }

    const now = Date.now();
    if (state.lastTrackedPage === page && now - state.lastTrackedAt < 1200) {
      return;
    }

    state.lastTrackedPage = page;
    state.lastTrackedAt = now;

    const payload = JSON.stringify({
      page,
      path: pathForPage(page),
      visitorId: getVisitorID(),
      referrer: document.referrer || ""
    });

    if (navigator.sendBeacon) {
      const blob = new Blob([payload], { type: "application/json" });
      navigator.sendBeacon(`${apiBase}/analytics/visit`, blob);
      return;
    }

    fetch(`${apiBase}/analytics/visit`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json"
      },
      body: payload,
      keepalive: true
    }).catch(() => {});
  }

  function setMultilineText(element, value) {
    if (!element) {
      return;
    }

    const normalized = String(value || "").trim();
    element.innerHTML = escapeHtml(normalized).replace(/\n/g, "<br />");
  }

  function normalizePathname(pathname = window.location.pathname) {
    const normalized = String(pathname || "/").replace(/\/+$/, "");
    return normalized || "/";
  }

  function getLegacyHashPage(hash = window.location.hash) {
    const normalized = String(hash || "").replace(/^#/, "").trim();
    return pages.includes(normalized) ? normalized : "";
  }

  function pathForPage(page) {
    return pageRoutes[page] || pageRoutes.home;
  }

  function getPageFromLocation() {
    switch (normalizePathname()) {
      case "/rules":
        return "rules";
      case "/contacts":
        return "contacts";
      case "/admin":
        return "admin";
      default: {
        const legacyPage = getLegacyHashPage();
        return legacyPage || "home";
      }
    }
  }

  function syncURLForPage(page, replace = false) {
    const nextPath = pathForPage(page);
    const currentPath = normalizePathname();
    if (currentPath === nextPath && !window.location.hash) {
      return;
    }

    history[replace ? "replaceState" : "pushState"](null, "", nextPath);
  }

  function isSpecialPurchaseLayout(item) {
    return Boolean(item?.variablePrice || item?.category === "case");
  }

  function getQuantityPlaceholder(item) {
    if (!item) {
      return "Количество";
    }
    if (item.slug === "donate-currency") {
      return "Кол-во пакетов";
    }
    if (item.category === "case") {
      return "Кол-во кейсов";
    }
    if (item.unitLabel) {
      return `Кол-во ${item.unitLabel}`;
    }
    return "Количество";
  }

  function openAnimatedModal(target) {
    if (!target) {
      return;
    }

    window.clearTimeout(state.modalCloseTimer);
    target.hidden = false;
    target.classList.remove("is-closing");
    requestAnimationFrame(() => {
      target.classList.add("is-open");
    });
    document.body.classList.add("modal-open");
  }

  function closeAnimatedModal(target, onClosed) {
    if (!target) {
      return;
    }

    target.classList.remove("is-open");
    target.classList.add("is-closing");
    document.body.classList.remove("modal-open");
    window.clearTimeout(state.modalCloseTimer);
    state.modalCloseTimer = window.setTimeout(() => {
      target.hidden = true;
      target.classList.remove("is-closing");
      onClosed?.();
    }, 220);
  }

  function showPage(page, options = {}) {
    const replace = Boolean(options.replace);
    const skipURL = Boolean(options.skipURL);
    const normalizedPage = pages.includes(page) ? page : "home";
    qsa(".page-section").forEach((section) => {
      section.classList.toggle("active-page", section.dataset.page === normalizedPage);
    });
    qsa("[data-page-link]").forEach((link) => {
      const linkedPage = link.dataset.pageLink || "";
      link.classList.toggle("active", linkedPage === normalizedPage);
    });
    document.title = titles[normalizedPage] || titles.home;
    if (!skipURL) {
      syncURLForPage(normalizedPage, replace);
    }
    setBodyPage(normalizedPage);
    closeMobile();
    trackPageVisit(normalizedPage);
    window.dispatchEvent(new CustomEvent("estelar:pagechange", {
      detail: { page: normalizedPage }
    }));
  }

  function openMobile() {
    qs(".mobile_menu")?.classList.remove("sf-hidden");
  }

  function closeMobile() {
    qs(".mobile_menu")?.classList.add("sf-hidden");
  }

  function setLinkState(element, url) {
    if (!element) {
      return;
    }

    const safeURL = url && url !== "#" ? url : "#";
    element.setAttribute("href", safeURL);

    if (safeURL === "#") {
      element.classList.add("disabled_link");
      element.setAttribute("aria-disabled", "true");
      element.removeAttribute("target");
      return;
    }

    element.classList.remove("disabled_link");
    element.removeAttribute("aria-disabled");
    element.setAttribute("target", "_blank");
  }

  function setSupportEmail(email) {
    if (!supportEmail) {
      return;
    }

    const value = String(email || "").trim();
    if (!value) {
      supportEmail.textContent = "support@estelar.shop";
      supportEmail.dataset.copy = "support@estelar.shop";
      supportEmail.classList.remove("disabled_copy");
      return;
    }

    supportEmail.textContent = value;
    supportEmail.dataset.copy = value;
    supportEmail.classList.remove("disabled_copy");
  }

  function setServerSelects(name) {
    qsa(".server_select option").forEach((option) => {
      option.textContent = name;
    });
  }

  function renderFooterLegal(serverName, projectName) {
    const copyright = qs(".footer .copyright");
    const developer = qs(".footer .developer");
    if (copyright) {
      copyright.innerHTML = `
        Сервер <span id="footer_server_name">${escapeHtml(serverName)}</span> никак не относится к Mojang, AB.
        <br/>
        Реквизиты будут добавлены перед запуском проекта
        <br/>
        ИНН 000000000000, ОГРНИП/ОГРН 000000000000000
        <br/>
        Копирование контента с сайта, серверов проекта запрещено.
        <br/>
        <span class="policy_links">
          <a href="/oferta" id="offer_link_footer">Договор оферты</a>
          |
          <a href="/privacy" id="privacy_link_footer">Политика обработки персональных данных</a>
        </span>
      `;
    }

    if (developer) {
      developer.innerHTML = `
        © 2026 <span id="footer_brand_name">${escapeHtml(projectName)}</span>
        <br/>
        Все права защищены
      `;
    }
  }

  function applyMeta(meta) {
    state.meta = meta;

    const projectName = meta.projectName || defaultProjectName;
    const serverName = meta.serverName || projectName;
    Object.assign(titles, createTitles(projectName));
    document.title = titles[getPageFromLocation()] || titles.home;

    setServerSelects(serverName);
    renderFooterLegal(serverName, projectName);

    setLinkState(qs("#offer_link_modal"), meta.offerUrl);
    setLinkState(qs("#offer_link_footer"), meta.offerUrl);
    setLinkState(qs("#privacy_link_footer"), meta.privacyUrl);
    applyContactSettings(meta);

  }

  function setMetaFallback() {
    renderFooterLegal(defaultProjectName, defaultProjectName);
    setLinkState(qs("#offer_link_modal"), "/oferta");
    setLinkState(qs("#offer_link_footer"), "/oferta");
    setLinkState(qs("#privacy_link_footer"), "/privacy");
    applyContactSettings({
      supportEmail: "support@estelar.shop",
      discordUrl: "https://discord.gg/estelar",
      vkUrl: "https://t.me/estelar",
      contactTitle: "Остались вопросы? Напиши нам в соц.сетях:",
      discordText: "А ты уже есть в нашем Discord-сервере?\nСкорее подключайся!",
      discordButtonLabel: "Подключиться",
      vkText: "Наш Telegram-канал: новости, анонсы и\nбыстрые обновления по серверу",
      vkButtonLabel: "Перейти",
      supportText: "Имеются другие вопросы?\nПиши на нашу почту:"
    });
  }

  function applyContactSettings(settings) {
    setLinkState(qs("#discord_link"), settings.discordUrl);
    setLinkState(qs("#vk_link"), settings.vkUrl);
    setSupportEmail(settings.supportEmail);
    setMultilineText(contactsTitle, settings.contactTitle || "Остались вопросы? Напиши нам в соц.сетях:");
    setMultilineText(discordText, settings.discordText || "А ты уже есть в нашем Discord-сервере?\nСкорее подключайся!");
    setMultilineText(vkText, settings.vkText || "Наш Telegram-канал: новости, анонсы и\nбыстрые обновления по серверу");
    setMultilineText(supportText, settings.supportText || "Имеются другие вопросы?\nПиши на нашу почту:");
    if (discordLinkLabel) {
      discordLinkLabel.textContent = settings.discordButtonLabel || "Подключиться";
    }
    if (vkLinkLabel) {
      vkLinkLabel.textContent = settings.vkButtonLabel || "Перейти";
    }
  }

  async function loadMeta() {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);

    try {
      const response = await fetch(`${apiBase}/meta`, {
        signal: controller.signal,
        headers: {
          Accept: "application/json"
        }
      });
      const payload = await parseJSONResponse(response, "Метаданные проекта пришли в неверном формате.");
      if (!response.ok) {
        throw new Error(payload.error || "Не удалось загрузить метаданные проекта.");
      }

      applyMeta(payload);
    } catch (error) {
      if (error.name === 'AbortError') {
        // Fallback below is enough for the storefront.
      }
      setMetaFallback();
    } finally {
      clearTimeout(timeout);
    }
  }

  function getSelectedItem() {
    return state.catalog.find((item) => item.slug === state.selectedSlug) || null;
  }

  function getPeriod(item, periodCode) {
    if (!item) {
      return null;
    }
    return item.periods.find((period) => period.code === periodCode) || item.periods[0] || null;
  }

  function getDefaultPeriod(item) {
    if (!item) {
      return null;
    }
    return item.periods.find((period) => period.default) || item.periods[0] || null;
  }

  function getModalQuantity(item) {
    if (!item?.variablePrice) {
      return 1;
    }

    const minQuantity = Math.max(Number(item.minQuantity || 1), 1);
    return parsePositiveInt(quantityInput?.value, minQuantity);
  }

  function renderCatalogError(message) {
    const itemInfo = qs("#item_info");
    const itemsList = qs("#items_list");
    if (itemsList) {
      itemsList.innerHTML = `
        <div class="catalog_notice">
          <p>${escapeHtml(message)}</p>
        </div>
      `;
    }
    if (itemInfo) {
      itemInfo.innerHTML = `
        <div class="catalog_notice">
          <p>${escapeHtml(message)}</p>
          <p>Попробуйте обновить страницу чуть позже.</p>
        </div>
      `;
    }
  }

  function renderCatalogLoading() {
    const itemInfo = qs("#item_info");
    const itemsList = qs("#items_list");
    if (itemsList) {
      itemsList.innerHTML = `
        <div class="catalog_notice">
          <p>Загрузка каталога...</p>
        </div>
      `;
    }
    if (itemInfo) {
      itemInfo.innerHTML = `
        <div class="catalog_notice">
          <p>Загружаем витрину товаров...</p>
        </div>
      `;
    }
  }

  function renderItems() {
    const itemsList = qs("#items_list");
    if (!itemsList) {
      return;
    }

    if (!state.catalog.length) {
      itemsList.innerHTML = `
        <div class="catalog_notice">
          <p>Каталог пока пуст.</p>
        </div>
      `;
      return;
    }

    itemsList.innerHTML = state.catalog.map((item) => {
      const period = getDefaultPeriod(item);
      const isActive = item.slug === state.selectedSlug;
      return `
        <button class="item${isActive ? " active" : ""}" data-item-slug="${escapeHtml(item.slug)}" type="button">
          <div class="item_copy">
            <p class="name">${escapeHtml(item.name)}</p>
          </div>
          <p class="price">${escapeHtml(createPriceLabel(item, period))}</p>
        </button>
      `;
    }).join("");

    qsa(".item", itemsList).forEach((itemNode) => {
      itemNode.addEventListener("click", () => {
        state.selectedSlug = itemNode.dataset.itemSlug || "";
        renderItems();
        renderSelectedItem();
      });
    });
  }

  function renderSelectedItem() {
    const itemInfo = qs("#item_info");
    const item = getSelectedItem();
    if (!itemInfo || !item) {
      return;
    }

    const activePeriod = getDefaultPeriod(item);
    const summary = escapeHtml(item.summary || "").replace(/\n/g, "<br>");
    const showPeriodSelector = item.variablePrice || item.periods.length > 1;
    const buyButtonLabel = item.category === "privilege" ? "Купить привилегию" : "Перейти к покупке";
    const highlights = Array.isArray(item.highlights)
      ? item.highlights
        .map((text) => escapeHtml(text).replace(/\n/g, "<br>"))
        .filter(Boolean)
        .map((text) => `<li>${text}</li>`)
        .join("")
      : "";
    const periods = item.periods.map((period) => `
      <p class="${period.code === activePeriod?.code ? "active" : ""}" data-id="${escapeHtml(period.code)}" data-price="${period.price}">
        ${escapeHtml(period.label)}
      </p>
    `).join("");

    const metaChips = item.variablePrice
      ? [
        item.categoryLabel,
        activePeriod?.label || "1 пакет = 100 EST",
        `Минимум: ${formatQuantity(item.minQuantity || 1, item.unitLabel || "единиц")}`
      ]
      : [
        item.categoryLabel,
        activePeriod?.label || "Навсегда",
        "Цифровой товар"
      ];

    const metaStrip = metaChips.map((value) => `
      <span class="item_meta_chip">${escapeHtml(value)}</span>
    `).join("");

    itemInfo.innerHTML = `
      <div class="item_head">
        <div class="item_head_copy">
          <p class="item_eyebrow">${escapeHtml(item.categoryLabel)}</p>
          <h2 class="item_name">${escapeHtml(item.name)}</h2>
          <p class="item_summary">${summary}</p>
        </div>
        <div class="item_price_badge">
          <span>${item.variablePrice ? "Стоимость от" : "Стоимость"}</span>
          <strong id="item_price_primary">${escapeHtml(createPriceLabel(item, activePeriod))}</strong>
        </div>
      </div>
      ${showPeriodSelector ? `
        <div class="method">
          <p class="title">${item.variablePrice ? "Формат покупки:" : "Выберите срок:"}</p>
          <div class="select" id="method_select">
            ${periods}
          </div>
        </div>
      ` : ""}
      <div class="info_content">
        <div class="item_meta_strip">
          ${metaStrip}
        </div>
        <div class="item_features">
          <p class="item_features_title">Что входит:</p>
          <ul>
            ${highlights || "<li>Товар будет выдан после подтверждения оплаты.</li>"}
          </ul>
        </div>
      </div>
      <div class="item_action" id="info_price">
        <p class="price">${item.variablePrice ? "Стоимость от:" : "Стоимость:"} <span>${escapeHtml(createPriceLabel(item, activePeriod))}</span></p>
        <button id="buy_button" data-product-slug="${escapeHtml(item.slug)}" data-period-code="${escapeHtml(activePeriod?.code || "forever")}" type="button">
          ${escapeHtml(buyButtonLabel)}
        </button>
      </div>
    `;

    qsa("#method_select p", itemInfo).forEach((option) => {
      option.addEventListener("click", () => {
        qsa("#method_select p", itemInfo).forEach((node) => node.classList.remove("active"));
        option.classList.add("active");
        const selectedPeriod = getPeriod(item, option.dataset.id);
        const priceSpan = qs("#info_price .price span", itemInfo);
        const badgePrice = qs("#item_price_primary", itemInfo);
        const buyButton = qs("#buy_button", itemInfo);
        if (priceSpan) {
          priceSpan.textContent = createPriceLabel(item, selectedPeriod);
        }
        if (badgePrice) {
          badgePrice.textContent = createPriceLabel(item, selectedPeriod);
        }
        if (buyButton) {
          buyButton.dataset.periodCode = selectedPeriod?.code || "forever";
        }
      });
    });

    qs("#buy_button", itemInfo)?.addEventListener("click", () => {
      const selectedButton = qs("#buy_button", itemInfo);
      openModal(item, selectedButton?.dataset.periodCode || activePeriod?.code || "forever");
    });
  }

  async function loadCatalog() {
    renderCatalogLoading();
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);

    try {
      const response = await fetch(`${apiBase}/catalog`, {
        signal: controller.signal,
        headers: {
          Accept: "application/json"
        }
      });
      const payload = await parseJSONResponse(response, "Каталог пришел в неверном формате.");
      if (!response.ok) {
        throw new Error(payload.error || "Не удалось загрузить каталог.");
      }

      state.catalog = Array.isArray(payload.items) ? payload.items : [];
      state.selectedSlug = state.catalog[0]?.slug || "";
      renderItems();
      renderSelectedItem();
    } catch (error) {
      if (error.name === 'AbortError') {
        renderCatalogError("Каталог недоступен (истёк таймаут подключения).");
      } else {
        renderCatalogError(error.message || "Каталог временно недоступен.");
      }
    } finally {
      clearTimeout(timeout);
    }
  }

  function setFormStatus(message, type = "info") {
    if (!formStatus) {
      return;
    }

    if (!message) {
      formStatus.textContent = "";
      formStatus.className = "form_status";
      formStatus.hidden = true;
      return;
    }

    formStatus.textContent = message;
    formStatus.className = `form_status ${type}`;
    formStatus.hidden = false;
  }

  function showPromoInfo(discountAmount) {
    if (!modalPromoInfo || !promoDiscount) {
      return;
    }

    if (discountAmount > 0) {
      promoDiscount.textContent = `-${formatPrice(discountAmount)}`;
      modalPromoInfo.hidden = false;
      return;
    }

    modalPromoInfo.hidden = true;
  }

  function updateModalPrice(value) {
    if (modalPrice) {
      modalPrice.textContent = formatPrice(value);
    }
  }

  function resetModalPanels() {
    if (donateInfo) {
      donateInfo.hidden = false;
    }
    if (paymentMethod) {
      paymentMethod.hidden = true;
      paymentMethod.innerHTML = "";
    }
    if (additionalInfo) {
      additionalInfo.hidden = true;
      additionalInfo.innerHTML = "";
    }
  }

  function getPurchaseValidationError() {
    if (!state.modalItem) {
      return "Ошибка\nЗаполните все поля!";
    }

    const nickname = nicknameInput?.value.trim() || "";
    const quantityValue = String(quantityInput?.value ?? "").trim();
    const requiresQuantity = !quantityWrap?.classList.contains("sf-hidden");

    if (!nickname || (requiresQuantity && !quantityValue)) {
      return "Ошибка\nЗаполните все поля!";
    }

    return "";
  }

  function openModal(item, periodCode) {
    const resolvedItem = item || getSelectedItem();
    const selectedPeriod = getPeriod(resolvedItem, periodCode) || getDefaultPeriod(resolvedItem);
    if (!modal || !resolvedItem || !selectedPeriod) {
      return;
    }

    state.modalItem = resolvedItem;
    state.lastQuote = null;
    resetModalPanels();
    buyForm?.reset();

    if (modalName) {
      modalName.textContent = `"${resolvedItem.name}"`;
    }
    if (nicknameInput) {
      nicknameInput.value = "";
    }
    if (promoInput) {
      promoInput.value = "";
    }

    modal.dataset.productSlug = resolvedItem.slug;
    modal.dataset.periodCode = selectedPeriod.code;
    modal.classList.toggle("is-special-layout", isSpecialPurchaseLayout(resolvedItem));
    buyForm?.classList.toggle("is-special-layout", isSpecialPurchaseLayout(resolvedItem));
    if (modalDualRow) {
      modalDualRow.classList.toggle("is-split-layout", isSpecialPurchaseLayout(resolvedItem));
    }

    if (resolvedItem.variablePrice && quantityWrap && quantityInput && quantityHint) {
      const minQuantity = Math.max(Number(resolvedItem.minQuantity || 1), 1);
      const maxQuantity = Math.max(Number(resolvedItem.maxQuantity || minQuantity), minQuantity);
      const step = Math.max(Number(resolvedItem.quantityStep || 1), 1);
      quantityWrap.classList.remove("sf-hidden");
      quantityInput.min = String(minQuantity);
      quantityInput.max = String(maxQuantity);
      quantityInput.step = String(step);
      quantityInput.value = String(minQuantity);
      quantityInput.placeholder = getQuantityPlaceholder(resolvedItem);
      quantityHint.textContent = createQuantityHint(resolvedItem, selectedPeriod);
    } else if (quantityWrap && quantityInput && quantityHint) {
      quantityWrap.classList.add("sf-hidden");
      quantityInput.value = "1";
      quantityInput.min = "1";
      quantityInput.step = "1";
      quantityInput.placeholder = "Количество";
      quantityInput.removeAttribute("max");
      quantityHint.textContent = "";
    }

    showPromoInfo(0);
    setFormStatus("");

    const initialPrice = resolvedItem.variablePrice
      ? Number(selectedPeriod.price || resolvedItem.price || 0) * getModalQuantity(resolvedItem)
      : Number(selectedPeriod.price || resolvedItem.price || 0);
    updateModalPrice(initialPrice);

    if (submitButton) {
      submitButton.disabled = false;
      submitButton.textContent = "Перейти к оплате";
    }

    openAnimatedModal(modal);
  }

  function closeModal() {
    if (!modal) {
      return;
    }

    closeAnimatedModal(modal, () => {
      state.modalItem = null;
      state.lastQuote = null;
      window.clearTimeout(state.quoteTimer);
      setFormStatus("");
      showPromoInfo(0);
      resetModalPanels();
    });
  }

  async function requestQuote() {
    if (!state.modalItem) {
      return null;
    }

    const payload = {
      productSlug: modal.dataset.productSlug || state.modalItem.slug,
      periodCode: modal.dataset.periodCode || "forever",
      promoCode: promoInput?.value.trim() || "",
      quantity: getModalQuantity(state.modalItem)
    };

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);

    try {
      const response = await fetch(`${apiBase}/orders/quote`, {
        signal: controller.signal,
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json"
        },
        body: JSON.stringify(payload)
      });

      const data = await parseJSONResponse(response, "Расчет стоимости вернул неверный ответ.");
      if (!response.ok) {
        throw new Error(data.error || "Не удалось рассчитать итоговую стоимость.");
      }

      clearTimeout(timeout);
      state.lastQuote = data;
      updateModalPrice(data.finalPrice);
      showPromoInfo(data.discountAmount);

      if (payload.promoCode) {
        setFormStatus(data.promo?.message || "Промокод проверен.", data.promo?.applied ? "success" : "warning");
      } else {
        setFormStatus("");
      }

      return data;
    } catch (error) {
      if (error.name === 'AbortError') {
        throw new Error("Не удалось проверить промокод (истёк таймаут).");
      }
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }

  function scheduleQuoteRefresh() {
    if (!state.modalItem) {
      return;
    }

    window.clearTimeout(state.quoteTimer);
    state.quoteTimer = window.setTimeout(async () => {
      try {
        await requestQuote();
      } catch (error) {
        setFormStatus(error.message || "Не удалось проверить промокод.", "error");
      }
    }, 250);
  }

  function showPaymentStage(order) {
    if (!paymentMethod || !donateInfo) {
      return;
    }

    const quantityLine = order.variablePrice || Number(order.quantity || 1) > 1
      ? `<p>Количество <span>${escapeHtml(formatQuantity(order.quantity, order.unitLabel || "единиц"))}</span></p>`
      : "";

    donateInfo.hidden = true;
    paymentMethod.hidden = false;
    if (additionalInfo) {
      additionalInfo.hidden = true;
      additionalInfo.innerHTML = "";
    }

    paymentMethod.innerHTML = `
      <div class="payment_stage">
        <p class="title">Оплата заказа</p>
        <p class="desc">Заказ уже создан. Следующий шаг будет вести на онлайн-оплату сразу после подключения платёжного провайдера.</p>

        <div class="payment_stage_summary">
          <p>Номер заказа <span>${escapeHtml(order.id)}</span></p>
          <p>Товар <span>${escapeHtml(order.productName)}</span></p>
          <p>Ник <span>${escapeHtml(order.nickname)}</span></p>
          ${quantityLine}
          <p>К оплате <span>${escapeHtml(formatPrice(order.finalPrice))}</span></p>
        </div>

        <ul class="methods payment_methods_list">
          ${paymentProviders.map((provider) => `
            <li>
              <button class="payment-btn${provider.enabled ? "" : " is-disabled"}" ${provider.enabled ? "" : "disabled"} data-payment-provider="${escapeHtml(provider.code)}" type="button">
                ${escapeHtml(provider.label)}
              </button>
              <p class="payment_provider_note">${escapeHtml(provider.note)}</p>
            </li>
          `).join("")}
        </ul>

        <div class="payment_notice">
          Здесь уже подготовлено место под оплату картой, СБП и резервные методы без переделки корзины.
        </div>

        <div class="payment_stage_actions">
          <button class="secondary" data-copy-order type="button">Скопировать номер</button>
          <button data-close-payment type="button">Закрыть</button>
        </div>
      </div>
    `;

    qs("[data-close-payment]", paymentMethod)?.addEventListener("click", closeModal);
    qs("[data-copy-order]", paymentMethod)?.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(order.id);
        setFormStatus("Номер заказа скопирован.", "success");
      } catch (_error) {
        setFormStatus(`Номер заказа: ${order.id}`, "info");
      }
    });
  }

  async function submitOrder(event) {
    event.preventDefault();

    const validationError = getPurchaseValidationError();
    if (validationError) {
      setFormStatus(validationError, "error");
      return;
    }

    const nickname = nicknameInput?.value.trim() || "";
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);

    try {
      if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = "Оформляем...";
      }

      await requestQuote();

      const response = await fetch(`${apiBase}/orders`, {
        signal: controller.signal,
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json"
        },
        body: JSON.stringify({
          productSlug: modal.dataset.productSlug || state.modalItem.slug,
          periodCode: modal.dataset.periodCode || "forever",
          nickname,
          promoCode: promoInput?.value.trim() || "",
          quantity: getModalQuantity(state.modalItem)
        })
      });

      const data = await parseJSONResponse(response, "Создание заказа вернуло неверный ответ.");
      if (!response.ok) {
        throw new Error(data.error || "Не удалось создать заказ.");
      }

      setFormStatus("");
      showPromoInfo(data.order.discountAmount);
      updateModalPrice(data.order.finalPrice);
      showPaymentStage(data.order);
    } catch (error) {
      if (error.name === 'AbortError') {
        setFormStatus("Запрос истёк по таймауту. Попробуйте позже.", "error");
      } else {
        setFormStatus(error.message || "Не удалось оформить заказ.", "error");
      }
    } finally {
      if (submitButton) {
        submitButton.disabled = false;
        submitButton.textContent = "Перейти к оплате";
      }
      clearTimeout(timeout);
    }
  }

  function setLookupResult(message, type = "info") {
    if (!lookupResult) {
      return;
    }

    lookupResult.textContent = message;
    lookupResult.className = `lookup_result ${type}`;
  }

  async function lookupOrder() {
    const orderID = lookupInput?.value.trim() || "";
    if (!orderID) {
      setLookupResult("Введите номер заказа.", "error");
      return;
    }

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);

    try {
      if (lookupButton) {
        lookupButton.disabled = true;
        lookupButton.textContent = "Проверяем...";
      }

      const response = await fetch(`${apiBase}/orders/${encodeURIComponent(orderID)}`, {
        signal: controller.signal,
        headers: {
          Accept: "application/json"
        }
      });
      const data = await parseJSONResponse(response, "Проверка заказа вернула неверный ответ.");
      if (!response.ok) {
        throw new Error(data.error || "Не удалось получить заказ.");
      }

      const order = data.order;
      const quantitySuffix = order.variablePrice || Number(order.quantity || 1) > 1
        ? ` · ${formatQuantity(order.quantity, order.unitLabel || "единиц")}`
        : "";
      setLookupResult(
        `${order.productName} · ${formatPrice(order.finalPrice)}${quantitySuffix} · ${order.statusLabel}`,
        "success"
      );
    } catch (error) {
      if (error.name === 'AbortError') {
        setLookupResult("Запрос истёк (API не отвечает).", "error");
      } else {
        setLookupResult(error.message || "Не удалось получить заказ.", "error");
      }
    } finally {
      if (lookupButton) {
        lookupButton.disabled = false;
        lookupButton.textContent = "Проверить заказ";
      }
      clearTimeout(timeout);
    }
  }

  qsa("[data-page-link]").forEach((link) => {
    link.addEventListener("click", (event) => {
      const page = link.dataset.pageLink;
      if (!page) {
        return;
      }
      event.preventDefault();
      showPage(page);
    });
  });

  qs(".open_menu")?.addEventListener("click", () => {
    const menu = qs(".mobile_menu");
    if (!menu) {
      return;
    }
    menu.classList.contains("sf-hidden") ? openMobile() : closeMobile();
  });

  document.addEventListener("click", (event) => {
    if (!event.target.closest(".mobile_menu") && !event.target.closest(".open_menu")) {
      closeMobile();
    }
  });

  qsa("[data-open-modal]").forEach((button) => {
    button.addEventListener("click", () => {
      const slug = button.dataset.itemSlug || "";
      const item = state.catalog.find((catalogItem) => catalogItem.slug === slug);
      if (item) {
        openModal(item, getDefaultPeriod(item)?.code || "forever");
      } else {
        showPage("home");
      }
    });
  });

  qsa("[data-open-home]").forEach((button) => {
    button.addEventListener("click", () => showPage("home"));
  });

  function setRuleExpanded(box, expanded) {
    if (!box) {
      return;
    }

    const content = qs(".rule_content", box);
    const head = qs(".head", box);
    const statuses = qsa(".box_status", head);

    box.classList.toggle("is-open", expanded);
    if (content) {
      content.classList.remove("sf-hidden");
      content.style.maxHeight = expanded ? `${content.scrollHeight}px` : "0px";
      content.style.opacity = expanded ? "1" : "0";
    }
    if (statuses[0]) {
      statuses[0].classList.toggle("sf-hidden", expanded);
      statuses[0].classList.toggle("active", !expanded);
    }
    if (statuses[1]) {
      statuses[1].classList.toggle("sf-hidden", !expanded);
      statuses[1].classList.toggle("active", expanded);
    }
  }

  qsa(".rule_box").forEach((box) => {
    setRuleExpanded(box, false);
    qs(".head", box)?.addEventListener("click", () => {
      setRuleExpanded(box, !box.classList.contains("is-open"));
    });
  });

  supportEmail?.addEventListener("click", async () => {
    const text = supportEmail.dataset.copy || "";
    if (!text) {
      return;
    }

    try {
      await navigator.clipboard.writeText(text);
    } catch (_error) {
      return;
    }

    supportEmail.classList.add("ok");
    setTimeout(() => supportEmail.classList.remove("ok"), 1400);
  });

  lookupButton?.addEventListener("click", lookupOrder);
  lookupInput?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      lookupOrder();
    }
  });

  promoInput?.addEventListener("input", scheduleQuoteRefresh);
  quantityInput?.addEventListener("input", scheduleQuoteRefresh);
  buyForm?.addEventListener("submit", submitOrder);
  qs(".close_bModal", modal)?.addEventListener("click", closeModal);
  modal?.addEventListener("click", (event) => {
    if (event.target === modal) {
      closeModal();
    }
  });
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && modal?.classList.contains("is-open")) {
      closeModal();
    }
  });

  window.addEventListener("popstate", () => {
    showPage(getPageFromLocation(), { skipURL: true });
  });

  window.addEventListener("hashchange", () => {
    const legacyPage = getLegacyHashPage();
    if (legacyPage) {
      showPage(legacyPage, { replace: true });
    }
  });

  window.addEventListener("estelar:settings-updated", (event) => {
    const settings = event.detail?.settings;
    if (!settings) {
      return;
    }

    state.meta = {
      ...(state.meta || {}),
      ...settings
    };
    applyContactSettings(state.meta);
  });

  window.__ESTELAR_ROUTER__ = {
    publicPages: [...publicPages],
    getPageFromLocation,
    pathForPage,
    showPage
  };

  showPage(getPageFromLocation(), { replace: true });
  Promise.allSettled([loadMeta(), loadCatalog()]);
})();
