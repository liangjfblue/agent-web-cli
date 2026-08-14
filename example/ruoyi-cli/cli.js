#!/usr/bin/env node
"use strict";

const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const BASE_URL = process.env.RUOYI_BASE_URL || "https://vue.ruoyi.vip";
const AUTH_NAME = process.env.RUOYI_AUTH_NAME || "ruoyi";
const LOGIN_TIMEOUT_MS = 315_000;
const sourceAwc = path.resolve(__dirname, "..", "..", "bin", process.platform === "win32" ? "awc.exe" : "awc");
const AWC_BIN = process.env.AWC_BIN || (fs.existsSync(sourceAwc) ? sourceAwc : "awc");

class CliError extends Error {
  constructor(message, exitCode = 1) {
    super(message);
    this.exitCode = exitCode;
  }
}

function acquireToken() {
  let output;
  try {
    output = execFileSync(AWC_BIN, [
      "session:acquire",
      AUTH_NAME,
      "--url",
      BASE_URL,
      "--json",
    ], {
      encoding: "utf8",
      timeout: 15_000,
      stdio: ["ignore", "pipe", "pipe"],
    });
  } catch (error) {
    if (error.status === 10) {
      throw new CliError(
        `login required; run: ${AWC_BIN} session:acquire ${AUTH_NAME} --url ${BASE_URL} --interactive --json`,
        10
      );
    }
    const hint = error.status === 2
      ? "auth profile or arguments invalid; run: awc auth:list"
      : "run: awc sys:status";
    throw new CliError(`unable to acquire the Chrome session (awc exit ${error.status ?? "unknown"}); ${hint}`, error.status || 1);
  }

  let session;
  try {
    session = JSON.parse(output);
  } catch {
    throw new CliError("awc returned invalid JSON");
  }
  const cookieHeader = session?.data?.cookieHeader || "";
  const tokenPart = cookieHeader
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith("Admin-Token="));
  if (!tokenPart) {
    throw new CliError("Chrome session does not contain Admin-Token", 10);
  }
  return tokenPart.slice("Admin-Token=".length);
}

function buildLoginArgs(refresh) {
  const args = [
    "session:acquire",
    AUTH_NAME,
    "--url",
    BASE_URL,
    "--interactive",
  ];
  if (refresh) args.push("--refresh");
  args.push("--json");
  return args;
}

function login(options) {
  validateOptions(options, ["refresh"]);
  try {
    const output = execFileSync(AWC_BIN, buildLoginArgs(Boolean(options.refresh)), {
      encoding: "utf8",
      timeout: LOGIN_TIMEOUT_MS,
      stdio: ["ignore", "pipe", "inherit"],
    });
    const session = JSON.parse(output);
    if (!session?.ok) throw new CliError("browser login did not produce a usable session", 10);
  } catch (error) {
    if (error instanceof CliError) throw error;
    if (error.status === 11) throw new CliError("browser login timed out", 11);
    if (error.status === 20) throw new CliError("awc host is unavailable; run: awc sys:status", 20);
    throw new CliError("unable to complete browser login", error.status || 1);
  }
  console.log("Browser login is available.");
}

