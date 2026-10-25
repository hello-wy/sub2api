(function () {
  "use strict";

  const defaults = {
    siteName: "SolidAPI",
    pageTitle: "Codex 接入教程",
    providerName: "SolidAPI",
    apiBaseUrl: window.location.origin,
    keysUrl: "/keys",
    homeUrl: "/home",
    docsUrl: "/docs",
    usePublicSettings: true,
    supportText: "联系平台客服",
    supportUrl: "",
    codexDownloadUrl: "https://chatgpt.com/zh-Hans-CN/codex/",
    ccSwitchDownloadUrl: "https://ccswitch.io/zh/download",
  };

  const config = Object.assign({}, defaults, window.TUTORIAL_CONFIG || {});

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

  function setText(selector, value) {
    $$(selector).forEach((element) => {
      element.textContent = value;
    });
  }

  function isHttpUrl(value) {
    try {
      const url = new URL(value);
      return url.protocol === "https:" || (url.protocol === "http:" && ["localhost", "127.0.0.1"].includes(url.hostname));
    } catch (_error) {
      return false;
    }
  }

  function normalizedBaseUrl(value) {
    return value.trim().replace(/\/+$/, "");
  }

  function baseUrls() {
    const base = normalizedBaseUrl(isHttpUrl(config.apiBaseUrl) ? config.apiBaseUrl : window.location.origin);
    const root = base.replace(/\/v1$/i, "");
    return { codex: root + "/v1", claude: root };
  }

  function shellQuote(value) {
    return "'" + value.replace(/'/g, "'\\''") + "'";
  }

  function escapeTomlString(value) {
    return value.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  }

  function applyConfig() {
    setText("[data-site-name]", config.siteName);
    const pageTitle = document.body.dataset.activeClient === "claude" ? "Claude Code 接入教程" : config.pageTitle;
    setText("[data-page-title]", pageTitle);
    setText("[data-support-text]", config.supportText);
    document.title = `${pageTitle} - ${config.siteName}`;

    $$(".download-link-codex").forEach((link) => {
      link.href = config.codexDownloadUrl;
    });
    $$(".download-link-ccswitch").forEach((link) => {
      link.href = config.ccSwitchDownloadUrl;
    });

    const apiBaseUrlInput = $("#api-base-url");
    if (!apiBaseUrlInput.value || apiBaseUrlInput.value === apiBaseUrlInput.dataset.defaultValue) {
      apiBaseUrlInput.value = baseUrls().codex;
      apiBaseUrlInput.dataset.defaultValue = apiBaseUrlInput.value;
    }

    $$('.platform-links a, .page-footer a').forEach((link) => {
      if (link.getAttribute("href") === "/home") link.href = config.homeUrl;
      if (link.getAttribute("href") === "/docs") link.href = config.docsUrl;
    });
    $(".nav-context-link").href = config.keysUrl;

    const supportLink = $("#support-link");
    if (config.supportUrl) {
      supportLink.href = config.supportUrl;
      if (/^https?:\/\//.test(config.supportUrl)) {
        supportLink.target = "_blank";
        supportLink.rel = "noopener noreferrer";
      }
    }

    const keyPageLink = $(".key-page-link");
    const keysPageButton = $("#keys-page-button");
    if (config.keysUrl) {
      keyPageLink.href = config.keysUrl;
      keysPageButton.href = config.keysUrl;
      keyPageLink.hidden = false;
      if (/^https?:\/\//.test(config.keysUrl)) {
        keyPageLink.target = "_blank";
        keyPageLink.rel = "noopener noreferrer";
        keysPageButton.target = "_blank";
        keysPageButton.rel = "noopener noreferrer";
      }
    }

    updateManualConfig();
  }

  function updateManualConfig() {
    const manualConfig = $("#manual-config code");
    const urls = baseUrls();
    const baseUrl = urls.codex;
    const providerName = config.providerName || config.siteName;
    manualConfig.textContent = `model_provider = "custom"

[model_providers.custom]
name = "${escapeTomlString(providerName)}"
base_url = "${escapeTomlString(baseUrl)}"
env_key = "SOLIDAPI_API_KEY"
wire_api = "responses"`;

    $("#claude-windows-env code").textContent =
      "[Environment]::SetEnvironmentVariable('ANTHROPIC_BASE_URL', '" + urls.claude.replace(/'/g, "''") + "', 'User')\n" +
      "[Environment]::SetEnvironmentVariable('ANTHROPIC_AUTH_TOKEN', '你的 API 密钥', 'User')";
    $("#claude-macos-env code").textContent =
      "export ANTHROPIC_BASE_URL=" + shellQuote(urls.claude) + "\n" +
      "export ANTHROPIC_AUTH_TOKEN='你的 API 密钥'";
  }

  async function loadPublicSettings() {
    if (!config.usePublicSettings) return;
    try {
      const response = await fetch("/api/v1/settings/public", { credentials: "omit", signal: AbortSignal.timeout(4000) });
      if (!response.ok) return;
      const payload = await response.json();
      if (payload.code !== 0 || !payload.data?.api_base_url || !isHttpUrl(payload.data.api_base_url)) return;
      config.apiBaseUrl = payload.data.api_base_url;
      applyConfig();
    } catch (_error) {
      // Standalone deployments can supply their endpoint through site.config.js.
    }
  }

  function setupOperatingSystemTabs() {
    const tabs = $$('[role="tab"][data-os]');

    function activate(os, focusTab) {
      tabs.forEach((tab) => {
        const isActive = tab.dataset.os === os;
        tab.setAttribute("aria-selected", String(isActive));
        tab.tabIndex = isActive ? 0 : -1;
        const panel = $(`#panel-${tab.dataset.os}`);
        panel.hidden = !isActive;
        if (isActive && focusTab) tab.focus();
      });
    }

    tabs.forEach((tab, index) => {
      tab.addEventListener("click", () => activate(tab.dataset.os, false));
      tab.addEventListener("keydown", (event) => {
        if (!["ArrowLeft", "ArrowRight"].includes(event.key)) return;
        event.preventDefault();
        const delta = event.key === "ArrowRight" ? 1 : -1;
        const nextIndex = (index + delta + tabs.length) % tabs.length;
        activate(tabs[nextIndex].dataset.os, true);
      });
    });

    const platform = `${navigator.userAgentData?.platform || ""} ${navigator.platform || ""} ${navigator.userAgent || ""}`.toLowerCase();
    activate(platform.includes("mac") ? "macos" : "windows", false);
  }

  function setupClientTabs() {
    const tabs = $$('[role="tab"][data-client-tab]');
    const sections = $$('[data-client-section]');
    const navGroups = $$('[data-client-nav]');
    const faqItems = $$(".faq-list details");
    const title = $("[data-page-title]");
    const eyebrow = $("#tutorial-eyebrow");
    const lede = $("#tutorial-lede");
    const footerTitle = $("#footer-tutorial-title");

    const copy = {
      codex: {
        title: config.pageTitle,
        eyebrow: "快速开始",
        lede: "跟随下面的步骤，在 Windows 或 macOS 上完成安装、导入和连接测试。",
      },
      claude: {
        title: "Claude Code 接入教程",
        eyebrow: "命令行接入",
        lede: "跟随下面的步骤，在 Windows 或 macOS 上完成 Claude Code 安装、配置和连接测试。",
      },
    };

    function activate(client, options = {}) {
      const previousClient = document.body.dataset.activeClient;

      tabs.forEach((tab) => {
        const isActive = tab.dataset.clientTab === client;
        tab.setAttribute("aria-selected", String(isActive));
        tab.tabIndex = isActive ? 0 : -1;
        if (isActive && options.focusTab) tab.focus();
      });

      sections.forEach((section) => {
        section.hidden = section.dataset.clientSection !== client;
      });
      navGroups.forEach((group) => {
        group.hidden = group.dataset.clientNav !== client;
      });
      faqItems.forEach((item) => {
        const owner = item.dataset.clientFaq || "codex";
        item.hidden = owner !== client;
      });

      title.textContent = copy[client].title;
      eyebrow.textContent = copy[client].eyebrow;
      lede.textContent = copy[client].lede;
      footerTitle.textContent = copy[client].title;
      document.title = `${copy[client].title} - ${config.siteName}`;
      document.body.dataset.activeClient = client;
      document.dispatchEvent(new CustomEvent("clientchange", {
        detail: { client, changed: previousClient !== client },
      }));

      if (options.scrollToTop) {
        const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        $(".client-tabs").scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth", block: "start" });
      }
    }

    tabs.forEach((tab, index) => {
      tab.addEventListener("click", () => activate(tab.dataset.clientTab, { scrollToTop: true }));
      tab.addEventListener("keydown", (event) => {
        if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
        event.preventDefault();
        let nextIndex = index;
        if (event.key === "ArrowLeft") nextIndex = (index - 1 + tabs.length) % tabs.length;
        if (event.key === "ArrowRight") nextIndex = (index + 1) % tabs.length;
        if (event.key === "Home") nextIndex = 0;
        if (event.key === "End") nextIndex = tabs.length - 1;
        activate(tabs[nextIndex].dataset.clientTab, { focusTab: true });
      });
    });

    const initialClient = window.location.hash === "#claude-code" ? "claude" : "codex";
    activate(initialClient, { scrollToTop: initialClient === "claude" });

    window.addEventListener("hashchange", () => {
      if (window.location.hash === "#claude-code") activate("claude");
      if (["#quick-start", "#install", "#create-key", "#configure", "#test", "#config-file", "#manual"].includes(window.location.hash)) {
        activate("codex");
      }
    });
  }

  function setupSecretField() {
    const input = $("#api-key");
    const button = $("#toggle-key");
    const use = $("use", button);

    button.addEventListener("click", () => {
      const isVisible = input.type === "text";
      input.type = isVisible ? "password" : "text";
      button.setAttribute("aria-label", isVisible ? "显示密钥" : "隐藏密钥");
      button.title = isVisible ? "显示密钥" : "隐藏密钥";
      use.setAttribute("href", `assets/icons.svg#${isVisible ? "eye" : "eye-off"}`);
    });
  }

  function setupImportForm() {
    const form = $("#setup-form");
    const baseUrlInput = $("#api-base-url");
    const apiKeyInput = $("#api-key");
    const message = $("#form-message");

    function invalidate(input, text) {
      input.classList.add("is-invalid");
      input.setAttribute("aria-invalid", "true");
      message.textContent = text;
      input.focus();
    }

    [baseUrlInput, apiKeyInput].forEach((input) => {
      input.addEventListener("input", () => {
        input.classList.remove("is-invalid");
        input.removeAttribute("aria-invalid");
        message.textContent = "";
        message.classList.remove("is-success");
      });
    });

    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const endpoint = normalizedBaseUrl(baseUrlInput.value);
      const apiKey = apiKeyInput.value.trim();

      if (!isHttpUrl(endpoint)) {
        invalidate(baseUrlInput, "请填写完整的 HTTPS API 接口地址。");
        return;
      }
      if (!apiKey) {
        invalidate(apiKeyInput, "请先粘贴 API 密钥。");
        return;
      }

      const params = new URLSearchParams({
        resource: "provider",
        app: "codex",
        name: config.providerName || config.siteName,
        endpoint,
        apiKey,
        enabled: "true",
      });
      const deepLink = `ccswitch://v1/import?${params.toString()}`;

      message.textContent = "已请求打开 CC Switch，请在弹出的窗口中确认导入。";
      message.classList.add("is-success");
      window.location.assign(deepLink);
    });
  }

  function showToast(text) {
    const toast = $("#toast");
    toast.textContent = text;
    toast.hidden = false;
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(() => {
      toast.hidden = true;
    }, 2400);
  }

  function setupCopyButtons() {
    async function copyText(text, target) {
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(text);
          return true;
        }
      } catch (_error) {
        // Fall through to the legacy copy path for local HTTP previews.
      }

      const helper = document.createElement("textarea");
      helper.value = text;
      helper.setAttribute("readonly", "");
      helper.style.position = "fixed";
      helper.style.top = "-1000px";
      helper.style.left = "-1000px";
      helper.style.opacity = "0";
      document.body.appendChild(helper);
      helper.focus();
      helper.select();
      const copied = document.execCommand("copy");
      helper.remove();

      if (!copied && target) {
        const selection = window.getSelection();
        const range = document.createRange();
        range.selectNodeContents(target);
        selection.removeAllRanges();
        selection.addRange(range);
      }
      return copied;
    }

    $$("[data-copy-target]").forEach((button) => {
      button.addEventListener("click", async () => {
        const target = $(`#${button.dataset.copyTarget}`);
        const copied = await copyText(target.innerText, target);
        if (copied) {
          const label = $("span", button);
          if (label) {
            const original = label.textContent;
            label.textContent = "已复制";
            window.setTimeout(() => {
              label.textContent = original;
            }, 1800);
          }
          showToast("内容已复制");
        } else {
          showToast("浏览器阻止了自动复制，请按 Ctrl/Cmd+C");
        }
      });
    });
  }

  function setupFaqAccordion() {
    const items = $$(".faq-list details");

    items.forEach((item) => {
      item.addEventListener("toggle", () => {
        if (!item.open) return;

        items.forEach((other) => {
          if (other !== item && other.open) other.open = false;
        });
      });
    });
  }

  function setupMobileMenu() {
    const button = $("#menu-toggle");
    const scrim = $("#mobile-scrim");
    const icon = $("svg", button);

    function setOpen(open) {
      document.body.classList.toggle("menu-open", open);
      button.setAttribute("aria-expanded", String(open));
      button.setAttribute("aria-label", open ? "关闭目录" : "打开目录");
      icon.innerHTML = open
        ? '<path d="M18 6 6 18"></path><path d="m6 6 12 12"></path>'
        : '<path d="M4 6h16"></path><path d="M4 12h16"></path><path d="M4 18h16"></path>';
      scrim.hidden = !open;
    }

    button.addEventListener("click", () => setOpen(!document.body.classList.contains("menu-open")));
    scrim.addEventListener("click", () => setOpen(false));
    $$(".doc-nav a").forEach((link) => link.addEventListener("click", () => setOpen(false)));
    window.addEventListener("keydown", (event) => {
      if (event.key === "Escape") setOpen(false);
    });
  }

  function setupActiveNavigation() {
    const links = $$(".nav-link[href^='#']");
    const sectionIds = [...new Set(links.map((link) => link.getAttribute("href").slice(1)))];
    const sections = sectionIds.map((id) => document.getElementById(id)).filter(Boolean);
    let releaseNavigationLock = null;

    function isVisible(section) {
      return !section.hidden && section.getClientRects().length > 0;
    }

    function stickyHeaderHeight() {
      const header = $(".mobile-header");
      return header && getComputedStyle(header).display !== "none" ? header.getBoundingClientRect().height : 0;
    }

    function activateLink(id) {
      links.forEach((link) => link.classList.remove("is-active"));
      links
        .filter((link) => link.getAttribute("href") === `#${id}` && !link.closest("[data-client-nav]")?.hidden)
        .forEach((link) => link.classList.add("is-active"));
    }

    function lockNavigationTo(id) {
      releaseNavigationLock?.();

      const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      let timer = 0;
      const release = () => {
        window.removeEventListener("scrollend", release);
        window.clearTimeout(timer);
        if (releaseNavigationLock === release) releaseNavigationLock = null;
      };

      releaseNavigationLock = release;
      activateLink(id);
      window.addEventListener("scrollend", release, { once: true });
      timer = window.setTimeout(release, reduceMotion ? 250 : 1400);
    }

    function scrollToSection(id, behavior) {
      const section = document.getElementById(id);
      if (!section || !isVisible(section)) return false;

      const targetTop = window.scrollY + section.getBoundingClientRect().top - stickyHeaderHeight();
      const maximumTop = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
      window.scrollTo({
        top: Math.min(Math.max(0, targetTop), maximumTop),
        behavior,
      });
      return true;
    }

    function navigateToSection(id, behavior) {
      if (!scrollToSection(id, behavior)) return false;
      lockNavigationTo(id);
      return true;
    }

    function updateHash(id) {
      if (window.location.hash !== `#${id}`) history.pushState(null, "", `#${id}`);
    }

    function handleAnchorClick(event) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

      const id = event.currentTarget.getAttribute("href").slice(1);
      if (!navigateToSection(id, window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth")) return;

      event.preventDefault();
      updateHash(id);
    }

    $$("a[href^='#']").forEach((link) => {
      const id = link.getAttribute("href").slice(1);
      if (sectionIds.includes(id)) link.addEventListener("click", handleAnchorClick);
    });

    let scrollFrame = 0;
    function syncActiveLink() {
      scrollFrame = 0;
      if (releaseNavigationLock) return;

      const marker = stickyHeaderHeight() + 1;
      const current = sections
        .filter(isVisible)
        .map((section) => ({ section, top: section.getBoundingClientRect().top }))
        .filter(({ top }) => top <= marker)
        .sort((a, b) => b.top - a.top)[0];

      if (current) activateLink(current.section.id);
    }

    window.addEventListener("scroll", () => {
      if (!scrollFrame) scrollFrame = window.requestAnimationFrame(syncActiveLink);
    }, { passive: true });

    function followHash() {
      const id = window.location.hash.slice(1);
      if (!sectionIds.includes(id)) return;
      window.requestAnimationFrame(() => navigateToSection(id, "auto"));
    }

    window.addEventListener("hashchange", followHash);
    document.addEventListener("clientchange", (event) => {
      if (!event.detail.changed) return;
      lockNavigationTo(event.detail.client === "claude" ? "claude-code" : "quick-start");
    });

    if (document.readyState === "complete") {
      window.setTimeout(followHash, 0);
    } else {
      window.addEventListener("load", () => window.setTimeout(followHash, 0), { once: true });
    }
  }

  applyConfig();
  setupOperatingSystemTabs();
  setupSecretField();
  setupImportForm();
  setupCopyButtons();
  setupFaqAccordion();
  setupMobileMenu();
  setupActiveNavigation();
  setupClientTabs();
  loadPublicSettings();
})();
