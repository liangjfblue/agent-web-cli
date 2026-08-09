#!/usr/bin/env node
// svcgov-cli — business CLI for the service governance platform.
//
// Reads cookies from Chrome via awc, calls the platform's authenticated APIs.
//
// Usage:
//   node cli.js services                  # list all services
//   node cli.js logs --service gateway    # gateway logs
//   node cli.js logs --level error        # only errors
//   node cli.js logs --service gateway --level error  # both filters
//   node cli.js tables                    # database tables
//   node cli.js table users              # preview table data
//   node cli.js status                    # check login state

const { execSync } = require("child_process");

const BASE_URL = "http://localhost:3001";
const AUTH_NAME = "svcgov";

// ── Read cookie from Chrome via awc ──
function getCookie() {
  try {
    return execSync(`awc cookies:get --url ${BASE_URL} --header`, {
      encoding: "utf8", timeout: 10000,
    }).trim();
  } catch {
    console.error("✗ 无法读取 cookie — awc 是否已连接？运行: awc sys:status");
    process.exit(1);
  }
}

function checkLogin() {
  try {
    return execSync(`awc auth:login ${AUTH_NAME} --check`, {
      encoding: "utf8", timeout: 10000,
    }).trim().includes("logged in");
  } catch {
    return false;
  }
}

async function callAPI(path) {
  const cookie = getCookie();
  const resp = await fetch(`${BASE_URL}${path}`, { headers: { Cookie: cookie } });
  if (resp.status === 401) {
    console.error("✗ cookie 失效或未登录");
    console.error(`  重新登录: awc auth:login ${AUTH_NAME}`);
    process.exit(1);
  }
  return resp.json();
}

function ensureLogin() {
  if (!checkLogin()) {
    console.error("✗ 未登录，请先运行:");
    console.error(`  awc auth:login ${AUTH_NAME}`);
    process.exit(1);
  }
}

// ── Commands ──

async function services() {
  ensureLogin();
  const data = await callAPI("/api/services");
  const svcs = data.services;
  console.log(`🛡 Services (${svcs.length})`);
  console.log("─".repeat(70));
  console.log("  NAME               STATUS     INST  VERSION   CPU     MEMORY");
  for (const s of svcs) {
    const statusIcon = s.status === "healthy" ? "✓" : s.status === "degraded" ? "⚠" : "✗";
    console.log(`  ${statusIcon} ${s.name.padEnd(18)} ${s.status.padEnd(10)} ${String(s.instances).padEnd(5)} ${s.version.padEnd(9)} ${String(s.cpu).padEnd(7)} ${s.memory}`);
  }
  // Highlight problems
  const issues = svcs.filter(s => s.status !== "healthy");
  if (issues.length) {
    console.log("");
    for (const s of issues) {
      console.log(`  ⚠ ${s.name}: ${s.status}`);
    }
  }
}

async function logs(args) {
  ensureLogin();
  let path = "/api/logs?";
  if (args.service) path += `service=${args.service}&`;
  if (args.level) path += `level=${args.level}&`;
  const data = await callAPI(path);
  const logs = data.logs;
  console.log(`📋 Logs (${data.total} total, showing ${logs.length})${args.service ? " · " + args.service : ""}${args.level ? " · " + args.level : ""}`);
  console.log("─".repeat(70));
  for (const l of logs) {
    const icon = l.level === "error" ? "✗" : l.level === "warn" ? "⚠" : "→";
    console.log(`  ${icon} [${l.ts.slice(11, 19)}] ${l.service.padEnd(16)} ${l.level.toUpperCase().padEnd(6)} ${l.message}`);
  }
}

async function tables() {
  ensureLogin();
  const data = await callAPI("/api/database/tables");
  console.log(`🗄 Database Tables (${data.tables.length})`);
  console.log("─".repeat(40));
  console.log("  NAME        ROWS  ENGINE");
  for (const t of data.tables) {
    console.log(`  ${t.name.padEnd(12)} ${String(t.rows).padEnd(6)} ${t.engine}`);
  }
}

async function showTable(name) {
  ensureLogin();
  const data = await callAPI(`/api/database/${name}`);
  console.log(`🗄 ${data.table} (${data.rows.length} rows)`);
  console.log("─".repeat(60));
  // Header
  console.log("  " + data.columns.map(c => c.padEnd(14)).join(""));
  console.log("  " + "─".repeat(56));
  // Rows
  for (const row of data.rows) {
    console.log("  " + row.map(c => String(c).padEnd(14)).join(""));
  }
}

async function status() {
  const loggedIn = checkLogin();
  if (loggedIn) {
    console.log("✓ 已登录 (svcgov)");
    const cookie = getCookie();
    if (cookie) console.log(`  cookie: ${cookie.slice(0, 30)}...`);
  } else {
    console.log("✗ 未登录");
    console.log(`  登录: awc auth:login ${AUTH_NAME}`);
  }
}

// ── Parse args ──

const cmd = process.argv[2];

if (!cmd) {
  console.log(`Usage: demo-svcgov <command>

Commands:
  services                         List all services and their status
  logs [--service <name>] [--level <level>]   Query logs (level: info|warn|error)
  tables                           List database tables
  table <name>                     Preview table data
  status                           Check login state

Examples:
  demo-svcgov services
  demo-svcgov logs --service gateway --level error
  demo-svcgov table users
`);
  process.exit(0);
}

const cmdArgs = {};
for (let i = 3; i < process.argv.length; i += 2) {
  const key = process.argv[i]?.replace("--", "");
  const val = process.argv[i + 1];
  if (key) cmdArgs[key] = val;
}

switch (cmd) {
  case "services": services(); break;
  case "logs":     logs(cmdArgs); break;
  case "tables":   tables(); break;
  case "table":    showTable(process.argv[3]); break;
  case "status":   status(); break;
  default:
    console.error(`Unknown command: ${cmd}`);
    process.exit(1);
}
