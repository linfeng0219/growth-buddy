(function (global) {
  var LEVELS = [
    { xp: 0, name: "一颗蛋", talk: "抱抱我，完成一件小事我就会慢慢破壳哦。" },
    { xp: 10, name: "破壳啦", talk: "哇，外面的世界好暖和！快带我去看看吧。" },
    { xp: 30, name: "幼年体", talk: "我已经长出小耳朵啦！今天你完成了什么呀？" },
    { xp: 60, name: "少年体", talk: "听说图鉴里有好多好朋友，带一位出来陪我玩吧！" },
    { xp: 120, name: "成年体", talk: "你好呀老朋友！神秘蛋也解锁啦，一起去看看惊喜吧。" },
    { xp: 200, name: "守护兽", talk: "守护兽达成！我们一起记录每一天的成长闪光吧！" }
  ];

  var AVATARS = [
    { shirt: "#5b8def", hair: "#4a3227", skin: "#f2c9a0", style: "short" },
    { shirt: "#e86a6a", hair: "#2e2a26", skin: "#f0c39a", style: "bun" },
    { shirt: "#58b368", hair: "#7a5230", skin: "#f4cda6", style: "short" },
    { shirt: "#9a6fca", hair: "#242b4d", skin: "#f2c9a0", style: "bob" },
    { shirt: "#e8913a", hair: "#5a3a1e", skin: "#f7d3ad", style: "short" },
    { shirt: "#4a9f9f", hair: "#c9a344", skin: "#f2c9a0", style: "bob" },
    { shirt: "#d9668f", hair: "#3b2a20", skin: "#f4cda6", style: "long" },
    { shirt: "#3f6ea8", hair: "#2b2420", skin: "#eec29a", style: "glasses" },
    { shirt: "#6c8f4e", hair: "#1f1b18", skin: "#e9b98f", style: "beard" },
    { shirt: "#c58b3d", hair: "#9a948c", skin: "#f2c9a0", style: "bun" },
    { shirt: "#5aa4d6", hair: "#3a2a1c", skin: "#f2c9a0", style: "cap" },
    { shirt: "#b57edc", hair: "#e0a03a", skin: "#f7d3ad", style: "pigtails" }
  ];

  var uidSeq = 1;
  function nextUid() { return "g" + (uidSeq++); }
  function esc(s) {
    return String(s || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }
  function levelForXp(xp) {
    var lv = 0;
    for (var i = 0; i < LEVELS.length; i++) {
      if (xp >= LEVELS[i].xp) lv = i;
    }
    return lv;
  }

  function getSpec() {
    if (global.__GB_PET_SPEC) return global.__GB_PET_SPEC;
    if (typeof XMLHttpRequest !== "undefined") {
      try {
        var xhr = new XMLHttpRequest();
        xhr.open("GET", "/shared/pet-spec.json", false);
        xhr.send(null);
        if (xhr.status >= 200 && xhr.status < 300 && xhr.responseText) {
          global.__GB_PET_SPEC = JSON.parse(xhr.responseText);
          return global.__GB_PET_SPEC;
        }
      } catch (e) {}
    }
    throw new Error("pet-spec.json not loaded");
  }

  function gradSVG(id, gDef, ctx) {
    var s = '<radialGradient id="' + id + '" cx="' + Math.round(gDef.cx * 100) + '%" cy="' + Math.round(gDef.cy * 100) + '%" r="' + Math.round(gDef.r * 100) + '%">';
    for (var i = 0; i < gDef.stops.length; i++) {
      var st = gDef.stops[i];
      var col = resolveTokenColor(st[1], ctx);
      var op = st[2] != null && st[2] < 1 ? ' stop-opacity="' + st[2] + '"' : "";
      s += '<stop offset="' + Math.round(st[0] * 100) + '%" stop-color="' + col + '"' + op + "/>";
    }
    return s + "</radialGradient>";
  }

  function resolveTokenColor(val, ctx) {
    if (!val) return "";
    if (val.charAt(0) !== "@") return val;
    if (ctx.sil && (val === "@body" || val === "@acc" || val.indexOf("@bodyTint") === 0)) {
      return "#bfb8ac";
    }
    return ctx.tokens[val] || val;
  }

  function resolvePaint(val, ctx) {
    if (!val) return "none";
    if (val.indexOf("grad:") === 0) {
      var gName = val.slice(5);
      ctx.usedGrads[gName] = true;
      return "url(#" + gName + "_" + ctx.uid + ")";
    }
    return resolveTokenColor(val, ctx);
  }

  function collectGradsInShapes(list, out) {
    if (!list) return;
    for (var i = 0; i < list.length; i++) {
      var sh = list[i];
      if (!sh) continue;
      if (typeof sh.fill === "string" && sh.fill.indexOf("grad:") === 0) out[sh.fill.slice(5)] = true;
      if (typeof sh.stroke === "string" && sh.stroke.indexOf("grad:") === 0) out[sh.stroke.slice(5)] = true;
      if (sh.type === "eyes") out.eye = true;
      if (sh.children) collectGradsInShapes(sh.children, out);
    }
  }

  function renderShape(sh, ctx) {
    if (!sh) return "";
    if (sh.when === "awake" && ctx.sleep) return "";
    if (sh.when === "normal" && ctx.sil) return "";
    if (sh.when === "sil" && !ctx.sil) return "";

    if (sh.type === "group") {
      var attrs = "";
      if (sh.anim === "breathe" && !ctx.sil) attrs += ' class="breathe"';
      else if (sh.anim === "egg" && !ctx.sil) attrs += ' class="egg-wobble"';
      if (sh.transform) attrs += ' transform="' + sh.transform + '"';
      if (sh.clipId) attrs += ' clip-path="url(#' + sh.clipId + ')"';
      return "<g" + attrs + ">" + renderShapes(sh.children, ctx) + "</g>";
    }
    if (sh.type === "eyes") {
      ctx.usedGrads.eye = true;
      return eyesSVG(sh.y, sh.x1, sh.x2, ctx.sleep, ctx.uid);
    }
    if (sh.type === "text") {
      return '<text x="' + sh.x + '" y="' + sh.y + '" text-anchor="middle" font-size="' + (sh.size || 38) + '" fill="' + resolvePaint(sh.fill, ctx) + '" font-family="Georgia,serif">' + esc(sh.text || "") + "</text>";
    }

    var fillVal = (ctx.sil && sh.silFill) ? sh.silFill : sh.fill;
    var fill = fillVal ? resolvePaint(fillVal, ctx) : "none";
    var common = ' fill="' + fill + '"';
    if (sh.stroke) {
      common += ' stroke="' + resolvePaint(sh.stroke, ctx) + '" stroke-width="' + (sh.sw || 2) + '"';
      if (sh.cap) common += ' stroke-linecap="' + sh.cap + '"';
    }
    if (sh.opacity != null && sh.opacity < 1) {
      common += ' opacity="' + sh.opacity + '"';
    }
    if (sh.rot && sh.rot.length === 3) {
      common += ' transform="rotate(' + sh.rot[0] + " " + sh.rot[1] + " " + sh.rot[2] + ')"';
    }

    if (sh.type === "ellipse") {
      return '<ellipse cx="' + sh.cx + '" cy="' + sh.cy + '" rx="' + sh.rx + '" ry="' + sh.ry + '"' + common + "/>";
    }
    if (sh.type === "circle") {
      return '<circle cx="' + sh.cx + '" cy="' + sh.cy + '" r="' + sh.r + '"' + common + "/>";
    }
    if (sh.type === "rect") {
      return '<rect x="' + sh.x + '" y="' + sh.y + '" width="' + sh.w + '" height="' + sh.h + '"' + (sh.rx ? ' rx="' + sh.rx + '"' : "") + common + "/>";
    }
    if (sh.type === "path") {
      return '<path d="' + sh.d + '"' + common + "/>";
    }
    return "";
  }

  function renderShapes(list, ctx) {
    if (!list) return "";
    var out = "";
    for (var i = 0; i < list.length; i++) out += renderShape(list[i], ctx);
    return out;
  }

  function buildDefs(ctx, extraClips) {
    var spec = getSpec();
    var grads = spec.gradients || {};
    var defs = "";
    for (var gName in ctx.usedGrads) {
      if (ctx.usedGrads.hasOwnProperty(gName) && grads[gName]) {
        defs += gradSVG(gName + "_" + ctx.uid, grads[gName], ctx);
      }
    }
    if (extraClips) defs += extraClips;
    return defs ? "<defs>" + defs + "</defs>" : "";
  }

  function pedestalSVG(base) {
    var spec = getSpec().pedestals;
    var pal = (spec.palettes && spec.palettes[base || "grass"]) || spec.palettes.grass;
    var ctx = {
      uid: nextUid(), sleep: false, sil: false, usedGrads: {},
      tokens: { "@pedFill": pal.fill, "@pedStroke": pal.stroke, "@pedDot": pal.dot }
    };
    return renderShapes(spec.shapes, ctx);
  }

  function eyesSVG(y, x1, x2, sleep, uid) {
    if (sleep) {
      return '<path d="M' + (x1 - 7) + " " + y + ' q7 6 14 0" stroke="#3a2f22" stroke-width="2.6" fill="none" stroke-linecap="round"/>'
        + '<path d="M' + (x2 - 7) + " " + y + ' q7 6 14 0" stroke="#3a2f22" stroke-width="2.6" fill="none" stroke-linecap="round"/>';
    }
    return '<g class="eyeG">'
      + '<ellipse cx="' + x1 + '" cy="' + y + '" rx="8.2" ry="10.2" fill="url(#eye_' + uid + ')"/>'
      + '<ellipse cx="' + x2 + '" cy="' + y + '" rx="8.2" ry="10.2" fill="url(#eye_' + uid + ')"/>'
      + '<g class="pupil"><circle cx="' + (x1 - 3.4) + '" cy="' + (y - 4.2) + '" r="3.1" fill="#fff"/><circle cx="' + (x1 + 3.4) + '" cy="' + (y + 4.2) + '" r="1.5" fill="#fff" opacity=".88"/></g>'
      + '<g class="pupil"><circle cx="' + (x2 - 3.4) + '" cy="' + (y - 4.2) + '" r="3.1" fill="#fff"/><circle cx="' + (x2 + 3.4) + '" cy="' + (y + 4.2) + '" r="1.5" fill="#fff" opacity=".88"/></g>'
      + "</g>";
  }

  function mainPetSVG(lv, o) {
    o = o || {};
    var spec = getSpec();
    var vb = spec.viewBox;
    var st = (spec.mainPetStages && spec.mainPetStages[String(lv)]) || spec.mainPetStages["5"];
    var uid = nextUid(), sleep = !!o.sleep, wAttr = o.w ? ' width="' + o.w + '"' : "";
    var ctx = { uid: uid, sleep: sleep, sil: false, usedGrads: {}, tokens: {} };
    collectGradsInShapes(st.shapes, ctx.usedGrads);
    var defs = buildDefs(ctx, "");
    var s = '<svg viewBox="0 0 ' + vb.w + " " + vb.h + '"' + wAttr + ' role="img" aria-label="主宠">' + defs;
    if (!o.noBase) s += pedestalSVG(st.base || "grass");
    if (st.halo) {
      s += renderShape({ type: "circle", cx: st.halo.cx, cy: st.halo.cy, r: st.halo.r, fill: st.halo.fill, opacity: st.halo.opacity }, ctx);
    }
    s += renderShape({ type: "group", anim: st.groupAnim || "breathe", children: st.shapes }, ctx);
    return s + "</svg>";
  }

  function composeSpeciesShapes(sp, tpl, sil) {
    var ear = sp.ear || "round", acc = sp.acc || "none", tail = sp.tail || "none", mouth = sp.mouth || "smile";
    var list = [];
    Array.prototype.push.apply(list, (tpl.tails && tpl.tails[tail]) || []);
    var inner = [];
    Array.prototype.push.apply(inner, (tpl.ears && tpl.ears[ear]) || (tpl.ears && tpl.ears.round) || []);
    if (sp.ahoge) Array.prototype.push.apply(inner, tpl.ahoge || []);
    Array.prototype.push.apply(inner, tpl.body || []);
    if (!sil) {
      Array.prototype.push.apply(inner, tpl.face || []);
      Array.prototype.push.apply(inner, (tpl.mouths && tpl.mouths[mouth]) || (tpl.mouths && tpl.mouths.smile) || []);
    } else {
      Array.prototype.push.apply(inner, tpl.silhouetteMark || []);
    }
    Array.prototype.push.apply(inner, (tpl.accessories && tpl.accessories[acc]) || []);
    list.push({ type: "group", anim: sil ? "" : "breathe", children: inner });
    if (sp._stageTransform) {
      return [{ type: "group", transform: sp._stageTransform, children: list }];
    }
    return list;
  }

  function speciesSVG(sp, o) {
    o = o || {};
    var sil = !!o.silo;
    if (!sil && sp && sp.id === "b0") {
      return mainPetSVG(o.lv != null ? Number(o.lv) : 3, o);
    }
    var spec = getSpec(), tpl = spec.speciesTemplate, vb = spec.viewBox;
    var acc = sp.acc || "none", base = sp.base || "grass";
    var bodyC = sil ? "#bfb8ac" : (sp.color || "#ec9844");
    var accC = sil ? "#bfb8ac" : (acc === "scarf" ? "#c95742" : (acc === "bow" ? "#e2758e" : "#f2c44d"));
    var uid = nextUid(), wAttr = o.w ? ' width="' + o.w + '"' : "";
    var ctx = {
      uid: uid, sleep: !!o.sleep, sil: sil, usedGrads: {},
      tokens: { "@body": bodyC, "@acc": accC }
    };
    var shapes = composeSpeciesShapes(sp, tpl, sil);
    collectGradsInShapes(shapes, ctx.usedGrads);
    var defs = sil ? "" : buildDefs(ctx, "");
    var s = '<svg viewBox="0 0 ' + vb.w + " " + vb.h + '"' + wAttr + ' role="img" aria-label="' + esc(sp.name || "伙伴") + '">' + defs;
    if (!o.noBase && !sil) s += pedestalSVG(base);
    if (o._halo) s += renderShape({ type: "circle", cx: o._halo.cx, cy: o._halo.cy, r: o._halo.r, fill: o._halo.fill, opacity: o._halo.opacity }, ctx);
    s += renderShapes(shapes, ctx);
    return s + "</svg>";
  }

  function avatarSVG(i, o) {
    o = o || {};
    var a = AVATARS[i] || AVATARS[0];
    var s = '<svg viewBox="0 0 60 60"' + (o.w ? ' width="' + o.w + '"' : "") + ">";
    if (a.style === "bun") s += '<circle cx="17" cy="14" r="6" fill="' + a.hair + '"/><circle cx="43" cy="14" r="6" fill="' + a.hair + '"/>';
    if (a.style === "bob") s += '<path d="M15 24 q0 14 6 16 h18 q6 -2 6 -16 z" fill="' + a.hair + '"/>';
    if (a.style === "long") s += '<path d="M14 23 q0 22 5 24 h22 q5 -2 5 -24 z" fill="' + a.hair + '"/>';
    if (a.style === "pigtails") s += '<circle cx="12" cy="24" r="6" fill="' + a.hair + '"/><circle cx="48" cy="24" r="6" fill="' + a.hair + '"/><ellipse cx="11" cy="31" rx="4" ry="7" fill="' + a.hair + '"/><ellipse cx="49" cy="31" rx="4" ry="7" fill="' + a.hair + '"/>';
    s += '<ellipse cx="30" cy="52" rx="15" ry="13" fill="' + a.shirt + '"/><path d="M23 41 q7 5 14 0" stroke="#fff" stroke-width="2" fill="none" stroke-linecap="round" opacity=".75"/><circle cx="30" cy="23" r="13" fill="' + a.skin + '"/>';
    if (a.style === "cap") {
      s += '<path d="M16 20 a14 14 0 0 1 28 0 z" fill="#e8913a"/><path d="M16 20 q16 -3 31 2 l-3 3 q-14 -4 -28 -2 z" fill="#cf7924"/>';
    } else {
      s += '<path d="M17 21 a13 13 0 0 1 26 0 q-4 -6 -13 -6 t-13 6 z" fill="' + a.hair + '"/>';
    }
    s += '<circle cx="25.5" cy="23" r="1.8" fill="#3a2f22"/><circle cx="34.5" cy="23" r="1.8" fill="#3a2f22"/><circle cx="26" cy="22.4" r=".6" fill="#fff"/><circle cx="35" cy="22.4" r=".6" fill="#fff"/>';
    if (a.style === "glasses") {
      s += '<circle cx="25.5" cy="23" r="4.1" fill="none" stroke="#3a2f22" stroke-width="1.3"/><circle cx="34.5" cy="23" r="4.1" fill="none" stroke="#3a2f22" stroke-width="1.3"/><path d="M29.6 23 h0.8" stroke="#3a2f22" stroke-width="1.3"/>';
    }
    s += '<path d="M26 29 q4 3 8 0" stroke="#3a2f22" stroke-width="1.5" fill="none" stroke-linecap="round"/>';
    if (a.style === "beard") {
      s += '<path d="M19 25 q0 11 11 11 t11 -11 q-3 6 -11 6 t-11 -6 z" fill="' + a.hair + '" opacity=".85"/>';
    }
    s += '<ellipse cx="20" cy="27" rx="2.7" ry="1.8" fill="#e87968" opacity=".4"/><ellipse cx="40" cy="27" rx="2.7" ry="1.8" fill="#e87968" opacity=".4"/></svg>';
    return s;
  }

  function avatarHtml(idx, url, size, extraClass) {
    var sz = size || 32;
    var avIdx = idx || 0;
    var clsName = extraClass ? ("gb-av " + extraClass) : "gb-av";
    var cls = ' class="' + esc(clsName) + '"';
    var attrs = ' data-gb-av-idx="' + avIdx + '" data-gb-av-url="' + esc(url || "") + '"';
    if (url) {
      return '<img src="' + esc(url) + '"' + cls + attrs + ' alt="头像" style="width:' + sz + 'px;height:' + sz + 'px;border-radius:50%;object-fit:cover;display:inline-block;vertical-align:middle;border:1.5px solid #e4d8c4;background:#fff">';
    }
    return '<span' + cls + attrs + ' style="width:' + sz + 'px;height:' + sz + 'px;border-radius:50%;overflow:hidden;display:inline-flex;align-items:center;justify-content:center;vertical-align:middle;background:#fdf8ef;border:1.5px solid #e4d8c4">' + avatarSVG(avIdx, { w: sz }) + '</span>';
  }

  function compressAvatarFile(file, cb) {
    if (!file) return;
    var reader = new FileReader();
    reader.onload = function (ev) {
      var img = new Image();
      img.onload = function () {
        var sw = img.width || 256, sh = img.height || 256;
        var side = Math.min(sw, sh);
        var sx = Math.floor((sw - side) / 2), sy = Math.floor((sh - side) / 2);
        var c = document.createElement("canvas");
        c.width = 256; c.height = 256;
        var g = c.getContext("2d");
        g.fillStyle = "#ffffff";
        g.fillRect(0, 0, 256, 256);
        g.drawImage(img, sx, sy, side, side, 0, 0, 256, 256);
        cb(c.toDataURL("image/jpeg", 0.85));
      };
      img.src = ev.target.result;
    };
    reader.readAsDataURL(file);
  }

  function enableEyeTracking() {
    if (global.__gbEyeInit) return;
    global.__gbEyeInit = true;
    document.addEventListener("pointermove", function (e) {
      var pupils = document.querySelectorAll(".pupil");
      for (var i = 0; i < pupils.length; i++) {
        var g = pupils[i], r = g.getBoundingClientRect();
        if (!r.width && !r.height) continue;
        var cx = r.left + r.width / 2, cy = r.top + r.height / 2;
        var dx = e.clientX - cx, dy = e.clientY - cy;
        var d = Math.sqrt(dx * dx + dy * dy) || 1;
        var k = Math.min(3.1, d / 48);
        g.setAttribute("transform", "translate(" + (dx / d * k).toFixed(2) + " " + (dy / d * k).toFixed(2) + ")");
      }
    }, { passive: true });
  }

  function tintHex(hex, whiteRatio) {
    var h = String(hex || "#ec9844").replace("#", "");
    if (h.length === 3) h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2];
    if (h.length !== 6) return "#fff1da";
    var r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16);
    if (isNaN(r) || isNaN(g) || isNaN(b)) return "#fff1da";
    var w = Math.max(0, Math.min(1, whiteRatio || 0));
    return "rgb(" + Math.round(r * (1 - w) + 255 * w) + "," + Math.round(g * (1 - w) + 255 * w) + "," + Math.round(b * (1 - w) + 255 * w) + ")";
  }

  function petStageSVG(sp, lv, o) {
    o = o || {};
    lv = Number(lv) || 0;
    if (!sp || sp.id === "b0") return mainPetSVG(lv, o);
    var spec = getSpec(), tpl = spec.speciesTemplate, vb = spec.viewBox;
    var uid = nextUid(), wAttr = o.w ? ' width="' + o.w + '"' : "";
    var base = sp.base || "grass";
    var bodyC = sp.color || "#ec9844";
    var tokens = {
      "@body": bodyC,
      "@bodyTint72": tintHex(bodyC, 0.72),
      "@bodyTint35": tintHex(bodyC, 0.35),
      "@bodyTint12": tintHex(bodyC, 0.12)
    };
    var ctx = { uid: uid, sleep: !!o.sleep, sil: false, usedGrads: {}, tokens: tokens };

    if (lv <= 0) {
      var st0 = tpl.stage0Shapes || [];
      collectGradsInShapes(st0, ctx.usedGrads);
      var s0 = '<svg viewBox="0 0 ' + vb.w + " " + vb.h + '"' + wAttr + ' role="img" aria-label="' + esc(sp.name || "宠物蛋") + '">' + buildDefs(ctx, "");
      if (!o.noBase) s0 += pedestalSVG(base);
      s0 += renderShape({ type: "group", anim: "egg", children: st0 }, ctx);
      return s0 + "</svg>";
    }

    if (lv === 1) {
      var s1t = tpl.stage1;
      var ear = sp.ear || "round", mouth = sp.mouth || "smile";
      var inner1 = [];
      Array.prototype.push.apply(inner1, (s1t.ears && s1t.ears[ear]) || []);
      if (sp.ahoge) Array.prototype.push.apply(inner1, s1t.ahoge || []);
      Array.prototype.push.apply(inner1, s1t.head || []);
      Array.prototype.push.apply(inner1, (s1t.mouths && s1t.mouths[mouth]) || (s1t.mouths && s1t.mouths.smile) || []);
      var clipId = "clip1_" + uid;
      var outer1 = [
        { type: "group", clipId: clipId, children: inner1 }
      ];
      Array.prototype.push.apply(outer1, s1t.shell || []);
      collectGradsInShapes(outer1, ctx.usedGrads);
      var cRect = s1t.clip || [40, 20, 140, 128];
      var clipXml = '<clipPath id="' + clipId + '"><rect x="' + cRect[0] + '" y="' + cRect[1] + '" width="' + cRect[2] + '" height="' + cRect[3] + '"/></clipPath>';
      var s1 = '<svg viewBox="0 0 ' + vb.w + " " + vb.h + '"' + wAttr + ' role="img" aria-label="' + esc(sp.name || "破壳中") + '">' + buildDefs(ctx, clipXml);
      if (!o.noBase) s1 += pedestalSVG(base);
      s1 += renderShape({ type: "group", anim: "breathe", children: outer1 }, ctx);
      return s1 + "</svg>";
    }

    var rule = (tpl.stageRules && tpl.stageRules[String(lv >= 5 ? 5 : lv)]) || { scale: 1.0, anchorX: 110, anchorY: 174 };
    var spDraw = sp;
    if (rule.overrideAcc || rule.wingToTail) {
      spDraw = Object.assign({}, sp, {
        acc: rule.overrideAcc || sp.acc,
        tail: (rule.wingToTail && sp.tail === "wing") ? rule.wingToTail : sp.tail
      });
    } else if (rule.crownIfNoAcc && (!sp.acc || sp.acc === "none")) {
      spDraw = Object.assign({}, sp, { acc: "crown" });
    }
    if (lv >= 4 && base === "grass") base = "sand";
    var halo = tpl.haloByLv && tpl.haloByLv[String(lv >= 5 ? 5 : lv)];
    var sc = rule.scale != null ? rule.scale : 1;
    var ax = rule.anchorX != null ? rule.anchorX : 110;
    var ay = rule.anchorY != null ? rule.anchorY : 174;
    spDraw = Object.assign({}, spDraw, {
      base: base,
      _stageTransform: "translate(" + ax + " " + ay + ") scale(" + sc + ") translate(" + (-ax) + " " + (-ay) + ")"
    });
    return speciesSVG(spDraw, Object.assign({}, o, { _halo: halo || null }));
  }

  global.GBArt = {
    LEVELS: LEVELS,
    AVATARS: AVATARS,
    esc: esc,
    levelForXp: levelForXp,
    mainPetSVG: mainPetSVG,
    petStageSVG: petStageSVG,
    speciesSVG: speciesSVG,
    avatarSVG: avatarSVG,
    avatarHtml: avatarHtml,
    compressAvatarFile: compressAvatarFile,
    enableEyeTracking: enableEyeTracking
  };
})(window);
