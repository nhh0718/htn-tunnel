/* ============================================================
   htn-tunnel Docs — docs.js
   - Scroll spy: highlights sidebar + outline on scroll
   - Smooth anchor navigation
   - Copy buttons for all code blocks
   - Mobile sidebar toggle
   - FAQ accordion
   - Reading progress bar
   ============================================================ */

(function () {
  'use strict';

  document.addEventListener('DOMContentLoaded', init);

  function init() {
    setupCopyButtons();
    setupScrollSpy();
    setupMobileNav();
    setupMobileSidebar();
    setupFAQ();
    setupReadingProgress();
    setupAnchorLinks();
  }

  /* ── Copy buttons ──────────────────────────────────────── */

  function setupCopyButtons() {
    document.querySelectorAll('.code-block').forEach(function (block) {
      // Add button if not already present in HTML
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

        // Extract plain text, strip prompt chars $ and #
        var text = pre.innerText
          .split('\n')
          .map(function (line) { return line.replace(/^[$#]\s*/, ''); })
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
    var orig = btn.textContent;
    btn.textContent = 'Copied!';
    btn.classList.add('copied');
    setTimeout(function () {
      btn.textContent = orig;
      btn.classList.remove('copied');
    }, 2000);
  }

  /* ── Scroll spy ─────────────────────────────────────────── */

  function setupScrollSpy() {
    if (!('IntersectionObserver' in window)) return;

    var headings = Array.from(
      document.querySelectorAll('.chapter h2[id], .chapter h3[id]')
    );
    if (!headings.length) return;

    // Track which heading is currently in view
    var activeId = null;

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          activeId = entry.target.id;
          updateActiveLinks(activeId);
        }
      });
    }, {
      rootMargin: '-60px 0px -65% 0px',
      threshold: 0
    });

    headings.forEach(function (h) { observer.observe(h); });

    // Fallback: on scroll find topmost visible heading
    var ticking = false;
    window.addEventListener('scroll', function () {
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(function () {
        var scrollY = window.scrollY + 80;
        var current = null;
        headings.forEach(function (h) {
          if (h.offsetTop <= scrollY) current = h.id;
        });
        if (current && current !== activeId) {
          activeId = current;
          updateActiveLinks(activeId);
        }
        ticking = false;
      });
    }, { passive: true });
  }

  function updateActiveLinks(id) {
    if (!id) return;

    // Sidebar chapter links (h2)
    document.querySelectorAll('.sidebar-nav .chapter-link').forEach(function (a) {
      var href = a.getAttribute('href');
      var linkId = href ? href.replace('#', '') : '';
      a.classList.toggle('active', linkId === id);

      // Show sub-nav for active chapter
      var subnav = a.closest('li') && a.closest('li').querySelector('.sidebar-subnav');
      if (subnav) {
        subnav.classList.toggle('visible', a.classList.contains('active') || subnav.querySelector('a.active'));
      }
    });

    // Sidebar sub-links (h3)
    document.querySelectorAll('.sidebar-subnav a').forEach(function (a) {
      var href = a.getAttribute('href');
      var linkId = href ? href.replace('#', '') : '';
      var isActive = linkId === id;
      a.classList.toggle('active', isActive);

      // If a sub-link is active, show its parent subnav + activate parent chapter
      if (isActive) {
        var subnav = a.closest('.sidebar-subnav');
        if (subnav) {
          subnav.classList.add('visible');
          var parentLi = subnav.closest('li');
          if (parentLi) {
            var parentChapter = parentLi.querySelector('.chapter-link');
            if (parentChapter) parentChapter.classList.add('active');
          }
        }
      }
    });

    // Outline links
    document.querySelectorAll('.outline-list a').forEach(function (a) {
      var href = a.getAttribute('href');
      var linkId = href ? href.replace('#', '') : '';
      a.classList.toggle('active', linkId === id);
    });
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

  /* ── Mobile docs sidebar toggle ─────────────────────────── */

  function setupMobileSidebar() {
    var toggle  = document.getElementById('sidebar-toggle');
    var sidebar = document.querySelector('.docs-sidebar');
    if (!toggle || !sidebar) return;

    toggle.addEventListener('click', function () {
      var isOpen = sidebar.classList.toggle('open');
      toggle.setAttribute('aria-expanded', String(isOpen));
      toggle.querySelector('.toggle-label').textContent =
        isOpen ? 'Ẩn mục lục' : 'Mục lục';
    });

    // Close sidebar when a link inside is clicked (mobile)
    sidebar.querySelectorAll('a').forEach(function (a) {
      a.addEventListener('click', function () {
        if (window.innerWidth <= 768) {
          sidebar.classList.remove('open');
          if (toggle) {
            toggle.setAttribute('aria-expanded', 'false');
            var lbl = toggle.querySelector('.toggle-label');
            if (lbl) lbl.textContent = 'Mục lục';
          }
        }
      });
    });

    // Close on outside click
    document.addEventListener('click', function (e) {
      if (
        window.innerWidth <= 768 &&
        sidebar.classList.contains('open') &&
        !sidebar.contains(e.target) &&
        !toggle.contains(e.target)
      ) {
        sidebar.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
        var lbl = toggle.querySelector('.toggle-label');
        if (lbl) lbl.textContent = 'Mục lục';
      }
    });
  }

  /* ── FAQ accordion ──────────────────────────────────────── */

  function setupFAQ() {
    document.querySelectorAll('.faq-question').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var item = btn.closest('.faq-item');
        if (!item) return;
        var wasOpen = item.classList.contains('open');

        // Close all others
        document.querySelectorAll('.faq-item.open').forEach(function (el) {
          el.classList.remove('open');
        });

        // Toggle clicked
        if (!wasOpen) item.classList.add('open');
      });
    });
  }

  /* ── Reading progress bar ───────────────────────────────── */

  function setupReadingProgress() {
    var bar = document.querySelector('.reading-progress-bar');
    if (!bar) return;

    window.addEventListener('scroll', function () {
      var scrollTop  = window.scrollY;
      var docHeight  = document.documentElement.scrollHeight - window.innerHeight;
      var pct        = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
      bar.style.width = Math.min(pct, 100) + '%';
    }, { passive: true });
  }

  /* ── Smooth anchor links (add # icon to h2/h3) ──────────── */

  function setupAnchorLinks() {
    document.querySelectorAll('.chapter h2[id], .chapter h3[id]').forEach(function (h) {
      // anchor icon already in HTML for h2; skip if present
      if (h.querySelector('.anchor-link')) return;
      var a = document.createElement('a');
      a.className = 'anchor-link';
      a.href = '#' + h.id;
      a.setAttribute('aria-hidden', 'true');
      a.textContent = '#';
      h.appendChild(a);
    });

    // Smooth scroll for all in-page anchor links
    document.addEventListener('click', function (e) {
      var a = e.target.closest('a[href^="#"]');
      if (!a) return;
      var id = a.getAttribute('href').slice(1);
      var target = id ? document.getElementById(id) : null;
      if (!target) return;
      e.preventDefault();
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
      // Update URL hash without jump
      history.replaceState(null, '', '#' + id);
    });
  }

}());
