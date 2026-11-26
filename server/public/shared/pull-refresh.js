(function (w) {
  function hasScrolledAncestor(el) {
    var cur = el;
    while (cur && cur !== document.body && cur !== document.documentElement) {
      if (cur.classList && (cur.classList.contains("modal") || cur.classList.contains("modal-card") || cur.classList.contains("chat-scroll-box"))) return true;
      var tag = cur.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
      if (cur.scrollTop && cur.scrollTop > 2) return true;
      cur = cur.parentElement;
    }
    return false;
  }

  function pageScrollTop() {
    return window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
  }

  function ensureIndicator() {
    var el = document.getElementById("gbPullRefreshBar");
    if (el) return el;
    var st = document.createElement("style");
    st.textContent =
      "#gbPullRefreshBar{position:fixed;left:50%;top:0;transform:translate(-50%,-120%);z-index:9990;" +
      "display:flex;align-items:center;gap:6px;padding:6px 14px;border-radius:999px;" +
      "background:#f3f9f5;color:#2f6b4f;border:1px solid #c5dfd0;box-shadow:0 6px 18px rgba(35,49,40,.12);" +
      "font-size:12.5px;font-weight:700;pointer-events:none;transition:transform .18s ease,opacity .18s ease;opacity:0}" +
      "#gbPullRefreshBar.ready{background:#fff8eb;color:#b8621b;border-color:#efd2ab}";
    document.head.appendChild(st);
    el = document.createElement("div");
    el.id = "gbPullRefreshBar";
    el.textContent = "↓ 下拉刷新";
    document.body.appendChild(el);
    return el;
  }

  w.GBPullRefresh = {
    enable: function (opts) {
      if (!opts || typeof opts.onRefresh !== "function") return;
      var shouldAllow = typeof opts.shouldAllow === "function" ? opts.shouldAllow : function () { return true; };
      var threshold = opts.thresholdPx || 62;
      var bar = ensureIndicator();

      var armed = false, pulling = false, refreshing = false;
      var sx = 0, sy = 0, pullDist = 0, lastAutoTs = Date.now();

      function resetBar() {
        bar.style.transition = "transform .2s ease, opacity .2s ease";
        bar.style.transform = "translate(-50%, -120%)";
        bar.style.opacity = "0";
        bar.classList.remove("ready");
      }

      async function triggerRefresh() {
        if (refreshing) return;
        refreshing = true;
        bar.classList.remove("ready");
        bar.style.transition = "transform .18s ease, opacity .18s ease";
        bar.style.transform = "translate(-50%, 14px)";
        bar.style.opacity = "1";
        bar.textContent = "⟳ 正在刷新...";
        var started = Date.now();
        try {
          await opts.onRefresh();
        } catch (e) {}
        lastAutoTs = Date.now();
        var wait = Math.max(0, 320 - (Date.now() - started));
        setTimeout(function () {
          refreshing = false;
          resetBar();
        }, wait);
      }

      document.addEventListener("touchstart", function (e) {
        if (refreshing || !shouldAllow() || !e.touches || e.touches.length !== 1) {
          armed = false;
          return;
        }
        if (pageScrollTop() > 2 || hasScrolledAncestor(e.target)) {
          armed = false;
          return;
        }
        armed = true;
        pulling = false;
        pullDist = 0;
        sx = e.touches[0].clientX;
        sy = e.touches[0].clientY;
      }, { passive: true });

      document.addEventListener("touchmove", function (e) {
        if (!armed || refreshing || !e.touches || e.touches.length !== 1) return;
        if (pageScrollTop() > 2) {
          armed = false;
          pulling = false;
          resetBar();
          return;
        }
        var dx = e.touches[0].clientX - sx;
        var dy = e.touches[0].clientY - sy;
        if (!pulling) {
          if (Math.abs(dx) > 10 && Math.abs(dx) >= Math.abs(dy) * 0.9) {
            armed = false;
            return;
          }
          if (dy <= 10) return;
          pulling = true;
        }
        if (dy < 0) dy = 0;
        pullDist = Math.min(92, dy * 0.48);
        if (pullDist > 6 && e.cancelable) {
          e.preventDefault();
        }
        bar.style.transition = "none";
        bar.style.opacity = String(Math.min(1, pullDist / 28));
        bar.style.transform = "translate(-50%, " + Math.round(-20 + pullDist * 0.55) + "px)";
        if (pullDist >= threshold) {
          bar.classList.add("ready");
          bar.textContent = "↻ 释放立即刷新";
        } else {
          bar.classList.remove("ready");
          bar.textContent = "↓ 下拉刷新";
        }
      }, { passive: false });

      function endTouch() {
        if (!armed) return;
        var shouldGo = pulling && pullDist >= threshold;
        armed = false;
        pulling = false;
        if (shouldGo) triggerRefresh();
        else resetBar();
      }
      document.addEventListener("touchend", endTouch, { passive: true });
      document.addEventListener("touchcancel", endTouch, { passive: true });

      // Back-to-foreground auto silent refresh (for both desktop browsers and mobile)
      function maybeAutoSync() {
        if (document.visibilityState === "hidden" || refreshing || !shouldAllow()) return;
        var now = Date.now();
        if (now - lastAutoTs < (opts.autoSyncIntervalMs || 25000)) return;
        lastAutoTs = now;
        try { opts.onRefresh(true); } catch (e) {}
      }
      document.addEventListener("visibilitychange", maybeAutoSync);
      w.addEventListener("focus", maybeAutoSync);
    }
  };
})(window);
