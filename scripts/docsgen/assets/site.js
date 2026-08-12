(function () {
  "use strict";

  var STORAGE_KEY = "pretty-pdf-site-theme";
  var root = document.documentElement;

  function applyTheme(name) {
    root.setAttribute("data-site-theme", name);
    var buttons = document.querySelectorAll(".theme-swatch");
    for (var i = 0; i < buttons.length; i++) {
      var isActive = buttons[i].getAttribute("data-theme") === name;
      buttons[i].setAttribute("aria-pressed", isActive ? "true" : "false");
    }
    // Sync the shared top-appbar theme dropdown (nav.css markup).
    document.querySelectorAll(".theme-option").forEach(function (el) {
      el.classList.toggle("active", el.dataset.name === name);
    });
    var navLabel = document.getElementById("navThemeLabel");
    if (navLabel) navLabel.textContent = name;
    var navSwatches = document.getElementById("navThemeSwatches");
    var pickedOption = document.querySelector('.theme-option[data-name="' + name + '"] .swatches');
    if (navSwatches && pickedOption) navSwatches.innerHTML = pickedOption.innerHTML;
    updateDownloadLink(name);
  }

  // Keeps the "Download these docs as a PDF" button pointed at the PDF
  // that matches whatever theme is currently on screen — docsgen
  // pre-renders one PDF per builtin theme (go-pretty-pdf-docs-<id>.pdf),
  // so this is just picking the right static file, not generating anything.
  function updateDownloadLink(name) {
    var link = document.getElementById("download-pdf-btn");
    var sub = document.getElementById("download-pdf-sub");
    if (link) link.setAttribute("href", "go-pretty-pdf-docs-" + name + ".pdf");
    if (sub) {
      var swatchLabel = document.querySelector('.theme-swatch[data-theme="' + name + '"] .theme-swatch-label');
      var displayName = swatchLabel ? swatchLabel.textContent : name;
      sub.textContent = "in the " + displayName + " theme — rendered by go-pretty-pdf itself";
    }
  }

  function initThemeSwitcher() {
    var saved = null;
    try {
      saved = localStorage.getItem(STORAGE_KEY);
    } catch (e) {
      /* localStorage unavailable (privacy mode) — fall back to default */
    }
    applyTheme(saved || root.getAttribute("data-site-theme") || "default");

    var buttons = document.querySelectorAll(".theme-swatch");
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].addEventListener("click", function () {
        var name = this.getAttribute("data-theme");
        applyTheme(name);
        try {
          localStorage.setItem(STORAGE_KEY, name);
        } catch (e) {
          /* ignore */
        }
      });
    }
  }

  // Distance from the viewport top the section heading should rest at after
  // a drawer jump — the sticky appbar (71px) plus a little breathing room.
  var NAV_OFFSET = 84;

  function setActiveLink(id) {
    var links = document.querySelectorAll(".sidebar-nav a");
    for (var i = 0; i < links.length; i++) {
      links[i].classList.toggle("is-active", links[i].getAttribute("href") === "#" + id);
    }
  }

  // Scroll-spy: highlights the drawer item for the section whose top is
  // closest to the line just under the sticky appbar. A "nearest to line"
  // approach (rather than an intersection band) also works for sections
  // near the bottom of a very long page, which can never scroll high enough
  // to enter a fixed band.
  function initScrollSpy() {
    var links = document.querySelectorAll(".sidebar-nav a");
    var sections = [];
    links.forEach(function (link) {
      var el = document.getElementById(link.getAttribute("href").slice(1));
      if (el) sections.push(el);
    });
    if (sections.length === 0) return;

    var ticking = false;
    function onScroll() {
      if (ticking) return;
      ticking = true;
      window.requestAnimationFrame(function () {
        var best = sections[0];
        var bestDist = Infinity;
        for (var i = 0; i < sections.length; i++) {
          var dist = Math.abs(sections[i].getBoundingClientRect().top - NAV_OFFSET);
          if (dist < bestDist) {
            bestDist = dist;
            best = sections[i];
          }
        }
        setActiveLink(best.id);
        ticking = false;
      });
    }
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    onScroll();
  }

  // Drawer navigation: replaces the native anchor jump with a precise,
  // clamped scroll to the section heading (offset below the appbar) and
  // marks the tapped item active immediately — the observer-based spy can
  // lag or pick a different item on long pages, which made jumps feel like
  // they landed "somewhere else".
  function initSectionNav() {
    var nav = document.querySelector(".sidebar-nav");
    if (!nav) return;
    nav.addEventListener("click", function (e) {
      var link = e.target.closest('a[href^="#"]');
      if (!link) return;
      var target = document.getElementById(link.getAttribute("href").slice(1));
      if (!target) return;
      e.preventDefault();
      setActiveLink(target.id);
      if (history.replaceState) history.replaceState(null, "", "#" + target.id);
      var top = target.getBoundingClientRect().top + window.pageYOffset - NAV_OFFSET;
      window.scrollTo({ top: Math.max(top, 0), behavior: "smooth" });
    });
  }

  function isApplePlatform() {
    var platform = (navigator.userAgentData && navigator.userAgentData.platform) ||
      navigator.platform || navigator.userAgent || "";
    return /Mac|iPhone|iPad|iPod/i.test(platform);
  }

  function initShortcutHint() {
    var hint = document.getElementById("palette-shortcut-hint");
    if (!hint) return;
    hint.textContent = isApplePlatform() ? "⌘ K" : "Ctrl K";
  }

  function initNavToggle() {
    var toggle = document.getElementById("nav-toggle");
    var nav = document.getElementById("sidebar-nav");
    if (!toggle || !nav) return;

    function close() {
      nav.classList.remove("is-open");
      toggle.setAttribute("aria-expanded", "false");
    }

    toggle.addEventListener("click", function () {
      var open = nav.classList.toggle("is-open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });

    nav.querySelectorAll("a").forEach(function (link) {
      link.addEventListener("click", close);
    });
  }

  // ---------- shared top-appbar theme dropdown (nav.css markup) ----------
  function initNavThemeDropdown() {
    var dropdown = document.querySelector(".theme-dropdown");
    var btn = document.getElementById("themeDropdownBtn");
    if (!dropdown || !btn) return;

    function close() {
      dropdown.classList.remove("open");
      btn.setAttribute("aria-expanded", "false");
    }
    function toggle() {
      var willOpen = !dropdown.classList.contains("open");
      dropdown.classList.toggle("open", willOpen);
      btn.setAttribute("aria-expanded", willOpen ? "true" : "false");
      if (willOpen) {
        var menuBtn = document.getElementById("menuBtn");
        if (menuBtn) {
          menuBtn.setAttribute("aria-expanded", "false");
          var header = menuBtn.closest("header");
          if (header) header.classList.remove("menu-open");
        }
      }
    }

    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      toggle();
    });
    dropdown.querySelectorAll(".theme-option").forEach(function (opt) {
      opt.addEventListener("click", function () {
        var name = opt.dataset.name;
        applyTheme(name);
        try { localStorage.setItem(STORAGE_KEY, name); } catch (e) {}
        close();
      });
    });
    document.addEventListener("click", function (e) {
      if (!dropdown.contains(e.target)) close();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") close();
    });
  }

  // ---------- shared mobile hamburger menu (nav.css markup) ----------
  function initNavMenu() {
    var header = document.querySelector("header.nav");
    var btn = document.getElementById("menuBtn");
    var links = document.getElementById("navLinks");
    if (!header || !btn || !links) return;

    function close() {
      header.classList.remove("menu-open");
      btn.setAttribute("aria-expanded", "false");
    }
    function open() {
      header.classList.add("menu-open");
      btn.setAttribute("aria-expanded", "true");
      var dropdown = document.querySelector(".theme-dropdown");
      if (dropdown) dropdown.classList.remove("open");
      var ddBtn = document.getElementById("themeDropdownBtn");
      if (ddBtn) ddBtn.setAttribute("aria-expanded", "false");
    }
    function toggle() {
      var willOpen = !header.classList.contains("menu-open");
      if (willOpen) open(); else close();
    }

    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      toggle();
    });
    links.addEventListener("click", function (e) {
      if (e.target.closest("a")) close();
    });
    document.addEventListener("click", function (e) {
      if (!header.contains(e.target)) close();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") close();
    });
    window.addEventListener("resize", function () {
      if (window.innerWidth > 1100) close();
    });
  }

  function initCommandPalette() {
    var palette = document.getElementById("command-palette");
    var input = document.getElementById("command-palette-input");
    var results = document.getElementById("command-palette-results");
    var trigger = document.getElementById("palette-trigger");
    if (!palette || !input || !results || !trigger) return;

    var index = Array.prototype.map.call(
      document.querySelectorAll(".sidebar-nav a"),
      function (link) {
        return { title: link.textContent.trim(), href: link.getAttribute("href") };
      }
    );

    var selectedIndex = 0;
    var visible = [];

    function render(items) {
      visible = items;
      selectedIndex = 0;
      if (items.length === 0) {
        results.innerHTML = '<li class="command-palette-empty">No matching section.</li>';
        return;
      }
      results.innerHTML = items
        .map(function (item, i) {
          return (
            '<li class="' + (i === 0 ? "is-selected" : "") + '" data-index="' + i + '">' +
            '<a href="' + item.href + '">' + item.title + "</a></li>"
          );
        })
        .join("");
    }

    function filter(query) {
      var q = query.trim().toLowerCase();
      if (!q) return render(index);
      var matches = index
        .map(function (item) {
          return { item: item, pos: item.title.toLowerCase().indexOf(q) };
        })
        .filter(function (m) { return m.pos !== -1; })
        .sort(function (a, b) { return a.pos - b.pos; })
        .map(function (m) { return m.item; });
      render(matches);
    }

    function updateSelection(next) {
      var items = results.querySelectorAll("li");
      if (items.length === 0) return;
      items[selectedIndex] && items[selectedIndex].classList.remove("is-selected");
      selectedIndex = (next + items.length) % items.length;
      items[selectedIndex].classList.add("is-selected");
      items[selectedIndex].scrollIntoView({ block: "nearest" });
    }

    function open() {
      palette.hidden = false;
      document.body.style.overflow = "hidden";
      input.value = "";
      render(index);
      setTimeout(function () { input.focus(); }, 0);
    }

    function close() {
      palette.hidden = true;
      document.body.style.overflow = "";
      trigger.focus();
    }

    function commit() {
      var item = visible[selectedIndex];
      if (!item) return;
      close();
      var target = document.querySelector(item.href);
      if (target) target.scrollIntoView({ block: "start" });
      history.replaceState(null, "", item.href);
    }

    trigger.addEventListener("click", open);

    palette.querySelectorAll("[data-palette-close]").forEach(function (el) {
      el.addEventListener("click", close);
    });

    results.addEventListener("click", function (e) {
      var link = e.target.closest("a");
      if (!link) return;
      e.preventDefault();
      var li = link.closest("li");
      selectedIndex = Array.prototype.indexOf.call(results.children, li);
      commit();
    });

    input.addEventListener("input", function () { filter(input.value); });

    input.addEventListener("keydown", function (e) {
      if (e.key === "ArrowDown") { e.preventDefault(); updateSelection(selectedIndex + 1); }
      else if (e.key === "ArrowUp") { e.preventDefault(); updateSelection(selectedIndex - 1); }
      else if (e.key === "Enter") { e.preventDefault(); commit(); }
      else if (e.key === "Escape") { e.preventDefault(); close(); }
    });

    document.addEventListener("keydown", function (e) {
      var isMod = e.metaKey || e.ctrlKey;
      if (isMod && e.key.toLowerCase() === "k") {
        e.preventDefault();
        if (palette.hidden) open(); else close();
      }
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    initThemeSwitcher();
    initScrollSpy();
    initSectionNav();
    initNavToggle();
    initCommandPalette();
    initShortcutHint();
    initNavThemeDropdown();
    initNavMenu();
  });
})();
