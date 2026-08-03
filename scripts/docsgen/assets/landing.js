(function () {
  "use strict";

  var STORAGE_KEY = "pretty-pdf-site-theme";

  // ---------- copy to clipboard ----------
  function wireCopy(btnId, text) {
    var btn = document.getElementById(btnId);
    if (!btn) return;
    var original = btn.innerHTML;
    btn.addEventListener("click", function () {
      if (navigator.clipboard) {
        navigator.clipboard.writeText(text).catch(function () {});
      }
      btn.classList.add("copied");
      btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6L9 17l-5-5"/></svg>';
      setTimeout(function () {
        btn.classList.remove("copied");
        btn.innerHTML = original;
      }, 1600);
    });
  }

  // ---------- reveal on scroll ----------
  function initReveal() {
    var els = document.querySelectorAll(".reveal");
    if (!("IntersectionObserver" in window)) {
      els.forEach(function (el) { el.classList.add("in"); });
      return;
    }
    var io = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("in");
            io.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12 }
    );
    els.forEach(function (el) { io.observe(el); });
  }

  // ---------- terminal typing animation ----------
  var termLines = [
    { t: "heading", text: "  Pre-flight checks" },
    { t: "ok", text: "  ✓ Chrome/Chromium available" },
    { t: "ok", text: "  ✓ Source directory (12 .md/.mdx files)" },
    { t: "ok", text: "  ✓ Output directory writable (./book.pdf)" },
    { t: "gap" },
    { t: "ok", text: "  ✓ Parsing MDX files..." },
    { t: "ok", text: "  ✓ Running validation..." },
    { t: "ok", text: "  ✓ Composing HTML..." },
    { t: "ok", text: "  ✓ Rendering PDF..." },
    { t: "gap" },
    { t: "heading", text: "  Build Complete!" },
    { t: "muted", text: "  Documents: 12" },
    { t: "muted", text: "  Output: book.pdf (812 kB)" },
    { t: "muted", text: "  Duration: 640ms" },
    { t: "accent", text: "  Theme: gruvbox" },
    { t: "muted", text: "  Warnings: 0" }
  ];

  function renderTerminal() {
    var body = document.getElementById("termBody");
    if (!body) return;
    var prefersReduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    body.innerHTML = "";

    var cmdLine = document.createElement("div");
    cmdLine.className = "line";
    cmdLine.style.opacity = 1;
    body.appendChild(cmdLine);
    var promptSpan = document.createElement("span");
    promptSpan.className = "tl-prompt";
    promptSpan.textContent = "$ ";
    var cmdSpan = document.createElement("span");
    cmdSpan.className = "tl-cmd";
    cmdLine.appendChild(promptSpan);
    cmdLine.appendChild(cmdSpan);
    var cursor = document.createElement("span");
    cursor.className = "cursor";
    cmdLine.appendChild(cursor);

    var fullCmd = "pretty-pdf build --source ./book --out book.pdf";

    function appendRest(idx) {
      if (idx >= termLines.length) return;
      var item = termLines[idx];
      var div = document.createElement("div");
      div.className = "line";
      if (item.t === "gap") {
        div.innerHTML = "&nbsp;";
      } else {
        div.classList.add("tl-" + item.t);
        div.textContent = item.text;
      }
      body.appendChild(div);
      setTimeout(function () { appendRest(idx + 1); }, item.t === "gap" ? 90 : 65);
    }

    if (prefersReduced) {
      cmdSpan.textContent = fullCmd;
      cursor.remove();
      appendRest(0);
      return;
    }

    var i = 0;
    function typeChar() {
      if (i <= fullCmd.length) {
        cmdSpan.textContent = fullCmd.slice(0, i);
        i++;
        setTimeout(typeChar, 26 + Math.random() * 22);
      } else {
        cursor.remove();
        setTimeout(function () { appendRest(0); }, 260);
      }
    }
    typeChar();
  }

  // ---------- live theme switcher ----------
  // Colors AND fonts come entirely from CSS ([data-theme="x"] rules
  // generated server-side by landing.go from each theme's real
  // --pdf-bg/--pdf-accent/--pdf-font-* values) — this just flips the
  // attribute, so there is exactly one source of truth for every theme's
  // look, shared by the nav dropdown and the Themes-section gallery.
  function applyTheme(name, persist) {
    document.documentElement.setAttribute("data-theme", name);
    document.querySelectorAll(".theme-card, .theme-option").forEach(function (el) {
      el.classList.toggle("active", el.dataset.name === name);
    });

    var sectionLabel = document.getElementById("activeThemeLabel");
    if (sectionLabel) sectionLabel.textContent = name;
    var navLabel = document.getElementById("navThemeLabel");
    if (navLabel) navLabel.textContent = name;

    var navSwatches = document.getElementById("navThemeSwatches");
    var pickedOption = document.querySelector('.theme-option[data-name="' + name + '"] .swatches');
    if (navSwatches && pickedOption) navSwatches.innerHTML = pickedOption.innerHTML;

    var pdfLink = document.getElementById("themePdfLink");
    if (pdfLink) pdfLink.setAttribute("href", "go-pretty-pdf-docs-" + name + ".pdf");

    if (persist) {
      try { localStorage.setItem(STORAGE_KEY, name); } catch (e) {}
    }
  }

  function initThemeSwitcher() {
    var options = document.querySelectorAll(".theme-card, .theme-option");
    if (!options.length) return;

    var saved = null;
    try { saved = localStorage.getItem(STORAGE_KEY); } catch (e) {}
    if (saved && document.querySelector('[data-name="' + saved + '"]')) {
      applyTheme(saved, false);
    }

    options.forEach(function (el) {
      var name = el.dataset.name;
      el.addEventListener("click", function () { applyTheme(name, true); });
      el.addEventListener("keydown", function (e) {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          applyTheme(name, true);
        }
      });
    });
  }

  // ---------- nav theme dropdown open/close ----------
  function initThemeDropdown() {
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
    }

    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      toggle();
    });
    dropdown.querySelectorAll(".theme-option").forEach(function (opt) {
      opt.addEventListener("click", close);
    });
    document.addEventListener("click", function (e) {
      if (!dropdown.contains(e.target)) close();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") close();
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    wireCopy("copyInstall", "go install github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf@latest");
    wireCopy("copyInit", "pretty-pdf init my-book");
    initReveal();
    renderTerminal();
    initThemeSwitcher();
    initThemeDropdown();
  });
})();
