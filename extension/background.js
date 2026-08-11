// Agent Web CLI background service worker.
//
// Connects to the native host via chrome.runtime.connectNative and bridges
// op requests (tid/op/args) from the host into chrome.* API calls, replying
// with {tid, ok, data, code, msg}.
//
// Handlers are grouped into plain objects keyed by op, registered in a single
// table (HANDLERS). This keeps the dispatcher a flat switch and makes each
// capability a small testable function.

const HOST_NAME = "com.awc.host";
const RECONNECT_ALARM = "awc.reconnect";

// Capability advertised in status replies. Grows as more ops land.
const CAPABILITIES = [
  "status.get",
  "cookies.getAll",
  "cookies.remove",
  "tabs.query",
  "tabs.create",
  "tabs.update",
  "screenshot.capture",
  "dom.snapshot",
  "dom.click",
  "dom.type",
  "dom.query",
  "dom.text",
  "net.watch",
  "net.debug",
  "net.stop",
  "console.watch",
  "console.clear",
  "cdp.send",
  "cdp.listen",
  "wait.for",
  "rec.start",
  "rec.status",
  "rec.stop",
  "profile.identity",
  "profile.rename",
  "auth.open",
  "auth.check"
];

// ── native port lifecycle ──────────────────────────────────────────────

let port = null;
let connectedAt = 0;

chrome.runtime.onInstalled.addListener(connect);
chrome.runtime.onStartup.addListener(connect);

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === RECONNECT_ALARM && !port) connect();
});

function connect() {
  try {
    port = chrome.runtime.connectNative(HOST_NAME);
    port.onMessage.addListener(onHostMessage);
    port.onDisconnect.addListener(() => {
      port = null;
      chrome.alarms.create(RECONNECT_ALARM, { delayInMinutes: 0.25 });
    });
    connectedAt = Date.now();
    announceProfile();
  } catch {
    port = null;
    chrome.alarms.create(RECONNECT_ALARM, { delayInMinutes: 0.25 });
  }
}

async function announceProfile() {
  try {
    const profile = await getProfileIdentity();
    if (!port) return;
    port.postMessage({
      tid: "__awc_hello__",
      ok: true,
      data: {
        profile,
        extension: {
          connected: true,
          connectedAt,
          capabilities: CAPABILITIES,
          version: chrome.runtime.getManifest().version
        }
      }
    });
  } catch {
    // A failed hello falls back to the legacy single-profile endpoint.
  }
}

async function onHostMessage(msg) {
  const { tid, op, args } = msg;
  try {
    const handler = HANDLERS[op];
    if (!handler) throw opError("UNKNOWN_OP", "Unknown op: " + op);
    const data = await handler(args || {});
    port.postMessage({ tid, ok: true, data });
    if (op === "profile.rename") announceProfile();
  } catch (err) {
    port.postMessage({
      tid,
      ok: false,
      code: err.code || "EXTENSION_ERROR",
      msg: err.message || String(err)
    });
  }
}

function opError(code, message) {
  const err = new Error(message);
  err.code = code;
  return err;
}

// ── op handlers ─────────────────────────────────────────────────────────
// Each handler is async(args) -> data. Throwing rejects the request.

const HANDLERS = {
  "status.get": async () => {
    const profile = await getProfileIdentity().catch(() => null);
    return {
      extension: {
        connected: Boolean(port),
        connectedAt,
        capabilities: CAPABILITIES,
        version: chrome.runtime.getManifest().version
      },
      profile: profile || { profileId: "", profileName: "" }
    };
  },

  "cookies.getAll": async (args) => {
    const details = {};
    if (args.url) {
      details.url = args.url;
    } else if (!args.all) {
      const [active] = await chrome.tabs.query({ active: true, currentWindow: true });
      if (!active || !active.url || !/^https?:\/\//i.test(active.url)) {
        throw opError("BAD_ARGS", "the active tab has no http(s) URL; pass --url or --all");
      }
      details.url = active.url;
    }
    if (args.name) {
      // chrome.cookies.getAll has no name filter; filter client-side.
      const all = await chrome.cookies.getAll(details);
      const lower = String(args.name).toLowerCase();
      return { cookies: all.filter((c) => c.name.toLowerCase() === lower) };
    }
    const cookies = await chrome.cookies.getAll(details);
    return { cookies };
  },

  "cookies.remove": async (args) => {
    if (!args.url || !args.name) {
      throw opError("BAD_ARGS", "cookies.remove requires url and name");
    }
    const removed = await chrome.cookies.remove({ url: args.url, name: args.name });
    return { removed: Boolean(removed) };
  },

  "tabs.query": async () => {
    const tabs = await chrome.tabs.query({});
    return { tabs: tabs.map(serializeTab) };
  },

  "tabs.create": async (args) => {
    const tab = await openOrReuse(args);
    return { tab: serializeTab(tab) };
  },

  "tabs.update": async (args) => {
    const tabId = Number(args.tabId);
    const tab = await chrome.tabs.update(tabId, { active: true });
    return { tab: serializeTab(tab) };
  },

  "screenshot.capture": async (args) => {
    // Capture the visible area of the active tab in the last-focused window.
    const dataUrl = await chrome.tabs.captureVisibleTab(undefined, {
      format: "png"
    });
    return { dataUrl };
  },

  // ── DOM ops: inject a script into the page and return its result. ──

  "dom.snapshot": async (args) => {
    const tabId = await resolveTabId(args);
    const [res] = await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: SNAPSHOT_FN,
      args: [{ max: args.max || 500, includeHidden: !!args.includeHidden }]
    });
    return res.result;
  },

  "dom.click": async (args) => injectDomOp(args, "awcClick"),

  "dom.type": async (args) => {
    if (args.value == null) throw opError("BAD_ARGS", "dom.type requires --value");
    return injectDomOp(args, "awcType");
  },

  "dom.query": async (args) => injectDomOp(args, "awcQuery"),

  "dom.text": async (args) => injectDomOp(args, "awcText"),

  // ── observation: network, console, cdp, wait ──

  "net.watch": async (args) => startNetWatch(args),

  "net.stop": async (args) => stopNetWatch(args),

  "net.debug": async (args) => netDebug(args),

  "console.watch": async (args) => consoleWatch(args),

  "console.clear": async (args) => {
    const tabId = await resolveTabId(args);
    await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: () => { console.clear(); }
    });
    return { cleared: true };
  },

  "cdp.send": async (args) => cdpSend(args),

  "cdp.listen": async (args) => cdpListen(args),

  "wait.for": async (args) => waitFor(args),

  // ── recording & profiles ──

  "rec.start": async (args) => recStart(args),

  "rec.status": async () => recStatus(),

  "rec.stop": async (args) => recStop(args),

  "profile.identity": async () => getProfileIdentity(),

  "profile.rename": async (args) => renameProfile(args),

  // ── auth: configuration-driven login flow ──

  "auth.open": async (args) => authOpen(args),

  "auth.check": async (args) => authCheck(args)
};

