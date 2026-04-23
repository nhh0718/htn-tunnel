/* ============================================================
   htn-tunnel Architecture Article — article.js
   - Reading progress bar
   - Scroll spy → highlight TOC
   - Copy buttons for code blocks
   - Mobile TOC toggle
   - Smooth anchor scroll + anchor icons
   ============================================================ */

(function () {
  'use strict';

  document.addEventListener('DOMContentLoaded', init);

  function init() {
    setupReadingProgress();
    setupScrollSpy();
    setupCopyButtons();
    setupMobileNav();
    setupMobileToc();
    setupAnchorLinks();
  }

  /* ── Reading progress ───────────────────────────────────── */

  function setupReadingProgress() {
    var bar = document.querySelector('.reading-progress-bar');
    if (!bar) return;

    window.addEventListener('scroll', function () {
      var scrollTop = window.scrollY;
      var docHeight = document.documentElement.scrollHeight - window.innerHeight;
      var pct = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
      bar.style.width = Math.min(pct, 100) + '%';
    }, { passive: true });
  }

  /* ── Scroll spy ─────────────────────────────────────────── */

  function setupScrollSpy() {
    var headings = Array.from(
      document.querySelectorAll('.article-content h2[id], .article-content h3[id]')
    );
    if (!headings.length) return;

    var activeId = null;
    var ticking  = false;

    function findActive() {
      var scrollY = window.scrollY + 100;
      var current = null;
      for (var i = 0; i < headings.length; i++) {
        if (headings[i].offsetTop <= scrollY) {
          current = headings[i].id;
        }
      }
      return current;
    }

    function updateToc(id) {
      document.querySelectorAll('.toc-list a').forEach(function (a) {
        var linkId = (a.getAttribute('href') || '').replace('#', '');
        a.classList.toggle('active', linkId === id);
      });
    }

    window.addEventListener('scroll', function () {
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(function () {
        var current = findActive();
        if (current && current !== activeId) {
          activeId = current;
          updateToc(activeId);
        }
        ticking = false;
      });
    }, { passive: true });

    // Init on load
    var initial = findActive();
    if (initial) { activeId = initial; updateToc(activeId); }
  }

  /* ── Copy buttons ───────────────────────────────────────── */

  function setupCopyButtons() {
    document.querySelectorAll('.code-block').forEach(function (block) {
      var btn = block.querySelector('.copy-btn');
      if (!btn) {
        btn = document.createElement('button');
        btn.className = 'copy-btn';
        btn.setAttribute('aria-label', 'Copy code');
        btn.textContent = 'Copy';
        block.appendChild(btn);
      }

      btn.addEventListener('click', function () {
        var pre = block.querySelector('pre');
        if (!pre) return;
        var text = pre.innerText.trim();
        copyToClipboard(text, btn);
      });
    });
  }

  function copyToClipboard(text, btn) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(function () {
        flashCopied(btn);
      }).catch(function () { fallbackCopy(text, btn); });
    } else {
      fallbackCopy(text, btn);
    }
  }

  function fallbackCopy(text, btn) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.cssText = 'position:fixed;top:-9999px;left:-9999px;opacity:0';
    document.body.appendChild(ta);
    ta.focus(); ta.select();
    try { document.execCommand('copy'); } catch (_) { /* silent */ }
    document.body.removeChild(ta);
    flashCopied(btn);
  }

  function flashCopied(btn) {
    var orig = btn.textContent;
    btn.textContent = 'Copied!';
    btn.classList.add('copied');
    setTimeout(function () {
      btn.textContent = orig;
      btn.classList.remove('copied');
    }, 2000);
  }

  /* ── Mobile top nav ─────────────────────────────────────── */

  function setupMobileNav() {
    var hamburger = document.getElementById('hamburger');
    var drawer    = document.getElementById('nav-drawer');
    if (!hamburger || !drawer) return;

    hamburger.addEventListener('click', function () {
      var isOpen = drawer.classList.toggle('open');
      hamburger.classList.toggle('open', isOpen);
      hamburger.setAttribute('aria-expanded', String(isOpen));
    });

    drawer.querySelectorAll('a').forEach(function (link) {
      link.addEventListener('click', function () {
        drawer.classList.remove('open');
        hamburger.classList.remove('open');
        hamburger.setAttribute('aria-expanded', 'false');
      });
    });

    document.addEventListener('click', function (e) {
      if (!drawer.contains(e.target) && !hamburger.contains(e.target)) {
        drawer.classList.remove('open');
        hamburger.classList.remove('open');
        hamburger.setAttribute('aria-expanded', 'false');
      }
    });
  }

  /* ── Mobile TOC ─────────────────────────────────────────── */

  function setupMobileToc() {
    var toggle = document.getElementById('toc-toggle');
    var body   = document.getElementById('toc-mobile-body');
    if (!toggle || !body) return;

    toggle.addEventListener('click', function () {
      var isOpen = body.classList.toggle('open');
      toggle.setAttribute('aria-expanded', String(isOpen));
      var arrow = toggle.querySelector('.toc-arrow');
      if (arrow) arrow.textContent = isOpen ? '▲' : '▼';
    });

    // Close on link click
    body.querySelectorAll('a').forEach(function (a) {
      a.addEventListener('click', function () {
        body.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
        var arrow = toggle.querySelector('.toc-arrow');
        if (arrow) arrow.textContent = '▼';
      });
    });
  }

  /* ── Smooth anchor + icons ──────────────────────────────── */

  function setupAnchorLinks() {
    // Append # icon to headings that don't have one
    document.querySelectorAll('.article-content h2[id], .article-content h3[id]').forEach(function (h) {
      if (h.querySelector('.anchor-link')) return;
      var a = document.createElement('a');
      a.className = 'anchor-link';
      a.href = '#' + h.id;
      a.setAttribute('aria-hidden', 'true');
      a.textContent = '#';
      h.appendChild(a);
    });

    // Smooth scroll all in-page anchors
    document.addEventListener('click', function (e) {
      var a = e.target.closest('a[href^="#"]');
      if (!a) return;
      var id = a.getAttribute('href').slice(1);
      var target = id ? document.getElementById(id) : null;
      if (!target) return;
      e.preventDefault();
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
      history.replaceState(null, '', '#' + id);
    });
  }

}());
