#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

function fail(message) {
  console.error(message);
  process.exit(2);
}

function parseArgs(argv) {
  const options = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) fail(`unexpected argument: ${arg}`);
    const key = arg.slice(2);
    const value = argv[i + 1];
    if (!value || value.startsWith("--")) fail(`missing value for --${key}`);
    options[key] = value;
    i += 1;
  }
  return options;
}

function requireSlug(value, flag) {
  if (!value || !/^[a-z0-9][a-z0-9-]*$/.test(value)) {
    fail(`--${flag} must use lowercase letters, digits, and hyphens`);
  }
  return value;
}

function requireUrl(value, flag) {
  try {
    const url = new URL(value);
    if (!/^https?:$/.test(url.protocol)) throw new Error();
  } catch {
    fail(`--${flag} must be an absolute HTTP(S) URL`);
  }
  return value.replace(/\/$/, "");
}

function replaceAll(value, replacements) {
  let result = value;
  for (const [marker, replacement] of Object.entries(replacements)) {
    result = result.split(marker).join(replacement);
  }
  return result;
}

function copyTemplate(source, destination, replacements) {
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const targetName = replaceAll(entry.name.replace(/\.tmpl$/, ""), replacements);
    const sourcePath = path.join(source, entry.name);
    const targetPath = path.join(destination, targetName);
    if (entry.isDirectory()) {
      fs.mkdirSync(targetPath, { recursive: true });
      copyTemplate(sourcePath, targetPath, replacements);
      continue;
    }
    const content = fs.readFileSync(sourcePath, "utf8");
    fs.writeFileSync(targetPath, replaceAll(content, replacements), "utf8");
  }
}

const options = parseArgs(process.argv.slice(2));
const required = ["name", "command", "base-url", "login-url", "cookie-url", "cookie-name", "output"];
for (const key of required) {
  if (!options[key]) fail(`missing required option: --${key}`);
}

const projectName = requireSlug(options.name, "name");
const commandName = requireSlug(options.command, "command");
const authName = requireSlug(options["auth-name"] || projectName, "auth-name");
const skillName = requireSlug(options["skill-name"] || `${commandName}-admin`, "skill-name");
const displayName = options["display-name"] || projectName;
if (/["\r\n]/.test(displayName)) fail("--display-name cannot contain quotes or newlines");
if (!/^[A-Za-z0-9._-]+$/.test(options["cookie-name"])) fail("--cookie-name contains unsupported characters");

const output = path.resolve(options.output);
if (fs.existsSync(output)) fail(`output already exists: ${output}`);

const envPrefix = commandName.replace(/-/g, "_").toUpperCase();
const replacements = {
  "__PROJECT_NAME__": projectName,
  "__COMMAND_NAME__": commandName,
  "__DISPLAY_NAME__": displayName,
  "__AUTH_NAME__": authName,
  "__SKILL_NAME__": skillName,
  "__ENV_PREFIX__": envPrefix,
  "__BASE_URL__": requireUrl(options["base-url"], "base-url"),
  "__LOGIN_URL__": requireUrl(options["login-url"], "login-url"),
  "__COOKIE_URL__": requireUrl(options["cookie-url"], "cookie-url"),
  "__COOKIE_NAME__": options["cookie-name"],
};

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const template = path.resolve(scriptDir, "..", "assets", "node-cli-template");
fs.mkdirSync(output, { recursive: false });
copyTemplate(template, output, replacements);
try {
  fs.chmodSync(path.join(output, "cli.js"), 0o755);
} catch {
  // Windows does not use the executable bit.
}

console.log(`Created ${projectName} at ${output}`);
console.log("Next: implement commands.js, complete skill/SKILL.md, and run npm test.");
