// Page-injected DOM operation scripts. Each is an IIFE that the background
// worker injects via chrome.scripting.executeScript. They share a common
// locator helper that resolves an anchor or a CSS selector to a live element.
//
// Like snapshot.js, this file is self-contained and not imported as a module.

// ── shared locator ──────────────────────────────────────────────────────
// Resolves opts to a single element. opts may carry:
//   anchor:  "hash:index"   (1-based index into a fresh snapshot)
//   selector: CSS selector
//   role/name/text/label/testid: semantic matchers
function awcLocate(opts) {
  var all = awcCollect(opts.includeHidden);
  // 1. anchor: re-snapshot and verify hash, then index.
  if (opts.anchor) {
    var parts = String(opts.anchor).split(":");
    var wantHash = parts[0];
    var wantIdx = parseInt(parts[1], 10);
    var cur = awcFingerprint(all);
    if (cur !== wantHash) {
      var e = new Error("anchor target changed (page DOM mutated)");
      e.code = "ANCHOR_STALE";
      throw e;
    }
    if (wantIdx < 1 || wantIdx > all.length) {
      throw new Error("anchor index out of range: " + opts.anchor);
    }
    return all[wantIdx - 1].el;
  }
  // 2. semantic matchers
  var matched = awcSemantic(all, opts);
  if (matched.length > 0) {
    if (opts.strict && matched.length > 1) {
      throw new Error("strict match found " + matched.length + " elements");
    }
    return matched[0];
  }
  // 3. CSS selector fallback
  if (opts.selector) {
    var node = document.querySelector(opts.selector);
    if (!node) throw new Error("Element not found: " + opts.selector);
    return node;
  }
  throw new Error("no locator provided");
}

function awcCollect(includeHidden) {
  var out = [];
  var all = document.querySelectorAll(
    "a,button,input,select,textarea,[role],[contenteditable],[tabindex],[data-testid]"
  );
  for (var i = 0; i < all.length; i++) {
    var el = all[i];
    if (!includeHidden && !awcVisible(el)) continue;
    out.push({ el: el, tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "",
               name: el.getAttribute("aria-label") || el.getAttribute("title") || "",
               text: (el.innerText || el.textContent || "").trim(),
               label: awcLabel(el), testid: el.getAttribute("data-testid") || "" });
  }
  return out;
}

function awcVisible(el) {
  if (!el) return false;
  var r = el.getBoundingClientRect();
  if (r.width === 0 || r.height === 0) return false;
  var s = getComputedStyle(el);
  return s.visibility !== "hidden" && s.display !== "none" && s.opacity !== "0";
}

function awcLabel(el) {
  // Find an associated <label>.
  if (el.id) {
    var lab = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
    if (lab) return (lab.innerText || "").trim();
  }
  var parent = el.closest("label");
  if (parent) return (parent.innerText || "").trim();
  return "";
}

function awcSemantic(all, opts) {
  var matched = [];
  for (var i = 0; i < all.length; i++) {
    var it = all[i];
    if (opts.role && it.role !== opts.role && it.tag !== opts.role) continue;
    if (opts.name && !(it.name && it.name.indexOf(opts.name) >= 0)) continue;
    if (opts.text && !(it.text && it.text.indexOf(opts.text) >= 0)) continue;
    if (opts.label && !(it.label && it.label.indexOf(opts.label) >= 0)) continue;
    if (opts.testid && it.testid !== opts.testid) continue;
    matched.push(it.el);
  }
  return matched;
}

function awcFingerprint(items) {
  var tagCounts = {};
  var vis = 0;
  for (var i = 0; i < items.length; i++) {
    tagCounts[items[i].tag] = (tagCounts[items[i].tag] || 0) + 1;
    if (awcVisible(items[i].el)) vis++;
  }
  var s = items.length + ":" + vis + ":" + JSON.stringify(tagCounts);
  var h = 0x811c9dc5;
  for (var k = 0; k < s.length; k++) { h ^= s.charCodeAt(k); h = (h * 0x01000193) >>> 0; }
  return h.toString(16).padStart(8, "0").slice(0, 6);
}

// ── click ───────────────────────────────────────────────────────────────
function awcClick(opts) {
  var el = awcLocate(opts);
  el.scrollIntoView({ block: "center", behavior: "instant" });
  el.click();
  return { clicked: true, tag: el.tagName.toLowerCase(), text: (el.innerText || "").trim().slice(0, 80) };
}

// ── type (input) ────────────────────────────────────────────────────────
function awcType(opts) {
  var el = awcLocate(opts);
  el.focus();
  el.value = opts.value;
  el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent(new Event("change", { bubbles: true }));
  return { typed: true, tag: el.tagName.toLowerCase() };
}

// ── query (return matching element summaries) ──────────────────────────
function awcQuery(opts) {
  var all = awcCollect(opts.includeHidden);
  var matched = awcSemantic(all, opts);
  if (matched.length === 0 && opts.selector) {
    var nodes = document.querySelectorAll(opts.selector);
    for (var i = 0; i < nodes.length; i++) matched.push(nodes[i]);
  }
  return {
    count: matched.length,
    items: matched.slice(0, opts.limit || 100).map(function (el) {
      return {
        tag: el.tagName.toLowerCase(),
        role: el.getAttribute("role") || "",
        name: el.getAttribute("aria-label") || "",
        text: (el.innerText || el.textContent || "").trim().slice(0, 80),
        selector: el.id ? "#" + el.id : el.tagName.toLowerCase()
      };
    })
  };
}

// ── text ────────────────────────────────────────────────────────────────
function awcText(opts) {
  if (opts.selector) {
    var el = document.querySelector(opts.selector);
    if (!el) throw new Error("Element not found: " + opts.selector);
    return { text: (el.innerText || el.textContent || "").trim() };
  }
  return { text: (document.body.innerText || "").trim().slice(0, 5000) };
}
