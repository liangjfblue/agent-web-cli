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

const { execSync } = require("child_process");

const BASE_URL = "http://localhost:3000";
const AUTH_NAME = "demo-admin";

// ── Read cookie from Chrome via awc ──
function getCookie() {
  try {
    return execSync(`awc cookies:get --url ${BASE_URL} --header`, {
      encoding: "utf8",
      timeout: 10000,
    }).trim();
  } catch (err) {
    console.error("✗ 无法读取 cookie — awc 是否已启动？");
    console.error("  运行: awc sys:status 检查连接");
    process.exit(1);
  }
}

// ── Check login state ──
function checkLogin() {
  try {
    const out = execSync(`awc auth:login ${AUTH_NAME} --check`, {
      encoding: "utf8",
      timeout: 10000,
    }).trim();
    return out.includes("logged in");
  } catch {
    return false;
  }
}

// ── Call API with cookie ──
async function callAPI(path) {
  const cookie = getCookie();
  const resp = await fetch(`${BASE_URL}${path}`, {
    headers: { Cookie: cookie },
  });

  if (resp.status === 401) {
    console.error("✗ cookie 失效或未登录");
    console.error(`  重新登录: awc auth:login ${AUTH_NAME}`);
    process.exit(1);
  }

  return resp.json();
}

// ── Commands ──

async function dashboard() {
  if (!checkLogin()) {
    console.error("✗ 未登录，请先登录:");
    console.error(`  awc auth:login ${AUTH_NAME}`);
    process.exit(1);
  }

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
  if (!checkLogin()) {
    console.error("✗ 未登录，请先登录:");
    console.error(`  awc auth:login ${AUTH_NAME}`);
    process.exit(1);
  }

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
  const loggedIn = checkLogin();
  if (loggedIn) {
    console.log("✓ 已登录 (demo-admin)");
    // Also show the cookie exists
    const cookie = getCookie();
    if (cookie) {
      console.log(`  cookie: ${cookie.slice(0, 30)}...`);
    }
  } else {
    console.log("✗ 未登录");
    console.log(`  登录: awc auth:login ${AUTH_NAME}`);
  }
}

// ── Main ──

const cmd = process.argv[2];
const commands = { dashboard, users, status };

if (!cmd || !commands[cmd]) {
  console.log("Usage: node cli.js <command>");
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
