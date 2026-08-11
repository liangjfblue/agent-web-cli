const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function loadCookieHandler(activeUrl = "https://active.example/path") {
  const calls = { tabQueries: [], cookieQueries: [] };
  const event = { addListener() {}, removeListener() {} };
  const chrome = {
    alarms: { onAlarm: event, create() {} },
    runtime: {
      onInstalled: event,
      onStartup: event,
      getManifest: () => ({ version: "test" }),
    },
    tabs: {
      query: async (details) => {
        calls.tabQueries.push(details);
        return [{ id: 1, url: activeUrl }];
      },
    },
    cookies: {
      getAll: async (details) => {
        calls.cookieQueries.push(details);
        return [];
      },
    },
  };
  const context = vm.createContext({ chrome, console, setTimeout, clearTimeout });
  const source = fs.readFileSync(path.join(__dirname, "background.js"), "utf8")
    + "\nglobalThis.__cookieHandler = HANDLERS['cookies.getAll'];";
  vm.runInContext(source, context);
  return { handler: context.__cookieHandler, calls };
}

test("cookies.getAll defaults to the active tab URL", async () => {
  const { handler, calls } = loadCookieHandler();
  await handler({ activeTab: true });

  assert.equal(calls.tabQueries.length, 1);
  assert.equal(calls.tabQueries[0].active, true);
  assert.equal(calls.tabQueries[0].currentWindow, true);
  assert.equal(calls.cookieQueries.length, 1);
  assert.equal(calls.cookieQueries[0].url, "https://active.example/path");
});

test("cookies.getAll reads every cookie only with explicit all", async () => {
  const { handler, calls } = loadCookieHandler();
  await handler({ all: true });

  assert.equal(calls.tabQueries.length, 0);
  assert.equal(calls.cookieQueries.length, 1);
  assert.equal(Object.keys(calls.cookieQueries[0]).length, 0);
});

test("cookies.getAll rejects a non-http active tab", async () => {
  const { handler } = loadCookieHandler("chrome://extensions");
  await assert.rejects(() => handler({ activeTab: true }), /pass --url or --all/);
});
