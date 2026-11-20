(function () {
  var cachedList = null;
  var modalEl = null;

  function ensureModal() {
    if (modalEl) return modalEl;
    modalEl = document.createElement("div");
    modalEl.className = "modal-backdrop";
    modalEl.id = "gbChangelogModal";
    modalEl.innerHTML =
      '<div class="modal-card" style="text-align:left;max-width:480px">' +
      '  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">' +
      '    <h3 id="gbClTitle" style="font-family:var(--serif);margin:0">版本更新内容</h3>' +
      '    <button type="button" class="ghost sm" id="gbClClose">关闭</button>' +
      '  </div>' +
      '  <div id="gbClBody" style="max-height:360px;overflow-y:auto;font-size:13px;line-height:1.7;color:var(--ink-soft)">加载中...</div>' +
      '</div>';
    document.body.appendChild(modalEl);
    modalEl.querySelector("#gbClClose").addEventListener("click", function () {
      modalEl.classList.remove("show");
    });
    modalEl.addEventListener("click", function (e) {
      if (e.target === modalEl) modalEl.classList.remove("show");
    });
    return modalEl;
  }

  function esc(s) {
    return String(s || "").replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  async function openChangelogModal(verText) {
    var m = ensureModal();
    m.querySelector("#gbClTitle").textContent = "更新内容" + (verText ? " · " + verText : "");
    var body = m.querySelector("#gbClBody");
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
    if (!cachedList.length) {
      body.innerHTML = '<div style="color:var(--muted)">暂无更新记录。</div>';
      return;
    }
    body.innerHTML = cachedList
      .map(function (entry) {
        var items = (entry.items || [])
          .map(function (it) {
            return '<li style="margin:4px 0">' + esc(it) + "</li>";
          })
          .join("");
        return (
          '<div style="margin-bottom:14px;padding-bottom:10px;border-bottom:1px dashed var(--hair-2)">' +
          '<div style="font-weight:600;color:var(--ink);margin-bottom:6px">📅 ' +
          esc(entry.date) +
          (entry.title ? " · " + esc(entry.title) : "") +
          "</div>" +
          '<ul style="padding-left:18px;margin:0">' +
          items +
          "</ul></div>"
        );
      })
      .join("");
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
