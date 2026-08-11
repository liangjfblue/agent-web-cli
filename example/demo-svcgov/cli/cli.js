#!/usr/bin/env node
"use strict";

// Business CLI example: acquire a versioned awc session, then call the API.
// Browser credentials stay in this process and are never printed or persisted.

const { execFileSync } = require("node:child_process");

const BASE_URL = "http://127.0.0.1:3001";
const AUTH_NAME = "svcgov";
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
    if (code === 10) throw new CliError("login required; run: demo-svcgov login", 10);
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
    throw new CliError("browser credentials were rejected; run: demo-svcgov login --refresh", 10);
  }
  if (!response.ok) throw new CliError(`demo API failed (HTTP ${response.status})`);
  return response.json();
}

async function services() {
  const data = await callAPI("/api/services");
  const serviceRows = data.services;
  console.log(`🛡 Services (${serviceRows.length})`);
  console.log("─".repeat(70));
  console.log("  NAME               STATUS     INST  VERSION   CPU     MEMORY");
  for (const service of serviceRows) {
    const statusIcon = service.status === "healthy" ? "✓" : service.status === "degraded" ? "⚠" : "✗";
    console.log(`  ${statusIcon} ${service.name.padEnd(18)} ${service.status.padEnd(10)} ${String(service.instances).padEnd(5)} ${service.version.padEnd(9)} ${String(service.cpu).padEnd(7)} ${service.memory}`);
  }
  const issues = serviceRows.filter((service) => service.status !== "healthy");
  if (issues.length) {
    console.log("");
    for (const service of issues) console.log(`  ⚠ ${service.name}: ${service.status}`);
  }
}

async function logs(options) {
  const url = new URL("/api/logs", BASE_URL);
  if (options.service) url.searchParams.set("service", options.service);
  if (options.level) url.searchParams.set("level", options.level);
  const data = await callAPI(`${url.pathname}${url.search}`);
  console.log(`📋 Logs (${data.total} total, showing ${data.logs.length})${options.service ? ` · ${options.service}` : ""}${options.level ? ` · ${options.level}` : ""}`);
  console.log("─".repeat(70));
  for (const item of data.logs) {
    const icon = item.level === "error" ? "✗" : item.level === "warn" ? "⚠" : "→";
    console.log(`  ${icon} [${item.ts.slice(11, 19)}] ${item.service.padEnd(16)} ${item.level.toUpperCase().padEnd(6)} ${item.message}`);
  }
}

async function tables() {
  const data = await callAPI("/api/database/tables");
  console.log(`🗄 Database Tables (${data.tables.length})`);
  console.log("─".repeat(40));
  console.log("  NAME        ROWS  ENGINE");
  for (const table of data.tables) {
    console.log(`  ${table.name.padEnd(12)} ${String(table.rows).padEnd(6)} ${table.engine}`);
  }
}

async function showTable(name) {
  if (!name || !/^[a-zA-Z0-9_-]+$/.test(name)) throw new CliError("table requires a valid table name", 2);
  const data = await callAPI(`/api/database/${encodeURIComponent(name)}`);
  console.log(`🗄 ${data.table} (${data.rows.length} rows)`);
  console.log("─".repeat(60));
  console.log(`  ${data.columns.map((column) => column.padEnd(14)).join("")}`);
  console.log(`  ${"─".repeat(56)}`);
  for (const row of data.rows) {
    console.log(`  ${row.map((cell) => String(cell).padEnd(14)).join("")}`);
  }
}

function status() {
  const session = acquireSession();
  console.log(`session available (${session.profileId || "legacy profile"})`);
}

function parseOptions(args) {
  const options = {};
  for (let index = 0; index < args.length; index += 2) {
    const flag = args[index];
    const value = args[index + 1];
    if (!flag?.startsWith("--") || !value || value.startsWith("--")) {
      throw new CliError(`invalid option: ${flag || ""}`, 2);
    }
    const key = flag.slice(2);
    if (!new Set(["service", "level"]).has(key)) throw new CliError(`unknown option: ${flag}`, 2);
    options[key] = value;
  }
  return options;
}

function printHelp() {
  console.log(`Usage: demo-svcgov <command>

Commands:
  login [--refresh]                         Open Chrome and wait for login
  services                                  List services
  logs [--service <name>] [--level <level>] Query logs
  tables                                    List database tables
  table <name>                              Preview table data
  status                                    Check browser credentials`);
}

async function main(argv = process.argv.slice(2)) {
  const [command, ...args] = argv;
  if (!command || command === "help" || command === "--help" || command === "-h") {
    printHelp();
    return;
  }
  if (command === "login") {
    if (args.some((arg) => arg !== "--refresh") || args.filter((arg) => arg === "--refresh").length > 1) {
      throw new CliError("usage: demo-svcgov login [--refresh]", 2);
    }
    return login(args.includes("--refresh"));
  }
  if (command === "services" && !args.length) return services();
  if (command === "logs") return logs(parseOptions(args));
  if (command === "tables" && !args.length) return tables();
  if (command === "table" && args.length === 1) return showTable(args[0]);
  if (command === "status" && !args.length) return status();
  throw new CliError(`invalid command or arguments: ${command}`, 2);
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error.message || String(error));
    process.exit(error.exitCode || 1);
  });
}

module.exports = { buildLoginArgs, LOGIN_TIMEOUT_MS, main };