// ── DOM injection helpers ───────────────────────────────────────────────
//
// The snapshot and domops scripts are large; rather than inline them in the
// handlers, we define them as functions here and inject their bodies. MV3
// permits function injection via chrome.scripting.executeScript({func}).

// SNAPSHOT_FN is the buildSnapshot function body (mirrors snapshot.js).
// Kept in sync manually; a build step can generate this from the .js file.
const SNAPSHOT_FN = buildSnapshot;

// buildSnapshot is the same function as in snapshot.js. It is duplicated here
// because chrome.scripting requires a real function reference, not a string.
function buildSnapshot(opts) {
  opts = opts || {};
  var max = opts.max || 500;
  var includeHidden = !!opts.includeHidden;
  var INTERACTIVE = { A:true,BUTTON:true,INPUT:true,SELECT:true,TEXTAREA:true,LABEL:true,SUMMARY:true };
  var ROLE_RE = /^(button|link|checkbox|radio|tab|menuitem|option|textbox|searchbox|combobox|slider)$/i;
  function visible(el){ if(!el||el.nodeType!==1)return false; var r=el.getBoundingClientRect(); if(r.width===0||r.height===0)return false; var s=getComputedStyle(el); return s.visibility!=="hidden"&&s.display!=="none"&&s.opacity!=="0"; }
  function textOf(el){ var t=(el.innerText||el.textContent||"").trim(); return t.length>120?t.slice(0,117)+"…":t; }
  function cssPath(el){ var parts=[]; var n=el; while(n&&n.nodeType===1&&parts.length<5){ var p=n.tagName.toLowerCase(); if(n.id){p+="#"+n.id;parts.unshift(p);break;} var cls=(n.className||"").toString().trim().split(/\s+/).filter(Boolean).slice(0,2); if(cls.length)p+="."+cls.join("."); parts.unshift(p); n=n.parentElement; } return parts.join(" > "); }
  function describe(el){ var role=el.getAttribute("role")||""; if(!role&&INTERACTIVE[el.tagName])role=el.tagName.toLowerCase(); if(!role&&el.isContentEditable)role="textbox"; var r=el.getBoundingClientRect(); return { tag:el.tagName.toLowerCase(), role:role, name:el.getAttribute("aria-label")||el.getAttribute("title")||"", text:textOf(el), type:el.getAttribute("type")||"", testid:el.getAttribute("data-testid")||el.getAttribute("data-test")||"", selector:cssPath(el), rect:{x:Math.round(r.x),y:Math.round(r.y),w:Math.round(r.width),h:Math.round(r.height)}, visible:visible(el) }; }
  var cands=[]; var all=document.querySelectorAll("a,button,input,select,textarea,[role],[contenteditable],[tabindex],[data-testid]");
  for(var i=0;i<all.length&&cands.length<max;i++){ var el=all[i]; if(!includeHidden&&!visible(el))continue; var role=el.getAttribute("role")||""; if(role&&!ROLE_RE.test(role)&&!INTERACTIVE[el.tagName])continue; if(!role&&!INTERACTIVE[el.tagName]&&!el.isContentEditable&&!el.hasAttribute("tabindex"))continue; cands.push(describe(el)); }
  var tc={},vc=0; for(var j=0;j<cands.length;j++){ tc[cands[j].tag]=(tc[cands[j].tag]||0)+1; if(cands[j].visible)vc++; }
  var fp=cands.length+":"+vc+":"+JSON.stringify(tc);
  var h=0x811c9dc5; for(var k=0;k<fp.length;k++){h^=fp.charCodeAt(k);h=(h*0x01000193)>>>0;} var hash=h.toString(16).padStart(8,"0").slice(0,6);
  for(var m=0;m<cands.length;m++){ cands[m].anchor=hash+":"+(m+1); }
  return { snapshotHash:hash, count:cands.length, elements:cands };
}

// injectDomOp runs one awc* function in the page. It passes a self-contained
// function to chrome.scripting.executeScript — one that includes ALL helper
// definitions inside its body, so it doesn't depend on closures that
// executeScript can't serialize.
//
// We use AWC_DOM_RUNNER below, a single function that defines every helper
// inline and dispatches by op name. This avoids new Function() (blocked by
// strict CSP) and avoids closure-serialization issues.
async function injectDomOp(args, fnName) {
  const tabId = await resolveTabId(args);
  const [res] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: AWC_DOM_RUNNER,
    args: [args, fnName]
  });
  return res.result;
}

// resolveTabId picks the tab to operate on: explicit tabId wins, else the
// first tab matching args.url, else the active tab.
async function resolveTabId(args) {
  if (args.tabId) return Number(args.tabId);
  if (args.url) {
    const tabs = await chrome.tabs.query({ url: args.url });
    if (tabs.length) return tabs[0].id;
    throw opError("TAB_NOT_FOUND", "no open tab matches url: " + args.url);
  }
  const [active] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (active) return active.id;
  throw opError("NO_TAB", "no active tab; pass --tab-id or --url");
}

// ── page-side helpers ───────────────────────────────────────────────────
// These mirror the functions in domops.js and are injected into the page via
// .toString(). They must be plain (no closures over background state).

function awcLocate(opts) {
  var all = awcCollect(opts.includeHidden);
  if (opts.anchor) {
    var parts = String(opts.anchor).split(":");
    var wantHash = parts[0], wantIdx = parseInt(parts[1], 10);
    if (awcFingerprint(all) !== wantHash) {
      var e = new Error("anchor target changed (page DOM mutated)");
      e.code = "ANCHOR_STALE"; throw e;
    }
    if (wantIdx < 1 || wantIdx > all.length) throw new Error("anchor index out of range: " + opts.anchor);
    return all[wantIdx - 1].el;
  }
  var matched = awcSemantic(all, opts);
  if (matched.length) { if (opts.strict && matched.length > 1) throw new Error("strict match found " + matched.length + " elements"); return matched[0]; }
  if (opts.selector) { var n = document.querySelector(opts.selector); if (!n) throw new Error("Element not found: " + opts.selector); return n; }
  throw new Error("no locator provided");
}

