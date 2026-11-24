(function () {
  var cachedList = null;
  var modalEl = null;
  var activeScope = "all"; // all | app | server

  function ensureModal() {
    if (modalEl) return modalEl;
    modalEl = document.createElement("div");
    modalEl.className = "modal-backdrop";
    modalEl.id = "gbChangelogModal";
    modalEl.innerHTML =
      '<div class="modal-card" style="text-align:left;max-width:520px">' +
      '  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;gap:8px">' +
      '    <h3 id="gbClTitle" style="font-family:var(--serif);margin:0">版本更新内容</h3>' +
      '    <button type="button" class="ghost sm" id="gbClClose">关闭</button>' +
      '  </div>' +
      '  <div id="gbClTabs" style="display:flex;gap:6px;margin-bottom:10px;flex-wrap:wrap">' +
      '    <button type="button" class="sm on" data-clscope="all">全部更新</button>' +
      '    <button type="button" class="sm" data-clscope="app">📱 App 端</button>' +
      '    <button type="button" class="sm" data-clscope="server">🖥️ 后台与服务端</button>' +
      '  </div>' +
      '  <div id="gbClBody" style="max-height:380px;overflow-y:auto;font-size:13px;line-height:1.7;color:var(--ink-soft)">加载中...</div>' +
      '</div>';
    document.body.appendChild(modalEl);
    modalEl.querySelector("#gbClClose").addEventListener("click", function () {
      modalEl.classList.remove("show");
    });
    modalEl.addEventListener("click", function (e) {
      if (e.target === modalEl) modalEl.classList.remove("show");
    });
    modalEl.querySelectorAll("[data-clscope]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        activeScope = btn.getAttribute("data-clscope") || "all";
        modalEl.querySelectorAll("[data-clscope]").forEach(function (b) {
          var on = b === btn;
          b.classList.toggle("on", on);
          b.style.background = on ? "var(--accent, #2F6B4F)" : "";
          b.style.color = on ? "#fff" : "";
        });
        renderModalBody();
      });
    });
    return modalEl;
  }

  function esc(s) {
    return String(s || "").replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function renderSubBlock(label, bg, border, color, title, items) {
    if (!items || !items.length) return "";
    var lis = items
      .map(function (it) {
        return '<li style="margin:3px 0">' + esc(it) + "</li>";
      })
      .join("");
    return (
      '<div style="background:' + bg + ";border:1px solid " + border + ';border-radius:10px;padding:8px 10px;margin-top:6px">' +
      '  <div style="font-weight:700;font-size:12px;color:' + color + ';margin-bottom:4px">' + label + (title ? " · " + esc(title) : "") + "</div>" +
      '  <ul style="padding-left:18px;margin:0">' + lis + "</ul>" +
      "</div>"
    );
  }

  function renderModalBody() {
    var m = ensureModal();
    var body = m.querySelector("#gbClBody");
    if (!cachedList || !cachedList.length) {
      body.innerHTML = '<div style="color:var(--muted)">暂无更新记录。</div>';
      return;
    }
    var html = cachedList
      .map(function (entry) {
        var hasSplit = (entry.app_items && entry.app_items.length) || (entry.server_items && entry.server_items.length);
        var inner = "";
        if (hasSplit) {
          if (activeScope === "all" || activeScope === "app") {
            inner += renderSubBlock("📱 App 客户端", "#f4fbf7", "#cbe4d4", "#2F6B4F", entry.app_title, entry.app_items);
          }
          if (activeScope === "all" || activeScope === "server") {
            inner += renderSubBlock("🖥️ 后台与服务端", "#fff9ef", "#eed5ae", "#9a5b13", entry.server_title, entry.server_items);
          }
        } else if (activeScope === "all") {
          var items = (entry.items || [])
            .map(function (it) {
              return '<li style="margin:3px 0">' + esc(it) + "</li>";
            })
            .join("");
          inner = '<ul style="padding-left:18px;margin:0">' + items + "</ul>";
        }
        if (!inner) return "";
        return (
          '<div style="margin-bottom:14px;padding-bottom:10px;border-bottom:1px dashed var(--hair-2)">' +
          '<div style="font-weight:700;color:var(--ink);margin-bottom:4px">📅 ' +
          esc(entry.date) +
          (entry.title ? " · " + esc(entry.title) : "") +
          "</div>" +
          inner +
          "</div>"
        );
      })
      .filter(Boolean)
      .join("");
    body.innerHTML = html || '<div style="color:var(--muted)">该分类暂无更新记录。</div>';
  }

  async function openChangelogModal(verText) {
    var m = ensureModal();
    m.querySelector("#gbClTitle").textContent = "更新内容" + (verText ? " · " + verText : "");
    var body = m.querySelector("#gbClBody");
    body.innerHTML = "加载中...";
    m.classList.add("show");
    if (!cachedList) {
      try {
        var r = await fetch("/api/changelog");
        var d = await r.json();
        cachedList = d.changelog || [];
      } catch (e) {
        cachedList = [];
      }
    }
    renderModalBody();
  }

  window.GBChangelog = {
    open: openChangelogModal,
    bindBadge: function (el, prefix) {
      if (!el) return;
      el.style.cursor = "pointer";
      el.title = "点击查看版本更新内容";
      var curVer = "";
      fetch("/api/health")
        .then(function (r) { return r.json(); })
        .then(function (d) {
          if (d && d.version) {
            curVer = d.version;
            el.textContent = (prefix || "") + d.version + " ⓘ";
          }
        })
        .catch(function () {});
      el.addEventListener("click", function () {
        openChangelogModal(curVer);
      });
    }
  };
})();
