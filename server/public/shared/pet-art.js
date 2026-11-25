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

  function pedestalSVG(base) {
    var map = {
      grass: { fill: "#ebe7d4", stroke: "#b6c79a", dot: "#cfe0b4" },
      snow: { fill: "#e8eef3", stroke: "#c3cfd9", dot: "#ffffff" },
      sand: { fill: "#f2e3cc", stroke: "#d4b78c", dot: "#e5cca6" },
      cloud: { fill: "#eef1f8", stroke: "#ccd4e6", dot: "#ffffff" }
    };
    var c = map[base || "grass"] || map.grass;
    return '<ellipse cx="110" cy="176" rx="68" ry="12" fill="' + c.fill + '"/>'
      + '<path d="M56 172 q4 -9 8 0 M162 172 q4 -9 8 0" stroke="' + c.stroke + '" stroke-width="2.2" fill="none" stroke-linecap="round"/>'
      + '<circle cx="48" cy="164" r="2.6" fill="' + c.dot + '"/><circle cx="174" cy="166" r="2.1" fill="' + c.dot + '"/>';
  }

  function eyesSVG(y, x1, x2, sleep, uid) {
    if (sleep) {
      return '<path d="M' + (x1 - 7) + " " + y + ' q7 6 14 0" stroke="#3a2f22" stroke-width="2.6" fill="none" stroke-linecap="round"/>'
        + '<path d="M' + (x2 - 7) + " " + y + ' q7 6 14 0" stroke="#3a2f22" stroke-width="2.6" fill="none" stroke-linecap="round"/>';
    }
    return '<g class="eyeG">'
      + '<ellipse cx="' + x1 + '" cy="' + y + '" rx="8.2" ry="10.2" fill="url(#ge_' + uid + ')"/>'
      + '<ellipse cx="' + x2 + '" cy="' + y + '" rx="8.2" ry="10.2" fill="url(#ge_' + uid + ')"/>'
      + '<g class="pupil"><circle cx="' + (x1 - 3.4) + '" cy="' + (y - 4.2) + '" r="3.1" fill="#fff"/><circle cx="' + (x1 + 3.4) + '" cy="' + (y + 4.2) + '" r="1.5" fill="#fff" opacity=".88"/></g>'
      + '<g class="pupil"><circle cx="' + (x2 - 3.4) + '" cy="' + (y - 4.2) + '" r="3.1" fill="#fff"/><circle cx="' + (x2 + 3.4) + '" cy="' + (y + 4.2) + '" r="1.5" fill="#fff" opacity=".88"/></g>'
      + "</g>";
  }

  function mainPetSVG(lv, o) {
    o = o || {};
    var uid = nextUid(), sleep = !!o.sleep, wAttr = o.w ? ' width="' + o.w + '"' : "";
    var s = '<svg viewBox="0 0 220 200"' + wAttr + ' role="img" aria-label="主宠">';
    s += '<defs>'
      + '<radialGradient id="mb_' + uid + '" cx="40%" cy="26%" r="82%"><stop offset="0%" stop-color="#ffdca8"/><stop offset="55%" stop-color="#f39d48"/><stop offset="100%" stop-color="#d87526"/></radialGradient>'
      + '<radialGradient id="ml_' + uid + '" cx="40%" cy="24%" r="85%"><stop offset="0%" stop-color="#ffe1ad"/><stop offset="55%" stop-color="#ed8e32"/><stop offset="100%" stop-color="#c45f1c"/></radialGradient>'
      + '<radialGradient id="mbel_' + uid + '" cx="45%" cy="32%" r="80%"><stop offset="0%" stop-color="#fff9ed"/><stop offset="100%" stop-color="#fce0b6"/></radialGradient>'
      + '<radialGradient id="meg_' + uid + '" cx="40%" cy="28%" r="82%"><stop offset="0%" stop-color="#fff8eb"/><stop offset="65%" stop-color="#f3e5cb"/><stop offset="100%" stop-color="#dfd0b2"/></radialGradient>'
      + '<radialGradient id="ge_' + uid + '" cx="38%" cy="30%" r="85%"><stop offset="0%" stop-color="#7c5630"/><stop offset="45%" stop-color="#4a3220"/><stop offset="100%" stop-color="#281b10"/></radialGradient>'
      + '<radialGradient id="mglow_' + uid + '" cx="50%" cy="30%" r="72%"><stop offset="0%" stop-color="#fff3d9" stop-opacity=".85"/><stop offset="100%" stop-color="#fff3d9" stop-opacity="0"/></radialGradient>'
      + "</defs>";
    if (!o.noBase) s += pedestalSVG(lv >= 4 ? "sand" : "grass");
    if (lv >= 3) s += '<circle cx="110" cy="108" r="82" fill="#f3c653" opacity="0.16"/>';
    if (lv === 0) {
      s += '<g class="egg-wobble"><ellipse cx="110" cy="120" rx="44" ry="54" fill="url(#meg_' + uid + ')" stroke="#d7c29e" stroke-width="2"/><ellipse cx="100" cy="96" rx="18" ry="24" fill="#fff" opacity=".45" transform="rotate(-18 100 96)"/><circle cx="94" cy="106" r="4" fill="#e1cda9"/><circle cx="128" cy="134" r="3.2" fill="#e1cda9"/><circle cx="120" cy="94" r="2.4" fill="#e1cda9"/>' + (sleep ? "" : '<path d="M98 118 q4 4 8 0" stroke="#a6906d" stroke-width="1.8" fill="none" stroke-linecap="round" opacity=".6"/>') + "</g>";
    } else if (lv === 1) {
      s += '<g class="breathe"><path d="M68 156 a44 48 0 0 0 84 0 l-14 -12 l-14 10 l-14 -12 l-14 12 l-14 -10 z" fill="url(#meg_' + uid + ')" stroke="#d7c29e" stroke-width="2"/><ellipse cx="110" cy="118" rx="40" ry="38" fill="url(#mb_' + uid + ')"/><ellipse cx="110" cy="102" rx="28" ry="18" fill="url(#mglow_' + uid + ')"/><path d="M110 82 q-7 -13 3 -18 q8 -4 6 5" stroke="#eb9a45" stroke-width="5" fill="none" stroke-linecap="round"/>' + eyesSVG(114, 94, 126, sleep, uid) + '<ellipse cx="78" cy="126" rx="7" ry="4.5" fill="#f28b82" opacity=".48"/><ellipse cx="142" cy="126" rx="7" ry="4.5" fill="#f28b82" opacity=".48"/><path d="M103 128 q7 5 14 0" stroke="#3a2f22" stroke-width="2.2" fill="none" stroke-linecap="round"/><path d="M78 86 a36 32 0 0 1 64 0 l-10 -6 l-11 7 l-11 -8 l-11 8 l-11 -7 z" fill="url(#meg_' + uid + ')" stroke="#d7c29e" stroke-width="2" transform="rotate(-12 110 74)"/></g>';
    } else if (lv === 2) {
      s += '<g class="breathe"><circle cx="72" cy="78" r="15" fill="#eb9540"/><circle cx="74" cy="80" r="7" fill="#fbd2a2"/><circle cx="148" cy="78" r="15" fill="#eb9540"/><circle cx="146" cy="80" r="7" fill="#fbd2a2"/><path d="M110 74 q-8 -14 4 -20 q8 -4 6 5" stroke="#eb9540" stroke-width="5.5" fill="none" stroke-linecap="round"/><ellipse cx="110" cy="124" rx="52" ry="44" fill="url(#mb_' + uid + ')"/><ellipse cx="110" cy="100" rx="38" ry="22" fill="url(#mglow_' + uid + ')"/><ellipse cx="76" cy="162" rx="13" ry="8" fill="#d87526"/><ellipse cx="144" cy="162" rx="13" ry="8" fill="#d87526"/><ellipse cx="110" cy="138" rx="31" ry="23" fill="url(#mbel_' + uid + ')" opacity=".95"/>' + eyesSVG(115, 90, 130, sleep, uid) + '<ellipse cx="70" cy="130" rx="8.5" ry="5.4" fill="#f28b82" opacity=".48"/><ellipse cx="150" cy="130" rx="8.5" ry="5.4" fill="#f28b82" opacity=".48"/><path d="M101 130 q4.5 4.4 9 0 q4.5 4.4 9 0" stroke="#3a2f22" stroke-width="2.3" fill="none" stroke-linecap="round"/></g>';
    } else if (lv === 3) {
      s += '<g class="breathe"><circle cx="158" cy="144" r="14" fill="#f8d29c"/><circle cx="166" cy="136" r="10" fill="#fce4be"/><path d="M62 84 Q56 44 86 58 Q90 72 78 88 Z" fill="#eb9540"/><path d="M68 78 Q65 56 82 64" fill="#fbd2a2"/><path d="M158 84 Q164 44 134 58 Q130 72 142 88 Z" fill="#eb9540"/><path d="M152 78 Q155 56 138 64" fill="#fbd2a2"/><path d="M110 64 q-10 -18 5 -24 q11 -5 8 7" stroke="#eb9540" stroke-width="6" fill="none" stroke-linecap="round"/><ellipse cx="110" cy="118" rx="58" ry="48" fill="url(#mb_' + uid + ')"/><ellipse cx="110" cy="94" rx="44" ry="24" fill="url(#mglow_' + uid + ')"/><ellipse cx="74" cy="162" rx="14" ry="9" fill="#d87526"/><ellipse cx="146" cy="162" rx="14" ry="9" fill="#d87526"/><path d="M68 162 q3 -3 6 0 M142 162 q3 -3 6 0" stroke="#b85f19" stroke-width="1.5" fill="none" stroke-linecap="round"/><ellipse cx="110" cy="134" rx="35" ry="26" fill="url(#mbel_' + uid + ')" opacity=".95"/>' + eyesSVG(108, 88, 132, sleep, uid) + '<ellipse cx="66" cy="126" rx="9.5" ry="6" fill="#f28b82" opacity=".48"/><ellipse cx="154" cy="126" rx="9.5" ry="6" fill="#f28b82" opacity=".48"/><path d="M100 124 q5 4.6 10 0 q5 4.6 10 0" stroke="#3a2f22" stroke-width="2.4" fill="none" stroke-linecap="round"/><path d="M150 78 l2.5 5 5.4 .7 -3.9 3.6 .9 5.3 -4.9 -2.7 -4.9 2.7 .9 -5.3 -3.9 -3.6 5.4 -.7 z" fill="#ffdf8e" opacity=".9"/></g>';
    } else if (lv === 4) {
      s += '<g class="breathe"><path d="M162 142 q26 6 24 -16 q-2 -11 -13 -9" stroke="#d87526" stroke-width="11" fill="none" stroke-linecap="round"/><path d="M60 102 q-18 -40 18 -44 q14 14 6 42 z" fill="#ec9139"/><path d="M64 96 q-12 -24 12 -28" fill="#fbd2a2" opacity=".7"/><path d="M160 102 q18 -40 -18 -44 q-14 14 -6 42 z" fill="#ec9139"/><path d="M156 96 q12 -24 -12 -28" fill="#fbd2a2" opacity=".7"/><path d="M84 54 L76 22 L96 46 Z" fill="#f0c24b"/><path d="M136 54 L144 22 L124 46 Z" fill="#f0c24b"/><ellipse cx="110" cy="116" rx="62" ry="50" fill="url(#mb_' + uid + ')"/><ellipse cx="110" cy="92" rx="48" ry="26" fill="url(#mglow_' + uid + ')"/><ellipse cx="76" cy="162" rx="15" ry="9" fill="#c9651d"/><ellipse cx="144" cy="162" rx="15" ry="9" fill="#c9651d"/><ellipse cx="110" cy="134" rx="38" ry="27" fill="url(#mbel_' + uid + ')" opacity=".95"/>' + eyesSVG(106, 88, 132, sleep, uid) + '<ellipse cx="64" cy="124" rx="10" ry="6.2" fill="#f28b82" opacity=".48"/><ellipse cx="156" cy="124" rx="10" ry="6.2" fill="#f28b82" opacity=".48"/><path d="M100 123 q5 4.6 10 0 q5 4.6 10 0" stroke="#3a2f22" stroke-width="2.4" fill="none" stroke-linecap="round"/><path d="M44 74 l3 6 6.4 .9 -4.6 4.3 1.1 6.2 -5.9 -3.2 -5.9 3.2 1.1 -6.2 -4.6 -4.3 6.4 -.9 z" fill="#ffdf8e"/></g>';
    } else {
      s += '<g class="breathe"><path d="M36 96 q-26 -24 -10 -44 q20 3 26 26 q3 11 -4 19 z" fill="#d97224"/><path d="M38 92 q-17 -16 -7 -32 q13 3 17 19" fill="#ffcf8c" opacity=".7"/><path d="M184 96 q26 -24 10 -44 q-20 3 -26 26 q-3 11 4 19 z" fill="#d97224"/><path d="M182 92 q17 -16 7 -32 q-13 3 -17 19" fill="#ffcf8c" opacity=".7"/><path d="M56 104 q-16 -44 20 -46 q15 15 6 44 z" fill="#e8892e"/><path d="M164 104 q16 -44 -20 -46 q-15 15 -6 44 z" fill="#e8892e"/><path d="M90 52 L100 28 L110 46 L120 28 L130 52 Z" fill="#f4c74e" stroke="#e2a929" stroke-width="1.5"/><ellipse cx="110" cy="114" rx="64" ry="52" fill="url(#ml_' + uid + ')"/><ellipse cx="110" cy="90" rx="50" ry="26" fill="url(#mglow_' + uid + ')"/><ellipse cx="74" cy="162" rx="15" ry="9.5" fill="#b85617"/><ellipse cx="146" cy="162" rx="15" ry="9.5" fill="#b85617"/><ellipse cx="110" cy="134" rx="39" ry="27" fill="url(#mbel_' + uid + ')" opacity=".95"/>' + eyesSVG(104, 88, 132, sleep, uid) + '<ellipse cx="64" cy="122" rx="10" ry="6.2" fill="#f28b82" opacity=".48"/><ellipse cx="156" cy="122" rx="10" ry="6.2" fill="#f28b82" opacity=".48"/><path d="M100 121 q5 4.8 10 0 q5 4.8 10 0" stroke="#3a2f22" stroke-width="2.5" fill="none" stroke-linecap="round"/><path d="M36 52 l4 8 8.6 1.2 -6.2 5.8 1.5 8.4 -7.9 -4.3 -7.9 4.3 1.5 -8.4 -6.2 -5.8 8.6 -1.2 z" fill="#f4c74e"/><path d="M182 40 l3 6 6.4 .9 -4.6 4.3 1.1 6.2 -5.9 -3.2 -5.9 3.2 1.1 -6.2 -4.6 -4.3 6.4 -.9 z" fill="#f4c74e"/></g>';
    }
    return s + "</svg>";
  }

  function speciesSVG(sp, o) {
    o = o || {};
    var sil = !!o.silo, uid = nextUid();
    var ear = sp.ear || "round", acc = sp.acc || "none", tail = sp.tail || "none", mouth = sp.mouth || "smile", base = sp.base || "grass";
    var bodyC = sil ? "#bfb8ac" : sp.color;
    var accC = sil ? "#bfb8ac" : (acc === "scarf" ? "#c95742" : (acc === "bow" ? "#e2758e" : "#f2c44d"));
    var wAttr = o.w ? ' width="' + o.w + '"' : "";
    var s = '<svg viewBox="0 0 220 200"' + wAttr + ' role="img" aria-label="' + esc(sp.name || "伙伴") + '">';
    if (!sil) {
      s += '<defs><radialGradient id="gb_' + uid + '" cx="40%" cy="24%" r="78%"><stop offset="0%" stop-color="#ffffff" stop-opacity=".55"/><stop offset="100%" stop-color="#ffffff" stop-opacity="0"/></radialGradient><radialGradient id="gs_' + uid + '" cx="50%" cy="92%" r="70%"><stop offset="0%" stop-color="#000000" stop-opacity=".16"/><stop offset="100%" stop-color="#000000" stop-opacity="0"/></radialGradient><radialGradient id="ge_' + uid + '" cx="38%" cy="30%" r="85%"><stop offset="0%" stop-color="#7a5530"/><stop offset="45%" stop-color="#4a3220"/><stop offset="100%" stop-color="#2b1e12"/></radialGradient></defs>';
    }
    if (!o.noBase && !sil) s += pedestalSVG(base);
    if (tail === "puff") {
      s += '<circle cx="164" cy="142" r="13" fill="' + bodyC + '"/><circle cx="172" cy="134" r="8" fill="' + bodyC + '"/>' + (!sil ? '<circle cx="164" cy="142" r="13" fill="url(#gb_' + uid + ')"/>' : "");
    } else if (tail === "fox") {
      s += '<path d="M150 144 q38 6 34 -28 q-12 -6 -24 8 z" fill="' + bodyC + '"/>' + (!sil ? '<path d="M168 122 q12 -8 16 -6 q-2 10 -12 16 z" fill="#fff" opacity=".6"/>' : "");
    } else if (tail === "dragon") {
      s += '<path d="M152 142 q30 8 28 -16 q-2 -11 -12 -9" stroke="' + bodyC + '" stroke-width="10" fill="none" stroke-linecap="round"/><path d="M170 118 l14 -10 l-4 15 z" fill="' + (sil ? "#bfb8ac" : "#f0a944") + '"/>';
    } else if (tail === "wing") {
      s += '<path d="M44 108 q-24 -22 -10 -40 q18 4 23 24 q2 10 -4 17 z" fill="' + bodyC + '"/><path d="M176 108 q24 -22 10 -40 q-18 4 -23 24 q-2 10 4 17 z" fill="' + bodyC + '"/>' + (!sil ? '<path d="M44 104 q-16 -16 -7 -30 q12 3 16 17 z" fill="#fff" opacity=".32"/><path d="M176 104 q16 -16 7 -30 q-12 3 -16 17 z" fill="#fff" opacity=".32"/>' : "");
    }
    s += "<g" + (sil ? "" : ' class="breathe"') + ">";
    if (ear === "round") {
      s += '<circle cx="72" cy="68" r="16" fill="' + bodyC + '"/><circle cx="148" cy="68" r="16" fill="' + bodyC + '"/>';
      if (!sil) s += '<circle cx="72" cy="68" r="15" fill="url(#gb_' + uid + ')"/><circle cx="148" cy="68" r="15" fill="url(#gb_' + uid + ')"/><circle cx="74" cy="70" r="7.5" fill="#fff" opacity=".35"/><circle cx="146" cy="70" r="7.5" fill="#fff" opacity=".35"/>';
    } else if (ear === "tall") {
      s += '<g transform="rotate(-7 82 48)"><ellipse cx="82" cy="48" rx="11" ry="30" fill="' + bodyC + '"/>' + (!sil ? '<ellipse cx="82" cy="51" rx="5.2" ry="20" fill="#f7c2cf" opacity=".75"/>' : "") + '</g><g transform="rotate(7 138 48)"><ellipse cx="138" cy="48" rx="11" ry="30" fill="' + bodyC + '"/>' + (!sil ? '<ellipse cx="138" cy="51" rx="5.2" ry="20" fill="#f7c2cf" opacity=".75"/>' : "") + "</g>";
    } else if (ear === "pointy") {
      s += '<path d="M62 78 L78 24 L98 68 Z" fill="' + bodyC + '"/><path d="M158 78 L142 24 L122 68 Z" fill="' + bodyC + '"/>';
      if (!sil) s += '<path d="M67 70 L78 34 L91 64 Z" fill="#fff" opacity=".28"/><path d="M153 70 L142 34 L129 64 Z" fill="#fff" opacity=".28"/>';
    } else if (ear === "horn") {
      s += '<path d="M74 68 L64 30 L88 56 Z" fill="' + (sil ? "#bfb8ac" : "#f0c24b") + '"/><path d="M146 68 L156 30 L132 56 Z" fill="' + (sil ? "#bfb8ac" : "#f0c24b") + '"/><path d="M58 88 L46 68 L70 76 Z" fill="' + bodyC + '"/><path d="M162 88 L174 68 L150 76 Z" fill="' + bodyC + '"/>';
    } else if (ear === "unicorn") {
      s += '<path d="M104 54 L110 14 L116 54 Z" fill="' + (sil ? "#bfb8ac" : "#f4c74e") + '"/>';
      if (!sil) s += '<path d="M107 40 L114 34 M106 30 L113 24" stroke="#fff" stroke-width="1.6" stroke-linecap="round" opacity=".7"/>';
    }
    if (sp.ahoge && !sil) s += '<path d="M110 66 q-9 -16 4 -22 q9 -4 7 6" stroke="' + bodyC + '" stroke-width="5.5" fill="none" stroke-linecap="round"/>';
    s += '<ellipse cx="110" cy="122" rx="56" ry="46" fill="' + bodyC + '"/>';
    if (!sil) s += '<ellipse cx="110" cy="118" rx="54" ry="43" fill="url(#gb_' + uid + ')"/><ellipse cx="110" cy="144" rx="48" ry="26" fill="url(#gs_' + uid + ')"/><ellipse cx="110" cy="138" rx="33" ry="24" fill="#ffffff" opacity=".48"/>';
    s += '<ellipse cx="70" cy="162" rx="13" ry="8.5" fill="' + bodyC + '"/><ellipse cx="150" cy="162" rx="13" ry="8.5" fill="' + bodyC + '"/>';
    if (!sil) {
      s += '<path d="M64 162 q3 -2.4 6 0 M72 162 q3 -2.4 6 0 M144 162 q3 -2.4 6 0 M152 162 q3 -2.4 6 0" stroke="#000" stroke-width="1.2" fill="none" stroke-linecap="round" opacity=".22"/>' + eyesSVG(112, 88, 132, false, uid) + '<ellipse cx="66" cy="128" rx="9.5" ry="6" fill="#e87968" opacity=".42"/><ellipse cx="154" cy="128" rx="9.5" ry="6" fill="#e87968" opacity=".42"/>';
      if (mouth === "cat") s += '<ellipse cx="110" cy="122" rx="3.2" ry="2.2" fill="#e28395"/><path d="M100 126 q5 5 10 0 q5 5 10 0" stroke="#3a2f22" stroke-width="2.3" fill="none" stroke-linecap="round"/>';
      else if (mouth === "buck") s += '<ellipse cx="110" cy="121" rx="3.6" ry="2.4" fill="#f09bb0"/><path d="M101 126 q4.5 4.2 9 0 q4.5 4.2 9 0" stroke="#3a2f22" stroke-width="2.2" fill="none" stroke-linecap="round"/><path d="M107 129 l3 4 l3 -4 z" fill="#fff" stroke="#d5ccbd" stroke-width=".7"/>';
      else if (mouth === "fang") s += '<path d="M101 126 q4.5 4.2 9 0 q4.5 4.2 9 0" stroke="#3a2f22" stroke-width="2.2" fill="none" stroke-linecap="round"/><path d="M116 127 l3.2 4.5 l-4.8 .8 z" fill="#fff"/>';
      else s += '<path d="M101 126 q4.5 4.4 9 0 q4.5 4.4 9 0" stroke="#3a2f22" stroke-width="2.3" fill="none" stroke-linecap="round"/>';
    } else {
      s += '<text x="110" y="130" text-anchor="middle" font-size="38" fill="#8d8578" font-family="Georgia,serif">?</text>';
    }
    if (acc === "scarf") s += '<rect x="64" y="148" width="92" height="15" rx="7.5" fill="' + accC + '"/><rect x="122" y="156" width="14" height="18" rx="6" fill="' + accC + '"/>';
    else if (acc === "crown") s += '<path d="M88 54 L98 34 L110 49 L122 34 L132 54 Z" fill="' + accC + '"/>';
    else if (acc === "star" && !sil) s += '<path d="M156 42 l4.5 9 10 1.4 -7.2 6.5 1.8 10 -9.1 -4.7 -9.1 4.7 1.8 -10 -7.2 -6.5 10 -1.4 z" fill="' + accC + '"/>';
    else if (acc === "bow") s += '<path d="M110 156 l-16 -8 v16 z M110 156 l16 -8 v16 z" fill="' + accC + '"/><circle cx="110" cy="156" r="4.2" fill="' + accC + '"/>';
    return s + "</g></svg>";
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

  global.GBArt = {
    LEVELS: LEVELS,
    AVATARS: AVATARS,
    esc: esc,
    levelForXp: levelForXp,
    mainPetSVG: mainPetSVG,
    speciesSVG: speciesSVG,
    avatarSVG: avatarSVG,
    avatarHtml: avatarHtml,
    compressAvatarFile: compressAvatarFile,
    enableEyeTracking: enableEyeTracking
  };
})(window);