function awcCollect(includeHidden) {
  var out = [], all = document.querySelectorAll("a,button,input,select,textarea,[role],[contenteditable],[tabindex],[data-testid]");
  for (var i = 0; i < all.length; i++) {
    var el = all[i];
    if (!includeHidden && !awcVisible(el)) continue;
    out.push({ el: el, tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "", name: el.getAttribute("aria-label") || el.getAttribute("title") || "", text: (el.innerText || el.textContent || "").trim(), label: awcLabel(el), testid: el.getAttribute("data-testid") || "" });
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
  if (el.id) { var lab = document.querySelector('label[for="' + CSS.escape(el.id) + '"]'); if (lab) return (lab.innerText || "").trim(); }
  var p = el.closest("label"); return p ? (p.innerText || "").trim() : "";
}

function awcSemantic(all, opts) {
  var m = [];
  for (var i = 0; i < all.length; i++) {
    var it = all[i];
    if (opts.role && it.role !== opts.role && it.tag !== opts.role) continue;
    if (opts.name && !(it.name && it.name.indexOf(opts.name) >= 0)) continue;
    if (opts.text && !(it.text && it.text.indexOf(opts.text) >= 0)) continue;
    if (opts.label && !(it.label && it.label.indexOf(opts.label) >= 0)) continue;
    if (opts.testid && it.testid !== opts.testid) continue;
    m.push(it.el);
  }
  return m;
}

function awcFingerprint(items) {
  var tc = {}, vc = 0;
  for (var i = 0; i < items.length; i++) { tc[items[i].tag] = (tc[items[i].tag] || 0) + 1; if (awcVisible(items[i].el)) vc++; }
  var s = items.length + ":" + vc + ":" + JSON.stringify(tc);
  var h = 0x811c9dc5; for (var k = 0; k < s.length; k++) { h ^= s.charCodeAt(k); h = (h * 0x01000193) >>> 0; }
  return h.toString(16).padStart(8, "0").slice(0, 6);
}

function awcClick(opts) { var el = awcLocate(opts); el.scrollIntoView({ block: "center", behavior: "instant" }); el.click(); return { clicked: true, tag: el.tagName.toLowerCase(), text: (el.innerText || "").trim().slice(0, 80) }; }
function awcType(opts) { var el = awcLocate(opts); el.focus(); el.value = opts.value; el.dispatchEvent(new Event("input", { bubbles: true })); el.dispatchEvent(new Event("change", { bubbles: true })); return { typed: true, tag: el.tagName.toLowerCase() }; }
function awcQuery(opts) {
  var all = awcCollect(opts.includeHidden), matched = awcSemantic(all, opts);
  if (!matched.length && opts.selector) { var ns = document.querySelectorAll(opts.selector); for (var i = 0; i < ns.length; i++) matched.push(ns[i]); }
  return { count: matched.length, items: matched.slice(0, opts.limit || 100).map(function (el) { return { tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "", name: el.getAttribute("aria-label") || "", text: (el.innerText || el.textContent || "").trim().slice(0, 80), selector: el.id ? "#" + el.id : el.tagName.toLowerCase() }; }) };
}
function awcText(opts) {
  if (opts.selector) { var el = document.querySelector(opts.selector); if (!el) throw new Error("Element not found: " + opts.selector); return { text: (el.innerText || el.textContent || "").trim() }; }
  return { text: (document.body.innerText || "").trim().slice(0, 5000) };
}

// AWC_DOM_RUNNER is the single entry point injected into pages for DOM ops.
// It is fully self-contained: all helper functions are defined INSIDE its
// body, so chrome.scripting.executeScript can serialize it without losing
// closures. This avoids new Function() (blocked by strict CSP on sites like
// GitHub) and avoids depending on outer-scope functions.
//
// The outer awcLocate/awcClick/etc. defined above are kept for readability and
// testing but are NOT used for injection — this runner re-declares them inline.
function AWC_DOM_RUNNER(opts, fnName) {
  // ── inline helpers (copies of the outer functions) ──
  function visible(el) {
    if (!el || el.nodeType !== 1) return false;
    var r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    var s = getComputedStyle(el);
    return s.visibility !== "hidden" && s.display !== "none" && s.opacity !== "0";
  }
  function label(el) {
    if (el.id) { var lab = document.querySelector('label[for="' + CSS.escape(el.id) + '"]'); if (lab) return (lab.innerText || "").trim(); }
    var p = el.closest("label"); return p ? (p.innerText || "").trim() : "";
  }
  function collect(includeHidden) {
    var out = [], all = document.querySelectorAll("a,button,input,select,textarea,[role],[contenteditable],[tabindex],[data-testid]");
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (!includeHidden && !visible(el)) continue;
      out.push({ el: el, tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "", name: el.getAttribute("aria-label") || el.getAttribute("title") || "", text: (el.innerText || el.textContent || "").trim(), label: label(el), testid: el.getAttribute("data-testid") || "" });
    }
    return out;
  }
  function semantic(all, opts) {
    var m = [];
    for (var i = 0; i < all.length; i++) {
      var it = all[i];
      if (opts.role && it.role !== opts.role && it.tag !== opts.role) continue;
      if (opts.name && !(it.name && it.name.indexOf(opts.name) >= 0)) continue;
      if (opts.text && !(it.text && it.text.indexOf(opts.text) >= 0)) continue;
      if (opts.label && !(it.label && it.label.indexOf(opts.label) >= 0)) continue;
      if (opts.testid && it.testid !== opts.testid) continue;
      m.push(it.el);
    }
    return m;
  }
  function fingerprint(items) {
    var tc = {}, vc = 0;
    for (var i = 0; i < items.length; i++) { tc[items[i].tag] = (tc[items[i].tag] || 0) + 1; if (visible(items[i].el)) vc++; }
    var s = items.length + ":" + vc + ":" + JSON.stringify(tc);
    var h = 0x811c9dc5; for (var k = 0; k < s.length; k++) { h ^= s.charCodeAt(k); h = (h * 0x01000193) >>> 0; }
    return h.toString(16).padStart(8, "0").slice(0, 6);
  }
  function locate(opts) {
    var all = collect(opts.includeHidden);
    if (opts.anchor) {
      var parts = String(opts.anchor).split(":");
      var wantHash = parts[0], wantIdx = parseInt(parts[1], 10);
      if (fingerprint(all) !== wantHash) { var e = new Error("anchor target changed (page DOM mutated)"); e.code = "ANCHOR_STALE"; throw e; }
      if (wantIdx < 1 || wantIdx > all.length) throw new Error("anchor index out of range: " + opts.anchor);
      return all[wantIdx - 1].el;
    }
    var matched = semantic(all, opts);
    if (matched.length) { if (opts.strict && matched.length > 1) throw new Error("strict match found " + matched.length + " elements"); return matched[0]; }
    if (opts.selector) { var n = document.querySelector(opts.selector); if (!n) throw new Error("Element not found: " + opts.selector); return n; }
    throw new Error("no locator provided");
  }
  function click(opts) {
    var el = locate(opts);
    el.scrollIntoView({ block: "center", behavior: "instant" });
    el.click();
    return { clicked: true, tag: el.tagName.toLowerCase(), text: (el.innerText || "").trim().slice(0, 80) };
  }
  function type(opts) {
    var el = locate(opts);
    el.focus(); el.value = opts.value;
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
    return { typed: true, tag: el.tagName.toLowerCase() };
  }
  function query(opts) {
    var all = collect(opts.includeHidden), matched = semantic(all, opts);
    if (!matched.length && opts.selector) { var ns = document.querySelectorAll(opts.selector); for (var i = 0; i < ns.length; i++) matched.push(ns[i]); }
    return { count: matched.length, items: matched.slice(0, opts.limit || 100).map(function (el) { return { tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "", name: el.getAttribute("aria-label") || "", text: (el.innerText || el.textContent || "").trim().slice(0, 80), selector: el.id ? "#" + el.id : el.tagName.toLowerCase() }; }) };
  }
  function text(opts) {
    if (opts.selector) { var el = document.querySelector(opts.selector); if (!el) throw new Error("Element not found: " + opts.selector); return { text: (el.innerText || el.textContent || "").trim() }; }
    return { text: (document.body.innerText || "").trim().slice(0, 5000) };
  }
  // ── dispatch ──
  switch (fnName) {
    case "awcClick": return click(opts);
    case "awcType": return type(opts);
    case "awcQuery": return query(opts);
    case "awcText": return text(opts);
    default: throw new Error("unknown dom op: " + fnName);
  }
}

