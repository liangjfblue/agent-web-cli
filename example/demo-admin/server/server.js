// Demo admin panel — a minimal Express server with login + 2 authenticated APIs.
//
// Run: npm install && node server.js
// Then open http://localhost:3000/login in Chrome and log in as admin/admin123.
//
// The session cookie is HttpOnly (page JS can't read it), but awc can read it
// via chrome.cookies API — that's the whole point of this demo.

const express = require("express");
const crypto = require("crypto");

const app = express();
const PORT = 3000;

// Parse form bodies.
app.use(express.urlencoded({ extended: true }));
app.use(express.json());

// ── Session store (in-memory, resets on restart) ──
const sessions = new Map();

function authMiddleware(req, res, next) {
  const token = req.headers.cookie
    ? req.headers.cookie.split(";").map((s) => s.trim()).find((s) => s.startsWith("session="))
    : null;
  const session = token ? sessions.get(token.split("=")[1]) : null;

  if (!session) {
    // For API routes, return 401 JSON. For pages, redirect to login.
    if (req.path.startsWith("/api/")) {
      return res.status(401).json({ error: "unauthorized", message: "session expired or missing" });
    }
    return res.redirect("/login");
  }
  req.user = session;
  next();
}

// ── Login page ──
app.get("/login", (req, res) => {
  res.send(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Demo Admin</title>
<style>
  body { font-family: system-ui; display:flex; justify-content:center; align-items:center; height:100vh; margin:0; background:#f5f5f5; }
  .card { background:#fff; padding:2rem; border-radius:8px; box-shadow:0 2px 8px rgba(0,0,0,.1); width:320px; }
  h1 { margin:0 0 1.5rem; font-size:1.25rem; }
  input { display:block; width:100%; padding:.6rem; margin:.5rem 0; border:1px solid #ddd; border-radius:4px; box-sizing:border-box; }
  button { width:100%; padding:.6rem; background:#2563eb; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:1rem; margin-top:.5rem; }
  button:hover { background:#1d4ed8; }
  .hint { color:#888; font-size:.8rem; margin-top:1rem; text-align:center; }
</style></head><body>
<form class="card" method="POST" action="/login">
  <h1>🔧 Demo Admin</h1>
  <input name="username" placeholder="用户名" value="admin" autofocus>
  <input name="password" type="password" placeholder="密码" value="admin123">
  <button type="submit">登录</button>
  <p class="hint">admin / admin123</p>
</form>
</body></html>`);
});

// ── Login handler ──
app.post("/login", (req, res) => {
  const { username, password } = req.body;
  if (username === "admin" && password === "admin123") {
    const token = crypto.randomUUID();
    sessions.set(token, { username, loginAt: Date.now() });
    // HttpOnly: page JS can't read it, but awc (chrome.cookies API) can.
    res.setHeader("Set-Cookie", `session=${token}; HttpOnly; Path=/; Max-Age=86400`);
    res.redirect("/");
  } else {
    res.redirect("/login?error=1");
  }
});

// ── Logout ──
app.get("/logout", (req, res) => {
  const token = req.headers.cookie
    ? req.headers.cookie.split(";").map((s) => s.trim()).find((s) => s.startsWith("session="))
    : null;
  if (token) sessions.delete(token.split("=")[1]);
  // Clear the browser cookie so --check (and the user's UI) actually reflect
  // the logged-out state. Without this the cookie lingers and awc keeps
  // reporting "logged in" even though the server session is gone.
  res.setHeader("Set-Cookie", "session=; HttpOnly; Path=/; Max-Age=0");
  res.redirect("/login");
});

// ── Dashboard page ──
app.get("/", authMiddleware, (req, res) => {
  res.send(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Dashboard · Demo Admin</title>
<style>
  body { font-family: system-ui; margin:2rem; background:#f5f5f5; }
  .header { display:flex; justify-content:space-between; align-items:center; margin-bottom:2rem; }
  h1 { margin:0; }
  .cards { display:grid; grid-template-columns:repeat(3,1fr); gap:1rem; }
  .card { background:#fff; padding:1.5rem; border-radius:8px; text-align:center; }
  .card .num { font-size:2rem; font-weight:bold; color:#2563eb; }
  .card .label { color:#666; margin-top:.5rem; }
  a { color:#2563eb; text-decoration:none; }
</style></head><body>
<div class="header">
  <h1>📊 Dashboard</h1>
  <span>Welcome, ${req.user.username} · <a href="/logout">退出</a></span>
</div>
<div class="cards">
  <div class="card"><div class="num">42</div><div class="label">今日订单</div></div>
  <div class="card"><div class="num">¥99,800</div><div class="label">今日收入</div></div>
  <div class="card"><div class="num">3</div><div class="label">待处理</div></div>
</div>
<p style="margin-top:2rem;color:#888">API: <a href="/api/dashboard">/api/dashboard</a> · <a href="/api/users">/api/users</a></p>
</body></html>`);
});

// ── Authenticated APIs ──
app.get("/api/dashboard", authMiddleware, (req, res) => {
  res.json({
    orders: 42,
    revenue: 99800,
    pending: 3,
    updatedAt: new Date().toISOString(),
    user: req.user.username,
  });
});

app.get("/api/users", authMiddleware, (req, res) => {
  res.json({
    users: [
      { id: 1, name: "张三", role: "admin", email: "zhangsan@example.com" },
      { id: 2, name: "李四", role: "editor", email: "lisi@example.com" },
      { id: 3, name: "王五", role: "viewer", email: "wangwu@example.com" },
    ],
    total: 3,
  });
});

app.listen(PORT, () => {
  console.log(`Demo admin running at http://localhost:${PORT}`);
  console.log(`Login: http://localhost:${PORT}/login (admin / admin123)`);
});
