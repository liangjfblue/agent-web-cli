// Service Governance Platform — a simulated sysop/consul-like admin panel.
//
// Run: npm install && node server.js
// Login at http://127.0.0.1:3001/login (admin / admin123)
//
// All data is in-memory mock data — no external database needed.

const express = require("express");
const crypto = require("crypto");

const app = express();
const PORT = 3001;

app.use(express.urlencoded({ extended: true }));
app.use(express.json());

// ════════════════════════════════════════════════════════════════════════
// Mock Data
// ════════════════════════════════════════════════════════════════════════

const SERVICES = [
  { name: "gateway",            status: "healthy",   instances: 3, version: "v2.1.0",  cpu: 12.5, memory: "256Mi" },
  { name: "user-service",       status: "healthy",   instances: 2, version: "v1.8.3",  cpu: 8.2,  memory: "512Mi" },
  { name: "order-service",      status: "degraded",  instances: 2, version: "v3.0.1",  cpu: 45.7, memory: "1Gi" },
  { name: "payment-service",    status: "healthy",   instances: 1, version: "v1.2.0",  cpu: 5.1,  memory: "256Mi" },
  { name: "notification",       status: "down",      instances: 0, version: "v0.9.2",  cpu: 0,    memory: "0" },
];

const LOG_LEVELS = ["info", "warn", "error"];
const LOG_MESSAGES = {
  "gateway":       ["Request routed to user-service", "Upstream timeout after 5000ms", "Connection pool exhausted", "Rate limit exceeded for IP 10.0.0.5", "Circuit breaker opened for order-service"],
  "user-service":  ["User login: admin", "JWT token refreshed", "Database query slow (1.2s)", "Failed login attempt: user not found", "Profile updated for uid=324950039"],
  "order-service": ["Order #12345 created", "Order processing started", "OOM killed, restarting pod", "Database connection refused", "Order batch completed: 50 items"],
  "payment-service":["Payment received: ¥299.00", "Refund processed: ¥50.00", "Payment gateway responded slowly", "Transaction verified", "Webhook delivered to merchant"],
  "notification":  ["Email sent to user", "Push notification delivered", "Service is DOWN — no instances", "SMS queue overflow", "Retry exhausted for email delivery"],
};

const LOGS = [];
function generateLogs() {
  const now = Date.now();
  let id = 1;
  for (const svc of SERVICES) {
    const msgs = LOG_MESSAGES[svc.name] || [];
    msgs.forEach((msg, i) => {
      const level = msg.includes("DOWN") || msg.includes("OOM") || msg.includes("refused") || msg.includes("exhausted") ? "error"
        : msg.includes("slow") || msg.includes("slowly") || msg.includes("overflow") || msg.includes("exceeded") ? "warn"
        : "info";
      LOGS.push({
        id: id++,
        ts: new Date(now - (i + 1) * 60000 * (svc.name === "gateway" ? 3 : 7)).toISOString(),
        service: svc.name,
        level,
        message: msg,
      });
    });
  }
}
generateLogs();

const DATABASE = {
  tables: [
    { name: "users",    rows: 10, engine: "InnoDB" },
    { name: "orders",   rows: 10, engine: "InnoDB" },
    { name: "products", rows: 10, engine: "InnoDB" },
  ],
  data: {
    users: {
      columns: ["id", "username", "email", "role", "created_at"],
      rows: [
        [1, "admin",    "admin@example.com",     "admin",  "2024-01-01"],
        [2, "zhangsan", "zhangsan@example.com",  "editor", "2024-02-15"],
        [3, "lisi",     "lisi@example.com",      "viewer", "2024-03-20"],
        [4, "wangwu",   "wangwu@example.com",    "editor", "2024-04-10"],
        [5, "zhaoliu",  "zhaoliu@example.com",   "viewer", "2024-05-05"],
      ],
    },
    orders: {
      columns: ["id", "user_id", "product", "amount", "status", "created_at"],
      rows: [
        [1, 1, "Premium Plan",  29900, "paid",    "2024-06-01"],
        [2, 2, "Basic Plan",     9900, "paid",    "2024-06-02"],
        [3, 3, "Enterprise",   99900, "pending",  "2024-06-03"],
        [4, 1, "Add-on Storage", 4900, "paid",    "2024-06-05"],
        [5, 4, "Basic Plan",     9900, "failed",  "2024-06-06"],
      ],
    },
    products: {
      columns: ["id", "name", "price", "stock", "category"],
      rows: [
        [1, "Basic Plan",      9900, 999, "subscription"],
        [2, "Premium Plan",   29900, 500, "subscription"],
        [3, "Enterprise",     99900,  50, "subscription"],
        [4, "Add-on Storage",  4900, 200, "addon"],
        [5, "Add-on Users",    2900, 200, "addon"],
      ],
    },
  },
};