// ── helpers ─────────────────────────────────────────────────────────────

function serializeTab(tab) {
  return {
    id: tab.id,
    title: tab.title || "",
    url: tab.url || "",
    active: Boolean(tab.active),
    windowId: tab.windowId
  };
}

// openOrReuse reuses an existing tab matching url, otherwise creates one.
async function openOrReuse(args) {
  const url = args.url;
  if (args.reuse !== false && url) {
    const existing = await chrome.tabs.query({ url: sameOriginPattern(url) });
    if (existing.length > 0) {
      const tab = existing[0];
      if (url && tab.url !== url) {
        await chrome.tabs.update(tab.id, { url, active: !!args.active });
      } else if (args.active) {
        await chrome.tabs.update(tab.id, { active: true });
      }
      return await chrome.tabs.get(tab.id);
    }
  }
  return await chrome.tabs.create({ url, active: !!args.active });
}

// sameOriginPattern returns a queryable pattern for chrome.tabs.query({url}).
// For exact reuse we match by full URL; this helper exists so the match set
// can be widened later (e.g. prefix reuse) without touching openOrReuse.
function sameOriginPattern(url) {
  return [url];
}

// ════════════════════════════════════════════════════════════════════════
// Observation layer: network capture, CDP debug, console, wait.
//
// Two independent mechanisms:
//   • webRequest  — net.watch captures request metadata (no bodies).
//   • debugger    — net.debug / cdp.* / wait(xhr/console/ws) attach CDP to a
//                   tab for a bounded session, then detach.
// ════════════════════════════════════════════════════════════════════════

// ── net.watch: webRequest metadata capture ──────────────────────────────

const STATIC_EXT = /\.(js|css|png|jpe?g|gif|svg|woff2?|ttf|ico|map|wasm|mp[34]|webm)$/i;
const activeNetCaptures = new Map(); // captureId -> { tabId, filter, requests, listener, timer }

async function startNetWatch(args) {
  const tabId = await resolveTabId(args);
  const durationMs = args.durationMs || 10000;
  const captureId = "cap_" + Date.now() + "_" + Math.random().toString(36).slice(2, 8);
  const ignoreStatic = args.ignoreStatic !== false;
  const urlPattern = args.urlPattern || null;

  const requests = [];
  const listener = (detail) => {
    if (detail.tabId !== tabId) return;
    if (ignoreStatic && STATIC_EXT.test(detail.url)) return;
    if (urlPattern && detail.url.indexOf(urlPattern) < 0) return;
    requests.push({
      url: detail.url,
      method: detail.method,
      type: detail.type,
      statusCode: detail.statusCode || 0,
      requestId: detail.requestId,
      ts: Date.now()
    });
  };

  chrome.webRequest.onBeforeRequest.addListener(listener, { urls: ["<all_urls>"] });
  if (chrome.webRequest.onCompleted) {
    chrome.webRequest.onCompleted.addListener(listener, { urls: ["<all_urls>"] });
  }

  const timer = setTimeout(() => finishNetCapture(captureId), durationMs);
  activeNetCaptures.set(captureId, { tabId, requests, listener, timer, stopped: false });

  return { captureId, tabId, durationMs, started: true };
}

function finishNetCapture(captureId) {
  const cap = activeNetCaptures.get(captureId);
  if (!cap) return null;
  chrome.webRequest.onBeforeRequest.removeListener(cap.listener);
  if (chrome.webRequest.onCompleted) chrome.webRequest.onCompleted.removeListener(cap.listener);
  clearTimeout(cap.timer);
  activeNetCaptures.delete(captureId);
  return cap;
}

async function stopNetWatch(args) {
  if (args.captureId) {
    const cap = finishNetCapture(args.captureId);
    if (cap) return { captureId: args.captureId, stopped: true, requests: cap.requests };
    return { captureId: args.captureId, stopped: false };
  }
  // Stop all active captures.
  const all = [];
  for (const [id] of activeNetCaptures) {
    const cap = finishNetCapture(id);
    if (cap) all.push({ captureId: id, requests: cap.requests });
  }
  return { stopped: all.length, captures: all };
}

// net.watch is synchronous-start; results are polled via net.stop.
// To support the "return after duration" pattern used by the CLI, we also
// schedule a deferred reply: the host re-requests with the captureId.
//
// For the MVP, net.watch returns immediately with the captureId; the CLI
// then sleeps durationMs and calls net.stop to collect results.

// ── net.debug: CDP debug session with response bodies ───────────────────
//
// Attaches chrome.debugger to a tab, enables Network domain, collects
// requests + response bodies for durationMs, caches large bodies to disk
// via the host's net.body op, then detaches.

