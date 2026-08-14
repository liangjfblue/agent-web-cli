"use strict";

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const cli = path.join(__dirname, "..", "cli.js");
const authConfig = JSON.parse(fs.readFileSync(path.join(__dirname, "..", "auth", "ruoyi.json"), "utf8"));
const { buildLoginArgs, LOGIN_TIMEOUT_MS, parseArgs } = require(cli);

function run(...args) {
  return spawnSync(process.execPath, [cli, ...args], { encoding: "utf8" });
}

test("rejects write commands", () => {
  for (const command of [
    "user:create", "user:update", "user:delete", "user:reset-password", "user:change-status", "user:assign-roles",
    "role:create", "role:update", "role:delete", "role:change-status", "role:authorize",
    "operlog:delete", "operlog:clean", "loginlog:delete", "loginlog:clean",
  ]) {
    const result = run(command, "1");
    assert.equal(result.status, 2, command);
    assert.match(result.stderr, /unknown command/);
  }
});

test("rejects unexpected positional arguments", () => {
  const result = run("user:list", "unexpected");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /usage: ruoyi user:list/);
});

test("limits page size", () => {
  const result = run("role:list", "--page-size", "101");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /must not exceed 100/);
});

test("requires numeric detail ids", () => {
  for (const command of ["user:get", "role:get"]) {
    const result = run(command, "not-an-id");
    assert.equal(result.status, 2, command);
    assert.match(result.stderr, /requires a numeric/);
  }
});

test("validates operation log business type", () => {
  const result = run("operlog:list", "--business-type", "bogus");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /must be 0-9 or one of/);
});

test("validates login log status", () => {
  const result = run("loginlog:list", "--status", "maybe");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /must be success, failed, 0, or 1/);
});

test("rejects unexpected positionals on log lists", () => {
  for (const command of ["operlog:list", "loginlog:list"]) {
    const result = run(command, "extra");
    assert.equal(result.status, 2, command);
    assert.match(result.stderr, new RegExp(`usage: ruoyi ${command}`));
  }
});

test("parses login refresh as a boolean option", () => {
  assert.deepEqual(parseArgs(["login", "--refresh"]), {
    command: "login",
    positionals: [],
    options: { refresh: true },
  });
});

test("builds interactive login arguments without exposing credentials", () => {
  assert.deepEqual(buildLoginArgs(false), [
    "session:acquire", "ruoyi",
    "--url", "https://vue.ruoyi.vip",
    "--interactive", "--json",
  ]);
  assert.deepEqual(buildLoginArgs(true), [
    "session:acquire", "ruoyi",
    "--url", "https://vue.ruoyi.vip",
    "--interactive", "--refresh", "--json",
  ]);
});

test("allows five minutes for interactive browser login", () => {
  assert.equal(authConfig.timeout, "300s");
  assert.equal(LOGIN_TIMEOUT_MS, 315_000);
});
