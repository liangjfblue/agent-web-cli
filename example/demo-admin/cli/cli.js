#!/usr/bin/env node
// demo-cli — a business CLI that reads cookies from Chrome via awc,
// then calls the demo admin panel's APIs.
//
// This demonstrates the real-world pattern:
//   1. awc reads the login cookie (HttpOnly — page JS can't, but awc can)
//   2. the cookie is passed as a header to fetch()
//   3. if the API rejects it, tell the user to re-login via awc
//
// Usage:
//   node cli.js dashboard    # show dashboard stats
//   node cli.js users        # list users
//   node cli.js status       # check if logged in

const { execFileSync } = require("child_process");

const BASE_URL = "http://localhost:3000";
const AUTH_NAME = "demo-admin";

function acquireSession() {
  try {
    const output = execFileSync("awc", [
      "session:acquire", AUTH_NAME,
      "--url", BASE_URL,
      "--json",
    ], {
      encoding: "utf8",
      timeout: 10000,
      stdio: ["ignore", "pipe", "pipe"],
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
  const resp = await fetch(`${BASE_URL}${path}`, {
    headers: { Cookie: session.cookieHeader },
  });

  if (resp.status === 401 || resp.status === 403) {
    console.error("browser credentials were rejected by the API");
    console.error(`  run: awc session:acquire ${AUTH_NAME} --url ${BASE_URL} --interactive --refresh --json`);
    process.exit(1);
  }

  return resp.json();
}

// ── Commands ──

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
  for (const u of data.users) {
    console.log(
      `  ${String(u.id).padEnd(4)}${u.name.padEnd(8)}${u.role.padEnd(9)}${u.email}`
    );
  }
}

async function status() {
  const session = acquireSession();
  console.log(`session available (${session.profileId || "legacy profile"})`);
}

// ── Main ──

const cmd = process.argv[2];
const commands = { dashboard, users, status };

if (!cmd || !commands[cmd]) {
  console.log("Usage: demo-admin <command>");
  console.log("");
  console.log("Commands:");
  console.log("  dashboard    Show dashboard stats (orders, revenue, pending)");
  console.log("  users        List all users");
  console.log("  status       Check login state");
  process.exit(1);
}

commands[cmd]().catch((err) => {
  console.error("Error:", err.message);
  process.exit(1);
});