async function netDebug(args) {
  const tabId = await resolveTabId(args);
  const durationMs = args.durationMs || 10000;
  const maxBody = args.maxBodyBytes || 500000;
  const noBody = args.noBody === true;
  const urlPattern = args.urlPattern || null;

  const target = { tabId };
  await chrome.debugger.attach(target, "1.3");
  try {
    await chrome.debugger.sendCommand(target, "Network.enable");
    if (args.includeConsole) await chrome.debugger.sendCommand(target, "Runtime.enable");

    const requests = [];
    const consoleEvents = [];
    const bodyMap = new Map(); // requestId -> { body, truncated }

    const onEvent = (source, method, params) => {
      if (source.tabId !== tabId) return;
      if (method === "Network.responseReceived") {
        const r = params.response;
        if (urlPattern && params.response.url.indexOf(urlPattern) < 0) return;
        requests.push({
          requestId: params.requestId,
          url: r.url,
          status: r.status,
          method: r.requestData?.method || r.method || "",
          mimeType: r.mimeType,
          headers: r.headers || {},
          ts: params.timestamp
        });
        if (!noBody) {
          // Fetch body lazily after response finishes loading.
          chrome.debugger.sendCommand(target, "Network.getResponseBody", {
            requestId: params.requestId
          }).then((body) => {
            if (body && body.body) {
              const raw = body.base64Encoded ? atob(body.body) : body.body;
              if (raw.length > maxBody) {
                bodyMap.set(params.requestId, { body: raw.slice(0, maxBody), truncated: true, fullLen: raw.length });
              } else {
                bodyMap.set(params.requestId, { body: raw, truncated: false });
              }
            }
          }).catch(() => {});
        }
      } else if (method === "Runtime.consoleAPICalled" && args.includeConsole) {
        consoleEvents.push({
          type: params.type,
          args: (params.args || []).map((a) => a.value || a.description || "").join(" "),
          ts: params.timestamp
        });
      }
    };
    chrome.debugger.onEvent.addListener(onEvent);

    await sleep(durationMs);
    chrome.debugger.onEvent.removeListener(onEvent);

    // Attach bodies to their requests. Include bodyKey (a hash) and the full
    // body data so the CLI can cache it to disk for net:body retrieval.
    const result = requests.map((r) => {
      const b = bodyMap.get(r.requestId);
      if (b) {
        r.bodyPreview = b.body.slice(0, 500);
        r.bodyTruncated = b.truncated;
        if (b.truncated) r.bodyFullLen = b.fullLen;
        // Generate a bodyKey for disk caching. The CLI writes the full body
        // to ~/.awc/net-bodies/<bodyKey> so net:body can read it later
        // without re-attaching the debugger.
        r.bodyKey = "body_" + r.requestId + "_" + r.url.length;
        r.bodyData = b.body; // full body (may be truncated to maxBody)
      }
      return r;
    });

    return {
      tabId,
      durationMs,
      requests: result,
      consoleEvents: args.includeConsole ? consoleEvents : undefined
    };
  } finally {
    try { await chrome.debugger.detach(target); } catch {}
  }
}

// ── console.watch: capture page console via injected listener ───────────
//
// Injects a script that captures console.* into a page-global array, waits,
// then reads it back. This avoids the debugger attach prompt for simple
// console capture.

async function consoleWatch(args) {
  const tabId = await resolveTabId(args);
  const durationMs = args.durationMs || 5000;
  const level = args.level || "all"; // all|error|warn|info|log

  // Start capture.
  await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: () => {
      if (window.__awcConsole) return;
      window.__awcConsole = [];
      ["log", "info", "warn", "error", "debug"].forEach((lvl) => {
        const orig = console[lvl];
        console[lvl] = (...a) => {
          window.__awcConsole.push({ level: lvl, text: a.map(String).join(" "), ts: Date.now() });
          orig.apply(console, a);
        };
      });
      window.addEventListener("error", (e) => {
        window.__awcConsole.push({ level: "error", text: e.message, ts: Date.now() });
      });
    }
  });

  await sleep(durationMs);

  // Read back.
  const [res] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: (lvl) => {
      const all = window.__awcConsole || [];
      const filtered = lvl === "all" ? all : all.filter((e) => e.level === lvl);
      // Clear after reading.
      window.__awcConsole = [];
      return filtered;
    },
    args: [level]
  });
  return { tabId, level, events: res.result };
}

// ── cdp.send: one-shot CDP command ──────────────────────────────────────

async function cdpSend(args) {
  const tabId = await resolveTabId(args);
  const target = { tabId };
  await chrome.debugger.attach(target, "1.3");
  try {
    const result = await chrome.debugger.sendCommand(target, args.method, args.params || {});
    return { tabId, method: args.method, result };
  } finally {
    try { await chrome.debugger.detach(target); } catch {}
  }
}

// ── cdp.listen: attach, enable domains, collect events ──────────────────

async function cdpListen(args) {
  const tabId = await resolveTabId(args);
  const durationMs = args.durationMs || 5000;
  const eventPatterns = args.events || [];     // e.g. ["Network.*", "Page.*"]
  const enables = args.enables || [];          // e.g. ["Network.enable"]
  const target = { tabId };

  await chrome.debugger.attach(target, "1.3");
  try {
    // Enable requested (or inferred) domains.
    const domains = enables.length ? enables : inferEnables(eventPatterns);
    for (const d of domains) {
      try { await chrome.debugger.sendCommand(target, d); } catch {}
    }

    const events = [];
    const matches = (method) => eventPatterns.some((p) => {
      if (p.endsWith(".*")) return method.startsWith(p.slice(0, -1));
      return method === p;
    });

    const onEvent = (source, method, params) => {
      if (source.tabId !== tabId && !matches(method)) return;
      if (matches(method)) events.push({ method, params, ts: Date.now() });
    };
    chrome.debugger.onEvent.addListener(onEvent);
    await sleep(durationMs);
    chrome.debugger.onEvent.removeListener(onEvent);

    return { tabId, durationMs, events };
  } finally {
    try { await chrome.debugger.detach(target); } catch {}
  }
}

function inferEnables(patterns) {
  const domains = new Set();
  for (const p of patterns) {
    const dot = p.indexOf(".");
    if (dot > 0) domains.add(p.slice(0, dot) + ".enable");
  }
  return [...domains];
}

// ── wait.for: wait for a condition with timeout ─────────────────────────
//
// kind: selector | text | url | xhr
// selector/text poll the DOM via injected script.
// url/xhr poll tab state or listen to webRequest.