// ════════════════════════════════════════════════════════════════════════
// Auth
// ════════════════════════════════════════════════════════════════════════

const sessions = new Map();

function authMiddleware(req, res, next) {
  const token = req.headers.cookie
    ? req.headers.cookie.split(";").map(s => s.trim()).find(s => s.startsWith("session="))
    : null;
  const session = token ? sessions.get(token.split("=")[1]) : null;
  if (!session) {
    if (req.path.startsWith("/api/")) return res.status(401).json({ error: "unauthorized" });
    return res.redirect("/login");
  }
  req.user = session;
  next();
}

// ════════════════════════════════════════════════════════════════════════
// Pages
// ════════════════════════════════════════════════════════════════════════

app.get("/login", (req, res) => {
  res.send(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Service Governance</title>
<style>
body{font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#0f172a;color:#e2e8f0}
.card{background:#1e293b;padding:2rem;border-radius:8px;box-shadow:0 4px 12px rgba(0,0,0,.3);width:340px}
h1{margin:0 0 1.5rem;font-size:1.25rem;text-align:center}
input{display:block;width:100%;padding:.6rem;margin:.5rem 0;background:#334155;border:1px solid #475569;border-radius:4px;color:#e2e8f0;box-sizing:border-box}
button{width:100%;padding:.6rem;background:#3b82f6;color:#fff;border:none;border-radius:4px;cursor:pointer;font-size:1rem;margin-top:.5rem}
button:hover{background:#2563eb}
.hint{color:#64748b;font-size:.8rem;margin-top:1rem;text-align:center}
</style></head><body>
<form class="card" method="POST" action="/login">
<h1>🛡 Service Governance</h1>
<input name="username" placeholder="用户名" value="admin" autofocus>
<input name="password" type="password" placeholder="密码" value="admin123">
<button type="submit">登录</button>
<p class="hint">admin / admin123</p>
</form></body></html>`);
});

app.post("/login", (req, res) => {
  const { username, password } = req.body;
  if (username === "admin" && password === "admin123") {
    const token = crypto.randomUUID();
    sessions.set(token, { username, loginAt: Date.now() });
    res.setHeader("Set-Cookie", `session=${token}; HttpOnly; Path=/; Max-Age=86400`);
    res.redirect("/");
  } else {
    res.redirect("/login?error=1");
  }
});

app.get("/logout", (req, res) => {
  const token = req.headers.cookie?.split(";").map(s=>s.trim()).find(s=>s.startsWith("session="));
  if (token) sessions.delete(token.split("=")[1]);
  // Clear the browser cookie so --check (and the user's UI) actually reflect
  // the logged-out state. Without this the cookie lingers and awc keeps
  // reporting "logged in" even though the server session is gone.
  res.setHeader("Set-Cookie", "session=; HttpOnly; Path=/; Max-Age=0");
  res.redirect("/login");
});

// Layout helper for authenticated pages.
function pageWrap(title, user, content) {
  return `<!DOCTYPE html><html><head><meta charset="utf-8"><title>${title} · Service Governance</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui;background:#f1f5f9;color:#1e293b}
.nav{background:#1e293b;color:#e2e8f0;padding:1rem 2rem;display:flex;justify-content:space-between;align-items:center}
.nav a{color:#93c5fd;text-decoration:none;margin-right:1rem}
.nav a:hover{color:#bfdbfe}
.container{max-width:960px;margin:2rem auto;padding:0 1rem}
table{width:100%;border-collapse:collapse;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.1)}
th{background:#334155;color:#e2e8f0;text-align:left;padding:.6rem 1rem;font-size:.85rem}
td{padding:.6rem 1rem;border-bottom:1px solid #e2e8f0;font-size:.9rem}
.badge{display:inline-block;padding:.15rem .5rem;border-radius:4px;font-size:.75rem;font-weight:600}
.badge.healthy{background:#d1fae5;color:#065f46}
.badge.degraded{background:#fef3c7;color:#92400e}
.badge.down{background:#fee2e2;color:#991b1b}
.badge.error{background:#fee2e2;color:#991b1b}
.badge.warn{background:#fef3c7;color:#92400e}
.badge.info{background:#dbeafe;color:#1e40af}
h2{margin-bottom:1rem}
</style></head><body>
<div class="nav">
<div><strong>🛡 Service Governance</strong>
&nbsp;&nbsp;<a href="/">服务</a><a href="/logs">日志</a><a href="/database">数据库</a></div>
<div>${user.username} · <a href="/logout" style="color:#93c5fd">退出</a></div>
</div>
<div class="container">${content}</div>
</body></html>`;
}

// ── Services page ──
app.get("/", authMiddleware, (req, res) => {
  const rows = SERVICES.map(s => `<tr>
    <td><strong>${s.name}</strong></td>
    <td><span class="badge ${s.status}">${s.status}</span></td>
    <td>${s.instances}</td>
    <td>${s.version}</td>
    <td>${s.cpu}%</td>
    <td>${s.memory}</td>
  </tr>`).join("");
  res.send(pageWrap("服务列表", req.user, `<h2>服务列表 (${SERVICES.length})</h2><table>
    <tr><th>NAME</th><th>STATUS</th><th>INSTANCES</th><th>VERSION</th><th>CPU</th><th>MEMORY</th></tr>
    ${rows}</table>`));
});

// ── Logs page ──
app.get("/logs", authMiddleware, (req, res) => {
  const svc = req.query.service || "";
  const level = req.query.level || "";
  let logs = LOGS;
  if (svc) logs = logs.filter(l => l.service === svc);
  if (level) logs = logs.filter(l => l.level === level);
  const rows = logs.slice(-50).reverse().map(l => `<tr>
    <td style="font-family:monospace;font-size:.8rem;color:#64748b">${l.ts.slice(11,19)}</td>
    <td>${l.service}</td>
    <td><span class="badge ${l.level}">${l.level}</span></td>
    <td>${l.message}</td>
  </tr>`).join("");
  res.send(pageWrap("日志", req.user, `<h2>日志查询${svc?" · "+svc:""}${level?" · "+level:""}</h2><table>
    <tr><th>TIME</th><th>SERVICE</th><th>LEVEL</th><th>MESSAGE</th></tr>
    ${rows}</table>`));
});

// ── Database page ──
app.get("/database", authMiddleware, (req, res) => {
  const table = req.query.t;
  if (table && DATABASE.data[table]) {
    const t = DATABASE.data[table];
    const rows = t.rows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join("")}</tr>`).join("");
    const cols = t.columns.map(c => `<th>${c}</th>`).join("");
    res.send(pageWrap(`数据库 · ${table}`, req.user,
      `<h2><a href="/database">数据库</a> / ${table} (${t.rows.length} rows)</h2><table><tr>${cols}</tr>${rows}</table>`));
    return;
  }
  const rows = DATABASE.tables.map(t => `<tr>
    <td><a href="/database?t=${t.name}"><strong>${t.name}</strong></a></td>
    <td>${t.rows}</td><td>${t.engine}</td>
  </tr>`).join("");
  res.send(pageWrap("数据库", req.user, `<h2>数据库表</h2><table>
    <tr><th>TABLE</th><th>ROWS</th><th>ENGINE</th></tr>${rows}</table>`));
});

// ════════════════════════════════════════════════════════════════════════
// Authenticated APIs
// ════════════════════════════════════════════════════════════════════════

app.get("/api/services", authMiddleware, (req, res) => res.json({ services: SERVICES }));

app.get("/api/logs", authMiddleware, (req, res) => {
  let logs = LOGS;
  if (req.query.service) logs = logs.filter(l => l.service === req.query.service);
  if (req.query.level) logs = logs.filter(l => l.level === req.query.level);
  res.json({ logs: logs.slice(-100).reverse(), total: logs.length });
});

app.get("/api/database/tables", authMiddleware, (req, res) => res.json({ tables: DATABASE.tables }));

app.get("/api/database/:table", authMiddleware, (req, res) => {
  const t = DATABASE.data[req.params.table];
  if (!t) return res.status(404).json({ error: "table not found" });
  res.json({ table: req.params.table, columns: t.columns, rows: t.rows });
});

app.listen(PORT, () => {
  console.log(`Service Governance running at http://127.0.0.1:${PORT}`);
  console.log(`Login: http://127.0.0.1:${PORT}/login (admin / admin123)`);
});
