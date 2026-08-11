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

const { execFileSync } = require("child_process");

const BASE_URL = "http://127.0.0.1:3001";
const AUTH_NAME = "svcgov";

function acquireSession() {
  try {
    const output = execFileSync("awc", [
      "session:acquire", AUTH_NAME,
      "--url", BASE_URL,
      "--json",
    ], {
      encoding: "utf8", timeout: 10000, stdio: ["ignore", "pipe", "pipe"],
    });
    return JSON.parse(output).data;
  } catch (err) {
    if (err.status === 10) {
      console.error("not logged in");
      console.error(`  run: awc session:acquire ${AUTH_NAME} --url ${BASE_URL} --interactive --json`);
    } else {
      console.error("unable to acquire a browser session; run: awc sys:status");
    }
    process.exit(1);
  }
}

async function callAPI(path) {
  const session = acquireSession();
  const resp = await fetch(`${BASE_URL}${path}`, { headers: { Cookie: session.cookieHeader } });
  if (resp.status === 401 || resp.status === 403) {
    console.error("browser credentials were rejected by the API");
    console.error(`  run: awc session:acquire ${AUTH_NAME} --url ${BASE_URL} --interactive --refresh --json`);
    process.exit(1);
  }
  return resp.json();
}

// ── Commands ──

async function services() {
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
  const data = await callAPI("/api/database/tables");
  console.log(`🗄 Database Tables (${data.tables.length})`);
  console.log("─".repeat(40));
  console.log("  NAME        ROWS  ENGINE");
  for (const t of data.tables) {
    console.log(`  ${t.name.padEnd(12)} ${String(t.rows).padEnd(6)} ${t.engine}`);
  }
}

async function showTable(name) {
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
  const session = acquireSession();
  console.log(`session available (${session.profileId || "legacy profile"})`);
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