async function request(path, query = {}) {
  const url = new URL(`/prod-api${path}`, BASE_URL);
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
  }

  const response = await fetch(url, {
    method: "GET",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${acquireToken()}`,
    },
  });
  let body;
  try {
    body = await response.json();
  } catch {
    throw new CliError(`RuoYi returned a non-JSON response (HTTP ${response.status})`);
  }
  if (response.status === 401 || response.status === 403 || body.code === 401) {
    throw new CliError(
      `browser credentials were rejected; run: ${AWC_BIN} session:acquire ${AUTH_NAME} --url ${BASE_URL} --interactive --refresh --json`,
      10
    );
  }
  if (!response.ok || body.code !== 200) {
    throw new CliError(body.msg || `RuoYi request failed (HTTP ${response.status})`);
  }
  return body;
}

function parseArgs(argv) {
  const command = argv[0] || "help";
  const positionals = [];
  const options = {};
  for (let i = 1; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) {
      positionals.push(arg);
      continue;
    }
    const key = arg.slice(2);
    if (key === "json" || key === "refresh") {
      options[key] = true;
      continue;
    }
    const value = argv[i + 1];
    if (!value || value.startsWith("--")) throw new CliError(`missing value for --${key}`, 2);
    options[key] = value;
    i += 1;
  }
  return { command, positionals, options };
}

function validateOptions(options, allowed) {
  for (const key of Object.keys(options)) {
    if (!allowed.includes(key)) throw new CliError(`unknown option: --${key}`, 2);
  }
}

function positiveInt(value, name, fallback, max) {
  if (value === undefined) return fallback;
  if (!/^\d+$/.test(value) || Number(value) < 1) throw new CliError(`--${name} must be a positive integer`, 2);
  if (max !== undefined && Number(value) > max) throw new CliError(`--${name} must not exceed ${max}`, 2);
  return Number(value);
}

function requirePositionals(positionals, count, usage) {
  if (positionals.length !== count) throw new CliError(`usage: ${usage}`, 2);
}

function statusCode(value) {
  if (value === undefined) return undefined;
  const normalized = String(value).toLowerCase();
  if (["0", "enabled", "normal"].includes(normalized)) return "0";
  if (["1", "disabled"].includes(normalized)) return "1";
  throw new CliError("--status must be enabled, disabled, 0, or 1", 2);
}

function statusLabel(value) {
  return String(value) === "0" ? "enabled" : "disabled";
}

function dataScopeLabel(value) {
  return ({
    "1": "all",
    "2": "custom",
    "3": "department",
    "4": "department-and-children",
    "5": "self",
  })[String(value)] || String(value ?? "");
}

const BUSINESS_TYPES = {
  "0": "other",
  "1": "create",
  "2": "update",
  "3": "delete",
  "4": "grant",
  "5": "export",
  "6": "import",
  "7": "force-logout",
  "8": "gen-code",
  "9": "clean",
};

function logStatusCode(value) {
  if (value === undefined) return undefined;
  const normalized = String(value).toLowerCase();
  if (["0", "success", "ok"].includes(normalized)) return "0";
  if (["1", "failed", "fail", "error"].includes(normalized)) return "1";
  throw new CliError("--status must be success, failed, 0, or 1", 2);
}

function logStatusLabel(value) {
  return String(value) === "0" ? "success" : "failed";
}

function businessTypeCode(value) {
  if (value === undefined) return undefined;
  const normalized = String(value).toLowerCase();
  if (/^[0-9]$/.test(normalized)) return normalized;
  const entry = Object.entries(BUSINESS_TYPES).find(([, label]) => label === normalized);
  if (entry) return entry[0];
  throw new CliError(`--business-type must be 0-9 or one of: ${Object.values(BUSINESS_TYPES).join(", ")}`, 2);
}

function businessTypeLabel(value) {
  return BUSINESS_TYPES[String(value)] ?? String(value ?? "");
}

function printJSON(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

function printTable(columns, rows) {
  const widths = columns.map((column, index) => Math.max(
    column.length,
    ...rows.map((row) => String(row[index] ?? "").length)
  ));
  const render = (row) => row.map((cell, index) => String(cell ?? "").padEnd(widths[index])).join("  ");
  console.log(render(columns));
  console.log(widths.map((width) => "-".repeat(width)).join("  "));
  for (const row of rows) console.log(render(row));
}

function printDetails(entries) {
  const width = Math.max(...entries.map(([key]) => key.length));
  for (const [key, value] of entries) console.log(`${key.padEnd(width)}  ${value ?? ""}`);
}

async function userList(options) {
  validateOptions(options, ["name", "phone", "status", "dept-id", "page", "page-size", "json"]);
  const body = await request("/system/user/list", {
    pageNum: positiveInt(options.page, "page", 1),
    pageSize: positiveInt(options["page-size"], "page-size", 20, 100),
    userName: options.name,
    phonenumber: options.phone,
    status: statusCode(options.status),
    deptId: options["dept-id"],
  });
  if (options.json) return printJSON(body);
  console.log(`Users: ${body.total}`);
  printTable(
    ["ID", "USERNAME", "NICKNAME", "DEPARTMENT", "PHONE", "STATUS", "CREATED"],
    body.rows.map((user) => [
      user.userId,
      user.userName,
      user.nickName,
      user.dept?.deptName || "",
      user.phonenumber || "",
      statusLabel(user.status),
      user.createTime || "",
    ])
  );
}

async function userGet(id, options) {
  validateOptions(options, ["json"]);
  if (!/^\d+$/.test(id || "")) throw new CliError("user:get requires a numeric user id", 2);
  const body = await request(`/system/user/${encodeURIComponent(id)}`);
  if (options.json) return printJSON(body);
  const user = body.data;
  printDetails([
    ["ID", user.userId],
    ["Username", user.userName],
    ["Nickname", user.nickName],
    ["Department", user.dept?.deptName || ""],
    ["Email", user.email || ""],
    ["Phone", user.phonenumber || ""],
    ["Status", statusLabel(user.status)],
    ["Login IP", user.loginIp || ""],
    ["Last login", user.loginDate || ""],
    ["Created", user.createTime || ""],
    ["Roles", (body.roles || []).filter((role) => body.roleIds?.includes(role.roleId)).map((role) => role.roleName).join(", ")],
  ]);
}

async function roleList(options) {
  validateOptions(options, ["name", "key", "status", "page", "page-size", "json"]);
  const body = await request("/system/role/list", {
    pageNum: positiveInt(options.page, "page", 1),
    pageSize: positiveInt(options["page-size"], "page-size", 20, 100),
    roleName: options.name,
    roleKey: options.key,
    status: statusCode(options.status),
  });
  if (options.json) return printJSON(body);
  console.log(`Roles: ${body.total}`);
  printTable(
    ["ID", "NAME", "KEY", "SORT", "DATA_SCOPE", "STATUS", "CREATED"],
    body.rows.map((role) => [
      role.roleId,
      role.roleName,
      role.roleKey,
      role.roleSort,
      dataScopeLabel(role.dataScope),
      statusLabel(role.status),
      role.createTime || "",
    ])
  );
}

async function roleGet(id, options) {
  validateOptions(options, ["json"]);
  if (!/^\d+$/.test(id || "")) throw new CliError("role:get requires a numeric role id", 2);
  const body = await request(`/system/role/${encodeURIComponent(id)}`);
  if (options.json) return printJSON(body);
  const role = body.data;
  printDetails([
    ["ID", role.roleId],
    ["Name", role.roleName],
    ["Key", role.roleKey],
    ["Sort", role.roleSort],
    ["Data scope", dataScopeLabel(role.dataScope)],
    ["Status", statusLabel(role.status)],
    ["Created", role.createTime || ""],
    ["Remark", role.remark || ""],
  ]);
}

async function operlogList(options) {
  validateOptions(options, ["title", "oper-name", "business-type", "status", "page", "page-size", "json"]);
  const body = await request("/monitor/operlog/list", {
    pageNum: positiveInt(options.page, "page", 1),
    pageSize: positiveInt(options["page-size"], "page-size", 20, 100),
    title: options.title,
    businessType: businessTypeCode(options["business-type"]),
    operName: options["oper-name"],
    status: logStatusCode(options.status),
  });
  if (options.json) return printJSON(body);
  console.log(`Operation logs: ${body.total}`);
  printTable(
    ["ID", "TITLE", "TYPE", "OPER_NAME", "IP", "STATUS", "COST_MS", "TIME"],
    body.rows.map((log) => [
      log.operId,
      log.title,
      businessTypeLabel(log.businessType),
      log.operName,
      log.operIp || "",
      logStatusLabel(log.status),
      log.costTime ?? "",
      log.operTime || "",
    ])
  );
}

async function loginlogList(options) {
  validateOptions(options, ["user", "ip", "status", "page", "page-size", "json"]);
  const body = await request("/monitor/logininfor/list", {
    pageNum: positiveInt(options.page, "page", 1),
    pageSize: positiveInt(options["page-size"], "page-size", 20, 100),
    userName: options.user,
    ipaddr: options.ip,
    status: logStatusCode(options.status),
  });
  if (options.json) return printJSON(body);
  console.log(`Login logs: ${body.total}`);
  printTable(
    ["ID", "USER", "IP", "LOCATION", "BROWSER", "STATUS", "TIME"],
    body.rows.map((log) => [
      log.infoId,
      log.userName,
      log.ipaddr || "",
      log.loginLocation || "",
      log.browser || "",
      logStatusLabel(log.status),
      log.loginTime || "",
    ])
  );
}

async function sessionStatus(options) {
  validateOptions(options, ["json"]);
  const body = await request("/getInfo");
  const result = {
    ok: true,
    userName: body.user?.userName,
    nickName: body.user?.nickName,
    roles: body.roles || [],
  };
  if (options.json) return printJSON(result);
  printDetails([
    ["Session", "available"],
    ["Username", result.userName],
    ["Nickname", result.nickName],
    ["Roles", result.roles.join(", ")],
  ]);
}

function printHelp() {
  console.log(`Usage: ruoyi <command> [options]

Authentication:
  login [--refresh]

Read-only commands:
  status [--json]
  user:list [--name <name>] [--phone <phone>] [--status <enabled|disabled>]
            [--dept-id <id>] [--page <n>] [--page-size <n>] [--json]
  user:get <user-id> [--json]
  role:list [--name <name>] [--key <key>] [--status <enabled|disabled>]
            [--page <n>] [--page-size <n>] [--json]
  role:get <role-id> [--json]
  operlog:list [--title <title>] [--oper-name <name>] [--business-type <0-9>]
               [--status <success|failed>] [--page <n>] [--page-size <n>] [--json]
  loginlog:list [--user <name>] [--ip <ip>] [--status <success|failed>]
                [--page <n>] [--page-size <n>] [--json]

Log rows contain the full detail fields (parameters, results, error messages);
use --json on the list commands to read them. business-type: 0 other, 1 create,
2 update, 3 delete, 4 grant, 5 export, 6 import, 7 force-logout, 8 gen-code,
9 clean.

This CLI intentionally has no create, update, delete, export, authorization,
trigger, or sync commands.`);
}

async function main() {
  const { command, positionals, options } = parseArgs(process.argv.slice(2));
  switch (command) {
    case "login":
      requirePositionals(positionals, 0, "ruoyi login [--refresh]");
      return login(options);
    case "status":
      requirePositionals(positionals, 0, "ruoyi status [--json]");
      return sessionStatus(options);
    case "user:list":
      requirePositionals(positionals, 0, "ruoyi user:list [options]");
      return userList(options);
    case "user:get":
      requirePositionals(positionals, 1, "ruoyi user:get <user-id> [--json]");
      return userGet(positionals[0], options);
    case "role:list":
      requirePositionals(positionals, 0, "ruoyi role:list [options]");
      return roleList(options);
    case "role:get":
      requirePositionals(positionals, 1, "ruoyi role:get <role-id> [--json]");
      return roleGet(positionals[0], options);
    case "operlog:list":
      requirePositionals(positionals, 0, "ruoyi operlog:list [options]");
      return operlogList(options);
    case "loginlog:list":
      requirePositionals(positionals, 0, "ruoyi loginlog:list [options]");
      return loginlogList(options);
    case "help":
    case "--help":
    case "-h": printHelp(); return;
    default: throw new CliError(`unknown command: ${command}`, 2);
  }
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error.message || String(error));
    process.exit(error.exitCode || 1);
  });
}

module.exports = { buildLoginArgs, LOGIN_TIMEOUT_MS, parseArgs };
