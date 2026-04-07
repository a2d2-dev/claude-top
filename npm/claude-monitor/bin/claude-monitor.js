#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");

// Map Node.js platform/arch to our npm package names.
const PLATFORM_MAP = {
  "darwin-arm64":  "@a2d2/claude-monitor-darwin-arm64",
  "darwin-x64":    "@a2d2/claude-monitor-darwin-x64",
  "linux-x64":     "@a2d2/claude-monitor-linux-x64",
  "linux-arm64":   "@a2d2/claude-monitor-linux-arm64",
  "win32-x64":     "@a2d2/claude-monitor-windows-x64",
};

// Binary name inside each platform package.
const BIN_NAME =
  process.platform === "win32" ? "claude-monitor.exe" : "claude-monitor";

function findBinary() {
  const key = `${process.platform}-${process.arch}`;
  const pkgName = PLATFORM_MAP[key];

  if (!pkgName) {
    throw new Error(
      `@a2d2/claude-monitor: unsupported platform "${key}".\n` +
      `Supported: ${Object.keys(PLATFORM_MAP).join(", ")}`
    );
  }

  try {
    // Resolve the platform package relative to this file so it works when
    // installed globally or locally.
    const pkgJson = require.resolve(`${pkgName}/package.json`);
    return path.join(path.dirname(pkgJson), "bin", BIN_NAME);
  } catch {
    throw new Error(
      `@a2d2/claude-monitor: could not find platform package "${pkgName}".\n` +
      `Try reinstalling: npm install -g @a2d2/claude-monitor`
    );
  }
}

let binPath;
try {
  binPath = findBinary();
} catch (err) {
  process.stderr.write(err.message + "\n");
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