async function waitFor(args) {
  const kind = args.kind;
  const timeoutMs = args.timeoutMs || 30000;
  const intervalMs = args.intervalMs || 500;
  const deadline = Date.now() + timeoutMs;

  if (kind === "selector" || kind === "text") {
    const tabId = await resolveTabId(args);
    while (Date.now() < deadline) {
      const [res] = await chrome.scripting.executeScript({
        target: { tabId },
        world: "MAIN",
        func: (k, sel, txt) => {
          if (k === "selector") return !!document.querySelector(sel);
          if (k === "text") return (document.body.innerText || "").indexOf(txt) >= 0;
          return false;
        },
        args: [kind, args.selector || "", args.text || ""]
      });
      if (res && res.result) return { ok: true, kind, tabId, waited: timeoutMs - (deadline - Date.now()) };
      await sleep(intervalMs);
    }
    return { ok: false, kind, reason: "timeout", timeoutMs };
  }

  if (kind === "url") {
    const tabId = await resolveTabId(args);
    const pattern = args.urlPattern || "";
    while (Date.now() < deadline) {
      const [tab] = await chrome.tabs.query({});
      const t = (await chrome.tabs.get(tabId));
      if (t && t.url && t.url.indexOf(pattern) >= 0) {
        return { ok: true, kind: "url", tabId };
      }
      await sleep(intervalMs);
    }
    return { ok: false, kind: "url", reason: "timeout", timeoutMs };
  }

  if (kind === "xhr") {
    const tabId = await resolveTabId(args);
    const urlPattern = args.urlPattern || "";
    const status = args.statusCode || 0;
    return new Promise((resolve) => {
      let done = false;
      const listener = (detail) => {
        if (done || detail.tabId !== tabId) return;
        if (urlPattern && detail.url.indexOf(urlPattern) < 0) return;
        if (status && detail.statusCode !== status) return;
        done = true;
        chrome.webRequest.onCompleted.removeListener(listener);
        resolve({ ok: true, kind: "xhr", tabId, url: detail.url, statusCode: detail.statusCode });
      };
      chrome.webRequest.onCompleted.addListener(listener, { urls: ["<all_urls>"] });
      setTimeout(() => {
        if (!done) {
          done = true;
          chrome.webRequest.onCompleted.removeListener(listener);
          resolve({ ok: false, kind: "xhr", reason: "timeout", timeoutMs });
        }
      }, timeoutMs);
    });
  }

  throw opError("BAD_ARGS", "wait.for: unknown kind " + kind);
}

// ── shared sleep helper ─────────────────────────────────────────────────
function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

// ════════════════════════════════════════════════════════════════════════
// Recording: inject a listener that captures user actions into a page-global
// array; rec.stop reads it back.
//
// Captured events: click, input, change, submit, keydown(Enter/Escape/Tab),
// and URL changes (via popstate + pushState/replaceState hooks). Mouse-down
// and scroll are off by default.
// ════════════════════════════════════════════════════════════════════════

const activeRecordings = new Map();   // recordId -> { tabId, startedAt, opts }
const completedRecordings = new Map(); // recordId -> result
const REC_TTL_MS = 10 * 60 * 1000;

async function recStart(args) {
  const tabId = await resolveTabId(args);
  const recordId = "rec_" + Date.now() + "_" + Math.random().toString(36).slice(2, 8);
  const opts = {
    mouse: !!args.mouse,
    scroll: !!args.scroll
  };

  // Inject the recorder into the page.
  await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: RECORDER_FN,
    args: [recordId, opts]
  });

  activeRecordings.set(recordId, { tabId, startedAt: Date.now(), opts });

  // Clean up completed recordings after TTL.
  setTimeout(() => completedRecordings.delete(recordId), REC_TTL_MS);

  return { recordId, tabId, started: true, opts };
}

async function recStatus() {
  return {
    active: [...activeRecordings.keys()].map((id) => ({
      recordId: id,
      tabId: activeRecordings.get(id).tabId,
      duration: Date.now() - activeRecordings.get(id).startedAt
    })),
    completed: [...completedRecordings.keys()]
  };
}

async function recStop(args) {
  let recordId = args.recordId;
  let tabId;

  // Resolve recordId: explicit, or the single active recording.
  if (!recordId) {
    if (activeRecordings.size === 0) {
      throw opError("NO_RECORDING", "no active recording; pass a recordId from rec:start");
    }
    if (activeRecordings.size > 1) {
      throw opError("AMBIGUOUS", "multiple recordings active; specify --record-id");
    }
    recordId = [...activeRecordings.keys()][0];
  }

  const active = activeRecordings.get(recordId);
  if (active) {
    tabId = active.tabId;
    activeRecordings.delete(recordId);
  } else {
    // Maybe already stopped and cached.
    const cached = completedRecordings.get(recordId);
    if (cached) return { recordId, ...cached };
    throw opError("NOT_FOUND", "recording not found: " + recordId);
  }

  // Read the captured events from the page.
  const [res] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: (id) => {
      const key = "__awcRec_" + id;
      const events = window[key] ? window[key].slice() : [];
      // Re-attach is prevented by a flag, so the page keeps its recorder but
      // no longer appends; we clear the array so a future start is clean.
      if (window[key]) window[key] = null;
      return events;
    },
    args: [recordId]
  });

  const result = {
    recordId,
    tabId,
    eventCount: res.result.length,
    events: res.result,
    duration: Date.now() - (active ? active.startedAt : 0)
  };
  completedRecordings.set(recordId, result);
  return result;
}

// RECORDER_FN is injected into the page. It must be self-contained.
const RECORDER_FN = function awcRecorder(recordId, opts) {
  var key = "__awcRec_" + recordId;
  if (window[key]) return; // already recording on this tab
  var events = [];
  window[key] = events;

  function describe(el) {
    if (!el || el.nodeType !== 1) return null;
    var tag = el.tagName.toLowerCase();
    return {
      tag: tag,
      type: el.getAttribute("type") || "",
      name: el.getAttribute("name") || "",
      id: el.id || "",
      role: el.getAttribute("role") || "",
      text: (el.innerText || el.textContent || "").trim().slice(0, 80),
      selector: el.id ? "#" + el.id : tag,
      href: el.getAttribute("href") || "",
      value: isSensitive(el) ? "<redacted>" : (el.value || "")
    };
  }

  function isSensitive(el) {
    if (el.type === "password") return true;
    var name = (el.getAttribute("name") || "").toLowerCase();
    return /password|token|cookie|captcha|verification|secret/.test(name);
  }

  var lastUrl = location.href;
  function push(type, detail) {
    events.push({ type: type, url: location.href, ts: Date.now(), detail: detail });
  }

  document.addEventListener("click", function (e) {
    push("click", describe(e.target));
  }, true);
  document.addEventListener("input", function (e) {
    push("input", describe(e.target));
  }, true);
  document.addEventListener("change", function (e) {
    push("change", describe(e.target));
  }, true);
  document.addEventListener("submit", function (e) {
    push("submit", describe(e.target));
  }, true);
  document.addEventListener("keydown", function (e) {
    if (e.key === "Enter" || e.key === "Escape" || e.key === "Tab") {
      push("keydown", { key: e.key, target: describe(e.target) });
    }
  }, true);

  if (opts.mouse) {
    document.addEventListener("mousedown", function (e) {
      push("mousedown", { x: e.clientX, y: e.clientY, target: describe(e.target) });
    }, true);
  }
  if (opts.scroll) {
    window.addEventListener("scroll", function () {
      push("scroll", { x: window.scrollX, y: window.scrollY });
    }, true);
  }

  // URL change detection.
  var origPush = history.pushState;
  var origReplace = history.replaceState;
  history.pushState = function () {
    var r = origPush.apply(this, arguments);
    push("navigate", { from: lastUrl, to: location.href });
    lastUrl = location.href;
    return r;
  };
  history.replaceState = function () {
    var r = origReplace.apply(this, arguments);
    push("navigate", { from: lastUrl, to: location.href });
    lastUrl = location.href;
    return r;
  };
  window.addEventListener("popstate", function () {
    push("navigate", { from: lastUrl, to: location.href });
    lastUrl = location.href;
  });
};

