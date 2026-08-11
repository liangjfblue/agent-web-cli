#!/usr/bin/env node
"use strict";

// Business CLI example: acquire a versioned awc session, then call the API.
// Browser credentials stay in this process and are never printed or persisted.

const { execFileSync } = require("node:child_process");

const BASE_URL = "http://localhost:3000";
const AUTH_NAME = "demo-admin";
const LOGIN_TIMEOUT_MS = 315_000;
const INFRA_EXIT_CODES = new Set([20, 21, 22, 30]);

class CliError extends Error {
  constructor(message, exitCode = 1) {
    super(message);
    this.exitCode = exitCode;
  }
}

function buildLoginArgs(refresh) {
  const args = ["session:acquire", AUTH_NAME, "--url", BASE_URL, "--interactive"];
  if (refresh) args.push("--refresh");
  args.push("--json");
  return args;
}

function runAwc(args, timeout) {
  try {
    return execFileSync("awc", args, {
      encoding: "utf8",
      timeout,
      stdio: ["ignore", "pipe", "pipe"],
    });
  } catch (error) {
    const code = Number(error.status) || (error.code === "ETIMEDOUT" ? 11 : 1);
    if (code === 10) throw new CliError("login required; run: demo-admin login", 10);
    if (code === 11) throw new CliError("browser login timed out", 11);
    if (INFRA_EXIT_CODES.has(code)) {
      throw new CliError("awc browser infrastructure is unavailable; run: awc sys:status", code);
    }
    throw new CliError("unable to acquire the Chrome session; run: awc sys:status", code);
  }
}

function parseSession(output) {
  let session;
  try {
    session = JSON.parse(output);
  } catch {
    throw new CliError("awc returned invalid JSON");
  }
  if (!session?.ok || !session?.data?.cookieHeader) {
    throw new CliError("awc did not return usable browser credentials", 10);
  }
  return session.data;
}

function acquireSession() {
  return parseSession(runAwc([
    "session:acquire", AUTH_NAME, "--url", BASE_URL, "--json",
  ], 15_000));
}

function login(refresh) {
  parseSession(runAwc(buildLoginArgs(refresh), LOGIN_TIMEOUT_MS));
  console.log("Browser login is available.");
}

async function callAPI(path) {
  const session = acquireSession();
  const response = await fetch(`${BASE_URL}${path}`, {
    headers: { Cookie: session.cookieHeader },
  });
  if (response.status === 401 || response.status === 403) {
    throw new CliError("browser credentials were rejected; run: demo-admin login --refresh", 10);
  }
  if (!response.ok) throw new CliError(`demo API failed (HTTP ${response.status})`);
  return response.json();
}

async function dashboard() {
  const data = await callAPI("/api/dashboard");
  console.log("📊 Dashboard");
  console.log("─────────────────────");
  console.log(`  今日订单:  ${data.orders}`);
  console.log(`  今日收入:  ¥${data.revenue.toLocaleString()}`);
  console.log(`  待处理:    ${data.pending}`);
  console.log(`  更新时间:  ${data.updatedAt}`);
  console.log(`  当前用户:  ${data.user}`);
}

async function users() {
  const data = await callAPI("/api/users");
  console.log(`👥 Users (${data.total})`);
  console.log("─────────────────────────────────────────");
  console.log("  ID  NAME    ROLE     EMAIL");
  for (const user of data.users) {
    console.log(`  ${String(user.id).padEnd(4)}${user.name.padEnd(8)}${user.role.padEnd(9)}${user.email}`);
  }
}

function status() {
  const session = acquireSession();
  console.log(`session available (${session.profileId || "legacy profile"})`);
}

function printHelp() {
  console.log(`Usage: demo-admin <command>

Commands:
  login [--refresh]  Open Chrome and wait for login
  dashboard          Show dashboard stats
  users              List users
  status             Check browser credentials`);
}

async function main(argv = process.argv.slice(2)) {
  const [command, ...args] = argv;
  if (!command || command === "help" || command === "--help" || command === "-h") {
    printHelp();
    return;
  }
  if (command === "login") {
    if (args.some((arg) => arg !== "--refresh") || args.filter((arg) => arg === "--refresh").length > 1) {
      throw new CliError("usage: demo-admin login [--refresh]", 2);
    }
    return login(args.includes("--refresh"));
  }
  if (args.length) throw new CliError(`unexpected argument: ${args[0]}`, 2);
  if (command === "dashboard") return dashboard();
  if (command === "users") return users();
  if (command === "status") return status();
  throw new CliError(`unknown command: ${command}`, 2);
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error.message || String(error));
    process.exit(error.exitCode || 1);
  });
}

module.exports = { buildLoginArgs, LOGIN_TIMEOUT_MS, main };
