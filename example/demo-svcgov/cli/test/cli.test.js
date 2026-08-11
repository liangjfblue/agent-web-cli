"use strict";

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const cliPath = path.join(__dirname, "..", "cli.js");
const { buildLoginArgs, LOGIN_TIMEOUT_MS } = require(cliPath);

test("uses the session contract for interactive login", () => {
  assert.deepEqual(buildLoginArgs(false), [
    "session:acquire", "svcgov", "--url", "http://127.0.0.1:3001", "--interactive", "--json",
  ]);
  assert.deepEqual(buildLoginArgs(true), [
    "session:acquire", "svcgov", "--url", "http://127.0.0.1:3001", "--interactive", "--refresh", "--json",
  ]);
  assert.equal(LOGIN_TIMEOUT_MS, 315_000);
});

test("does not call the legacy cookies:get integration", () => {
  assert.doesNotMatch(fs.readFileSync(cliPath, "utf8"), /cookies:get/);
});

test("unknown commands use the invalid-arguments exit code", () => {
  const result = spawnSync(process.execPath, [cliPath, "unknown"], { encoding: "utf8" });
  assert.equal(result.status, 2);
});