// ════════════════════════════════════════════════════════════════════════
// Profiles: each Chrome user-profile gets a stable id stored in
// chrome.storage.local. The host uses this to route commands to the right
// browser profile.
// ════════════════════════════════════════════════════════════════════════

const PROFILE_KEY = "awcProfile";

async function getProfileIdentity() {
  let stored = await chrome.storage.local.get(PROFILE_KEY);
  let profile = stored[PROFILE_KEY];
  if (!profile || !profile.id) {
    profile = { id: genProfileId(), name: "", createdAt: Date.now() };
    await chrome.storage.local.set({ [PROFILE_KEY]: profile });
  }
  return {
    profileId: profile.id,
    profileName: profile.name || "",
    createdAt: profile.createdAt
  };
}

async function renameProfile(args) {
  let stored = await chrome.storage.local.get(PROFILE_KEY);
  let profile = stored[PROFILE_KEY];
  if (!profile) {
    profile = { id: genProfileId(), name: "", createdAt: Date.now() };
  }
  profile.name = String(args.name || "").trim().slice(0, 80);
  await chrome.storage.local.set({ [PROFILE_KEY]: profile });
  return {
    profileId: profile.id,
    profileName: profile.name,
    renamed: true
  };
}

// genProfileId produces a stable-looking id: awc-<8 hex chars>. It is NOT a
// cryptographic secret — it only needs to be unique within one machine.
function genProfileId() {
  var s = "";
  var chars = "0123456789abcdef";
  for (var i = 0; i < 8; i++) s += chars[Math.floor(Math.random() * 16)];
  return "awc-" + s;
}

// ════════════════════════════════════════════════════════════════════════
// authLogin: configuration-driven login flow.
//
// The CLI sends a config (loginUrl, loginButton, loggedInWhen, ssoSteps).
// The extension opens the login URL, then polls: check if logged in, if not
// click the login button / SSO buttons, until the loggedInWhen condition is
// met or timeout. This is generic — any site can be configured without code.
// ════════════════════════════════════════════════════════════════════════

// authOpen opens the login URL, waits for it to settle, then attempts
// auto-login actions (click login button, SSO steps). It returns immediately
// after the first round of actions — the CLI drives the polling loop and
// calls auth.check repeatedly until logged in.
async function authOpen(args) {
  var loginUrl = args.loginUrl;
  var loginButton = args.loginButton || {};
  var loggedInWhen = args.loggedInWhen || {};
  var ssoSteps = args.ssoSteps || [];
  var autoDetect = !loginButton.text && !loginButton.selector && !loginButton.role && !loginButton.name;

  // Open the login URL (no reuse — we must visit the actual login page).
  var tab = await openOrReuse({ url: loginUrl, reuse: false, active: false });
  var tabId = tab.id;
  await sleep(2000); // let the page settle

  // First check: maybe already logged in (cached session / SSO).
  var state = await readLoginState(tabId, loggedInWhen);
  if (state.loggedIn) {
    return { loggedIn: true, status: "logged-in", tabId: tabId, url: state.url };
  }

  // Try SSO steps (e.g. UUAP passwordless).
  for (var i = 0; i < ssoSteps.length; i++) {
    var step = ssoSteps[i];
    if (state.url.indexOf(step.hostContains) >= 0) {
      await tryClickByText(tabId, step.clickText);
      await sleep(1500);
      break;
    }
  }

  // Try auto-clicking the login button.
  if (autoDetect) {
    await autoClickLoginButton(tabId);
  } else {
    await tryClickLocator(tabId, loginButton);
  }

  await sleep(1000);
  // Re-check after actions.
  state = await readLoginState(tabId, loggedInWhen);
  return {
    loggedIn: state.loggedIn,
    status: state.status,
    tabId: tabId,
    url: state.url
  };
}

// authCheck checks login status. Cookie condition is checked FIRST and does
// not depend on any tab — chrome.cookies.get queries the cookie store
// directly. Only if no cookie condition is configured does it fall back to
// DOM-based checks (which need a tab).
async function authCheck(args) {
  var loginUrl = args.loginUrl;
  var loggedInWhen = args.loggedInWhen || {};
  var cond = loggedInWhen || {};

  // ── Cookie check: tab-independent, most reliable ──
  if (cond.cookie && cond.cookie.url && cond.cookie.name) {
    try {
      var cookie = await chrome.cookies.get({ url: cond.cookie.url, name: cond.cookie.name });
      if (cookie) {
        var valueOk = !cond.cookie.value || cookie.value === cond.cookie.value;
        if (valueOk) {
          return { loggedIn: true, status: "logged-in (cookie)", url: cond.cookie.url };
        }
        return { loggedIn: false, status: "cookie-value-mismatch", url: cond.cookie.url };
      }
      return { loggedIn: false, status: "cookie-missing", url: cond.cookie.url };
    } catch (e) {
      // Fall through to DOM-based check.
    }
  }

  // ── DOM-based check: needs a tab ──
  var origin = loginUrl.replace(/^(https?:\/\/[^/]+).*/, "$1");
  var tabs = await chrome.tabs.query({});
  var tab = tabs.find(function (t) {
    return t.url && t.url.indexOf(origin) >= 0;
  });
  if (!tab) {
    return { loggedIn: false, status: "no-tab", reason: "no open tab matches " + origin };
  }

  var state = await readLoginState(tab.id, loggedInWhen);
  return {
    loggedIn: state.loggedIn,
    status: state.status,
    tabId: tab.id,
    url: state.url
  };
}

