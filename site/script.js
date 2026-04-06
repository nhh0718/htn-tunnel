/* ============================================================
   htn-tunnel docs — minimal vanilla JS
   - Copy buttons for code blocks
   - Mobile nav toggle
   - Smooth anchor scroll
   - Fade-in via IntersectionObserver
   ============================================================ */

(function () {
  'use strict';

  /* ── DOM ready ─────────────────────────────────────────── */
  document.addEventListener('DOMContentLoaded', init);

  function init() {
    setupCopyButtons();
    setupMobileNav();
    setupFadeIn();
  }

  /* ── Copy buttons ──────────────────────────────────────── */

  function setupCopyButtons() {
    document.querySelectorAll('.code-block').forEach(function (block) {
      var btn = block.querySelector('.copy-btn');
      if (!btn) return;

      btn.addEventListener('click', function () {
        var pre = block.querySelector('pre');
        if (!pre) return;

        // Strip leading $ / # prompt chars and extract plain text
        var text = pre.innerText
          .split('\n')
          .map(function (line) { return line.replace(/^[$#]\s+/, ''); })
          .join('\n')
          .trim();

        copyToClipboard(text, btn);
      });
    });
  }

  function copyToClipboard(text, btn) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(function () {
        flashCopied(btn);
      }).catch(function () {
        fallbackCopy(text, btn);
      });
    } else {
      fallbackCopy(text, btn);
    }
  }

  function fallbackCopy(text, btn) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.cssText = 'position:fixed;top:-9999px;left:-9999px;opacity:0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    try { document.execCommand('copy'); } catch (_) { /* silent */ }
    document.body.removeChild(ta);
    flashCopied(btn);
  }

  function flashCopied(btn) {
    var original = btn.textContent;
    btn.textContent = 'Copied!';
    btn.classList.add('copied');
    setTimeout(function () {
      btn.textContent = original;
      btn.classList.remove('copied');
    }, 2000);
  }

  /* ── Mobile nav ────────────────────────────────────────── */

  function setupMobileNav() {
    var hamburger = document.getElementById('hamburger');
    var drawer    = document.getElementById('nav-drawer');
    if (!hamburger || !drawer) return;

    hamburger.addEventListener('click', function () {
      var isOpen = drawer.classList.toggle('open');
      hamburger.classList.toggle('open', isOpen);
      hamburger.setAttribute('aria-expanded', String(isOpen));
    });

    // Close drawer on link click
    drawer.querySelectorAll('a').forEach(function (link) {
      link.addEventListener('click', function () {
        drawer.classList.remove('open');
        hamburger.classList.remove('open');
        hamburger.setAttribute('aria-expanded', 'false');
      });
    });

    // Close on outside click
    document.addEventListener('click', function (e) {
      if (!drawer.contains(e.target) && !hamburger.contains(e.target)) {
        drawer.classList.remove('open');
        hamburger.classList.remove('open');
        hamburger.setAttribute('aria-expanded', 'false');
      }
    });
  }

  /* ── Fade-in on scroll ─────────────────────────────────── */

  function setupFadeIn() {
    if (!('IntersectionObserver' in window)) {
      // Fallback: show everything immediately
      document.querySelectorAll('.fade-in').forEach(function (el) {
        el.classList.add('visible');
      });
      return;
    }

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          observer.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });

    document.querySelectorAll('.fade-in').forEach(function (el) {
      observer.observe(el);
    });
  }

}());
