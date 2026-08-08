// Page-injected snapshot script. Executed via chrome.scripting.executeScript
// with world: "MAIN" so it runs in the page context.
//
// It walks the document for actionable elements, assigns each an anchor of
// the form "<snapshotHash>:<index>", and returns a compact representation.
// The snapshotHash is a deterministic fingerprint of the element set, so it
// changes when the page DOM changes — which is how stale anchors are detected.
//
// This file is NOT loaded as a service-worker module; it is read by the
// background worker and injected as a function body. Keep it self-contained.

(function buildSnapshot(opts) {
  opts = opts || {};
  var max = opts.max || 500;
  var includeHidden = !!opts.includeHidden;

  var INTERACTIVE = {
    A: true, BUTTON: true, INPUT: true, SELECT: true, TEXTAREA: true,
    LABEL: true, SUMMARY: true
  };
  var ROLE_RE = /^(button|link|checkbox|radio|tab|menuitem|option|textbox|searchbox|combobox|slider)$/i;

  function isVisible(el) {
    if (!el || el.nodeType !== 1) return false;
    var rect = el.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return false;
    var style = getComputedStyle(el);
    if (style.visibility === "hidden" || style.display === "none") return false;
    if (style.opacity === "0") return false;
    return true;
  }

  function textOf(el) {
    var t = (el.innerText || el.textContent || "").trim();
    return t.length > 120 ? t.slice(0, 117) + "…" : t;
  }

  function describe(el) {
    var role = el.getAttribute("role") || "";
    if (!role && INTERACTIVE[el.tagName]) role = el.tagName.toLowerCase();
    if (!role && el.isContentEditable) role = "textbox";
    return {
      tag: el.tagName.toLowerCase(),
      role: role,
      name: el.getAttribute("aria-label") || el.getAttribute("title") || "",
      text: textOf(el),
      type: el.getAttribute("type") || "",
      testid: el.getAttribute("data-testid") || el.getAttribute("data-test") || "",
      selector: cssPath(el),
      rect: (function (r) {
        return { x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height) };
      })(el.getBoundingClientRect()),
      visible: isVisible(el)
    };
  }

  // cssPath builds a short CSS selector for el. It is not guaranteed unique;
  // it is a hint for humans and a fallback for re-location.
  function cssPath(el) {
    var parts = [];
    var node = el;
    while (node && node.nodeType === 1 && parts.length < 5) {
      var part = node.tagName.toLowerCase();
      if (node.id) { part += "#" + node.id; parts.unshift(part); break; }
      var cls = (node.className || "").toString().trim().split(/\s+/).filter(Boolean).slice(0, 2);
      if (cls.length) part += "." + cls.join(".");
      parts.unshift(part);
      node = node.parentElement;
    }
    return parts.join(" > ");
  }

  // Collect candidate elements.
  var candidates = [];
  var all = document.querySelectorAll(
    "a,button,input,select,textarea,[role],[contenteditable],[tabindex],[data-testid]"
  );
  for (var i = 0; i < all.length && candidates.length < max; i++) {
    var el = all[i];
    if (!includeHidden && !isVisible(el)) continue;
    var role = el.getAttribute("role") || "";
    if (role && !ROLE_RE.test(role) && !INTERACTIVE[el.tagName]) continue;
    if (!role && !INTERACTIVE[el.tagName] && !el.isContentEditable && !el.hasAttribute("tabindex")) continue;
    candidates.push(describe(el));
  }

  // Deterministic fingerprint: element count + tag histogram + visibility sum.
  // Same DOM -> same hash; structural change -> different hash.
  var tagCounts = {};
  var visCount = 0;
  for (var j = 0; j < candidates.length; j++) {
    var tag = candidates[j].tag;
    tagCounts[tag] = (tagCounts[tag] || 0) + 1;
    if (candidates[j].visible) visCount++;
  }
  var fingerprint = candidates.length + ":" + visCount + ":" + JSON.stringify(tagCounts);
  var hash = (function (s) {
    var h = 0x811c9dc5;
    for (var k = 0; k < s.length; k++) {
      h ^= s.charCodeAt(k);
      h = (h * 0x01000193) >>> 0;
    }
    return h.toString(16).padStart(8, "0").slice(0, 6);
  })(fingerprint);

  // Assign anchors: hash:1-based-index.
  for (var m = 0; m < candidates.length; m++) {
    candidates[m].anchor = hash + ":" + (m + 1);
  }

  return {
    snapshotHash: hash,
    count: candidates.length,
    elements: candidates
  };
})