// autoClickLoginButton finds and clicks a likely login button using common
// patterns. This frees users from writing CSS selectors.
//
// Patterns tried in order:
//   1. <button>/<input type=submit> whose text/value matches 登录|login|sign in|登入|登入
//   2. type=submit (the only submit button on the form)
//   3. <button> inside a <form> with a password field
//   4. element with role=button matching the login text patterns
async function autoClickLoginButton(tabId) {
  try {
    var [res] = await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: function () {
        var LOGIN_RE = /(登录|登入|登陸|login|sign\s*in|log\s*in)/i;

        // 1. buttons/inputs with login text
        var candidates = Array.from(document.querySelectorAll(
          "button, input[type=submit], a[role=button], [role=button], a"
        ));
        for (var i = 0; i < candidates.length; i++) {
          var el = candidates[i];
          var text = (el.innerText || el.textContent || el.value || "").trim();
          if (text && LOGIN_RE.test(text) && el.offsetParent !== null) {
            el.click();
            return { clicked: true, method: "text-match", text: text.slice(0, 40) };
          }
        }

        // 2. a single type=submit on the page
        var submits = document.querySelectorAll("input[type=submit], button[type=submit]");
        if (submits.length === 1) {
          submits[0].click();
          return { clicked: true, method: "single-submit" };
        }

        // 3. button inside a form that has a password field
        var pwForm = document.querySelector("form input[type=password]");
        if (pwForm) {
          var form = pwForm.closest("form");
          if (form) {
            var btn = form.querySelector("button, input[type=submit]");
            if (btn) {
              btn.click();
              return { clicked: true, method: "form-password" };
            }
          }
        }

        return { clicked: false };
      }
    });
    return res.result && res.result.clicked;
  } catch (e) {
    return false;
  }
}

// readLoginState checks the loggedInWhen conditions against the current tab.
//
// Signals checked (in priority order):
//   1. cookie condition (if configured) — chrome.cookies.get, most reliable
//   2. body className (body.logged-in etc.) — always checked as a bonus signal
//   3. URL + button text heuristics — default when nothing else configured
//
// If the user supplied a cookie condition, only that matters (it is the
// authoritative signal). Otherwise defaults: URL must not contain login
// paths AND no visible login button on the page, with body-class as an
// override positive signal.
async function readLoginState(tabId, cond) {
  cond = cond || {};
  var url = "";
  try { url = (await chrome.tabs.get(tabId)).url || ""; } catch (e) { return { loggedIn: false, url: "", status: "no-tab" }; }

  // ── Priority 1: cookie check (user-configured, most reliable) ──
  if (cond.cookie && cond.cookie.url && cond.cookie.name) {
    try {
      var cookie = await chrome.cookies.get({ url: cond.cookie.url, name: cond.cookie.name });
      if (cookie) {
        var valueOk = !cond.cookie.value || cookie.value === cond.cookie.value;
        if (valueOk) {
          return { loggedIn: true, url: url, status: "logged-in (cookie)" };
        }
      }
      return { loggedIn: false, url: url, status: "cookie-missing" };
    } catch (e) {
      // Fall through to other signals if cookies API fails.
    }
  }

  // ── Priority 2: body class + buttons (injected in one call) ──
  var bodyClass = "";
  var buttons = [];
  try {
    var [res] = await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: function () {
        var cls = (document.body && document.body.className) || "";
        var btns = Array.from(document.querySelectorAll("button,a,[role=button]"))
          .map(function (el) {
            if (el.offsetParent === null && el.tagName !== "INPUT") return null;
            var t = (el.innerText || el.textContent || el.value || "").trim();
            return t || null;
          })
          .filter(Boolean);
        return { bodyClass: cls, buttons: btns };
      }
    });
    if (res && res.result) {
      bodyClass = res.result.bodyClass || "";
      buttons = res.result.buttons || [];
    }
  } catch (e) {}

  // Body class strong signal: body.logged-in / body.authenticated
  var bodyLoggedIn = /\b(logged[-_]?in|authenticated|user[-_]?logged)\b/i.test(bodyClass);

  // ── Priority 3: URL + button heuristics ──
  var hasCond = cond.urlNotContains || cond.noButtonText;
  var effectiveUrlCond = cond.urlNotContains || "/login";
  var urlOk = url.indexOf(effectiveUrlCond) < 0;

  // Login button detection: exact phrase match (not substring).
  var loginButtonPresent = buttons.some(function (t) {
    var lower = t.toLowerCase().trim();
    if (cond.noButtonText && lower.indexOf(cond.noButtonText.toLowerCase()) >= 0) return true;
    if (!hasCond) {
      // Default exact matches.
      if (lower === "登录" || lower === "登入" || lower === "登陸") return true;
      if (lower === "sign in" || lower === "log in" || lower === "login") return true;
      if (/^(sign in|login)\b/i.test(lower.replace(/\s+/g, " "))) return true;
      if (/^(登录|登入)/.test(t.trim()) && t.trim().length <= 6) return true;
    }
    return false;
  });

  var buttonOk = !loginButtonPresent;
  var loggedIn = (bodyLoggedIn || (urlOk && buttonOk)) && url !== "" && url.indexOf("about:") !== 0;

  var status;
  if (loggedIn) status = bodyLoggedIn ? "logged-in (body-class)" : "logged-in";
  else if (!urlOk) status = "on-login-page";
  else if (!buttonOk) status = "login-button-present";
  else status = "unknown";

  return { loggedIn: loggedIn, url: url, status: status, bodyClass: bodyClass.slice(0, 80), buttonCount: buttons.length };
}

// tryClickByText clicks the first element whose text contains the given string.
async function tryClickByText(tabId, text) {
  try {
    var [res] = await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: function (txt) {
        var els = Array.from(document.querySelectorAll("button,a,[role=button]"));
        var target = els.find(function (el) { return (el.innerText || "").indexOf(txt) >= 0; });
        if (target) { target.click(); return true; }
        return false;
      },
      args: [text]
    });
    return res.result;
  } catch (e) { return false; }
}

// tryClickLocator clicks using a locator (text/selector/role/name).
async function tryClickLocator(tabId, loc) {
  try {
    if (loc.selector) {
      var [res] = await chrome.scripting.executeScript({
        target: { tabId },
        world: "MAIN",
        func: function (sel) {
          var el = document.querySelector(sel);
          if (el) { el.click(); return true; }
          return false;
        },
        args: [loc.selector]
      });
      return res.result;
    }
    if (loc.text) return await tryClickByText(tabId, loc.text);
    if (loc.role) {
      var [res2] = await chrome.scripting.executeScript({
        target: { tabId },
        world: "MAIN",
        func: function (role) {
          var el = document.querySelector('[role="' + role + '"]') ||
                   document.querySelector(role);
          if (el) { el.click(); return true; }
          return false;
        },
        args: [loc.role]
      });
      return res2.result;
    }
  } catch (e) {}
  return false;
}
