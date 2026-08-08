#!/usr/bin/env node
// install-native-host.js — npm postinstall hook.
//
// Registers the awc native-messaging host with Chrome so the extension can
// communicate with the local host binary. This runs automatically on
// `npm install -g @agent/web-cli`. It is a best-effort, non-fatal step: if it
// fails (e.g. Chrome not installed), the user can run `awc sys:install` later.
//
// This mirrors what `awc sys:install` does, but is self-contained in Node so
// it works even before the platform binary is confirmed present.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");

const HOST_NAME = "com.awc.host";
const EXTENSION_ID = "klhhipipedegmphifibmojnhbaecjodf";

function main() {
  const installRoot = findInstallRoot();
  if (!installRoot) {
    console.warn("[awc] could not determine install root; skipping host registration.");
    console.warn("      run `awc sys:install` manually after setup.");
    return;
  }

  const hostBin = path.join(installRoot, "bin", os.platform() === "win32" ? "awc-host.exe" : "awc-host");
  if (!fs.existsSync(hostBin)) {
    console.warn(`[awc] host binary not found at ${hostBin}; skipping host registration.`);
    console.warn("      this is expected if the package was installed on an unsupported platform.");
    return;
  }

  const launcherPath = ensureLauncher(hostBin);
  const manifestPath = writeManifest(launcherPath);
  if (os.platform() === "win32") registerWindows(manifestPath);

  console.log("[awc] native-messaging host registered.");
  console.log(`[awc]   manifest:  ${manifestPath}`);
  console.log("[awc]   next step: load the extension in chrome://extensions, then run: awc sys:status");
}

// findInstallRoot walks up from this script's location to find the directory
// containing bin/ and extension/.
function findInstallRoot() {
  let dir = __dirname;
  for (let i = 0; i < 6; i++) {
    if (fs.existsSync(path.join(dir, "bin")) && fs.existsSync(path.join(dir, "extension"))) {
      return dir;
    }
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
}

// ensureLauncher creates a shell/bat launcher in ~/.awc/launchers/ that execs
// the host binary.
function ensureLauncher(hostBin) {
  const launcherDir = path.join(os.homedir(), ".awc", "launchers");
  fs.mkdirSync(launcherDir, { recursive: true });

  if (os.platform() === "win32") {
    const launcher = path.join(launcherDir, "awc-host-launcher.bat");
    fs.writeFileSync(launcher, `@echo off\r\n"${hostBin}" %*\r\n`, { mode: 0o755 });
    return launcher;
  }

  const launcher = path.join(launcherDir, "awc-host-launcher");
  const content = `#!/bin/sh\nexec '${hostBin}' "$@"\n`;
  fs.writeFileSync(launcher, content, { mode: 0o755 });
  return launcher;
}

// writeManifest writes the Chrome native-messaging manifest to the
// platform-specific location and returns its path.
function writeManifest(launcherPath) {
  const manifest = {
    name: HOST_NAME,
    description: "Agent Web CLI Native Host",
    path: launcherPath,
    type: "stdio",
    allowed_origins: [`chrome-extension://${EXTENSION_ID}/`],
  };

  const manifestPath = manifestLocation();
  fs.mkdirSync(path.dirname(manifestPath), { recursive: true });
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n", "utf8");
  return manifestPath;
}

function manifestLocation() {
  const home = os.homedir();
  switch (os.platform()) {
    case "darwin":
      return path.join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts", HOST_NAME + ".json");
    case "linux":
      return path.join(home, ".config", "google-chrome", "NativeMessagingHosts", HOST_NAME + ".json");
    case "win32":
      return path.join(home, ".awc", "host-manifest", HOST_NAME + ".json");
    default:
      throw new Error("unsupported platform: " + os.platform());
  }
}

// registerWindows writes the registry entry pointing to the manifest.
function registerWindows(manifestPath) {
  const { execSync } = require("child_process");
  const key = `HKCU\\Software\\Google\\Chrome\\NativeMessagingHosts\\${HOST_NAME}`;
  try {
    execSync(`reg add "${key}" /ve /t REG_SZ /d "${manifestPath}" /f`, { stdio: "ignore" });
  } catch (err) {
    console.warn("[awc] could not write Windows registry entry; run `awc sys:install` manually.");
  }
}

try {
  main();
} catch (err) {
  // Non-fatal: the user can run `awc sys:install` manually.
  console.warn("[awc] host registration skipped:", err.message);
}
