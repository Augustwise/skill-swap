#!/usr/bin/env node
// Starts the whole local stack (Mailpit, Go API, Next.js) in one terminal with prefixed logs.
// Usage: node scripts/dev.mjs [--migrate] [--seed] [--no-mail] [--no-api] [--no-web] [--open]

import { spawn, spawnSync } from "node:child_process";
import { createWriteStream, existsSync, mkdirSync, readFileSync } from "node:fs";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const BACKEND = path.join(ROOT, "backend");
const FRONTEND = path.join(ROOT, "frontend");
const LOG_DIR = path.join(ROOT, "logs");
const IS_WIN = process.platform === "win32";
const EXE = IS_WIN ? ".exe" : "";

const HELP = `Skill Swap dev runner

Usage: node scripts/dev.mjs [options]     (or .\\dev from the repo root on Windows)

Options:
  --migrate   apply Goose migrations (go run ./cmd/db up) before starting the API
  --seed      load backend/seeds/demo.sql before starting the API
  --no-mail   do not start Mailpit
  --no-api    do not build or start the Go API
  --no-web    do not start the Next.js dev server
  --open      open http://localhost:3000 in the browser once everything is ready
  -h, --help  show this help

Logs: every line is echoed here and written to logs/dev.log plus logs/<service>.log.
Stop everything with Ctrl+C.`;

const flags = new Set(process.argv.slice(2));
if (flags.has("-h") || flags.has("--help")) {
  console.log(HELP);
  process.exit(0);
}
const known = ["--migrate", "--seed", "--no-mail", "--no-api", "--no-web", "--open"];
for (const flag of flags) {
  if (!known.includes(flag)) {
    console.error(`Unknown option: ${flag}\n\n${HELP}`);
    process.exit(2);
  }
}

// ---------- logging ----------

const color = process.stdout.isTTY && !process.env.NO_COLOR;
const paint = (code, text) => (color ? `\x1b[${code}m${text}\x1b[0m` : text);
const COLORS = { dev: "1;37", mail: "35", db: "33", build: "34", api: "36", web: "32" };
const stripAnsi = (text) => text.replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "");

mkdirSync(LOG_DIR, { recursive: true });
const combinedLog = createWriteStream(path.join(LOG_DIR, "dev.log"), { flags: "w" });
const serviceLogs = new Map();
const width = Math.max(...Object.keys(COLORS).map((name) => name.length));

function serviceLog(name) {
  if (!serviceLogs.has(name)) {
    serviceLogs.set(name, createWriteStream(path.join(LOG_DIR, `${name}.log`), { flags: "w" }));
  }
  return serviceLogs.get(name);
}

function clock() {
  const now = new Date();
  return [now.getHours(), now.getMinutes(), now.getSeconds()]
    .map((part) => String(part).padStart(2, "0"))
    .join(":");
}

function emit(name, line, level = "info") {
  const stamp = clock();
  const tag = name.padEnd(width);
  let body = line;
  if (level === "error") body = paint("31", line);
  else if (level === "warn") body = paint("33", line);
  else if (level === "ok") body = paint("32", line);
  console.log(`${paint("90", stamp)} ${paint(COLORS[name] ?? "37", tag)} │ ${body}`);
  const plain = `${stamp} ${tag} | ${stripAnsi(line)}\n`;
  combinedLog.write(plain);
  if (name !== "dev") serviceLog(name).write(plain);
}

const log = {
  info: (text) => emit("dev", text),
  ok: (text) => emit("dev", text, "ok"),
  warn: (text) => emit("dev", text, "warn"),
  error: (text) => emit("dev", text, "error"),
};

// Highlight obvious problems from child output without touching normal lines.
function levelOf(line) {
  const plain = stripAnsi(line);
  if (/\blevel=ERROR\b|\b(error|panic|fatal)\b|⨯|failed/i.test(plain)) return "error";
  if (/\blevel=WARN\b|\bwarn(ing)?\b|⚠/i.test(plain)) return "warn";
  return "info";
}

function pipeLines(stream, name) {
  let pending = "";
  stream.setEncoding("utf8");
  stream.on("data", (chunk) => {
    pending += chunk;
    const lines = pending.split(/\r?\n/);
    pending = lines.pop();
    for (const line of lines) if (line.trim()) emit(name, line, levelOf(line));
  });
  stream.on("end", () => {
    if (pending.trim()) emit(name, pending, levelOf(pending));
  });
}

// ---------- process management ----------

const children = new Map();
let shuttingDown = false;

function start(name, command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: options.cwd ?? ROOT,
    env: { ...process.env, ...options.env },
    stdio: ["ignore", "pipe", "pipe"],
    windowsHide: true,
  });
  pipeLines(child.stdout, name);
  pipeLines(child.stderr, name);
  children.set(name, child);
  child.on("error", (error) => emit(name, `failed to start: ${error.message}`, "error"));
  child.on("exit", (code, signal) => {
    children.delete(name);
    if (shuttingDown) return;
    const reason = signal ? `signal ${signal}` : `code ${code}`;
    emit(name, `exited unexpectedly (${reason}); see logs/${name}.log`, "error");
    shutdown(1);
  });
  return child;
}

// Runs a one-off command to completion, streaming its output under `name`.
function run(name, command, args, cwd) {
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      cwd,
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    pipeLines(child.stdout, name);
    pipeLines(child.stderr, name);
    child.on("error", (error) => {
      emit(name, `failed to start: ${error.message}`, "error");
      resolve(false);
    });
    child.on("exit", (code) => resolve(code === 0));
  });
}

function killTree(child) {
  if (child.exitCode !== null || child.signalCode !== null) return;
  if (IS_WIN) {
    spawnSync("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore" });
  } else {
    child.kill("SIGTERM");
  }
}

async function shutdown(code = 0, graceful = false) {
  if (shuttingDown) return;
  shuttingDown = true;
  log.info(code === 0 ? "Stopping services..." : "Stopping remaining services...");
  // On Ctrl+C the console already delivered the interrupt to every child; give them a moment
  // to shut down gracefully before force-killing whatever is left.
  const deadline = Date.now() + (graceful ? 4000 : 0);
  while (children.size > 0 && Date.now() < deadline) await sleep(100);
  for (const child of children.values()) killTree(child);
  log.info(`Logs saved to ${path.relative(ROOT, LOG_DIR)}${path.sep}`);
  await Promise.all(
    [combinedLog, ...serviceLogs.values()].map((stream) => new Promise((r) => stream.end(r))),
  );
  process.exit(code);
}

process.on("SIGINT", () => shutdown(0, true));
process.on("SIGTERM", () => shutdown(0, true));
if (IS_WIN) process.on("SIGBREAK", () => shutdown(0, true));

// ---------- helpers ----------

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function portOpen(port, host = "127.0.0.1") {
  return new Promise((resolve) => {
    const socket = net.connect({ port, host });
    socket.setTimeout(500);
    socket.once("connect", () => {
      socket.destroy();
      resolve(true);
    });
    socket.once("timeout", () => {
      socket.destroy();
      resolve(false);
    });
    socket.once("error", () => resolve(false));
  });
}

async function waitFor(check, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (shuttingDown) return false;
    if (await check()) return true;
    await sleep(300);
  }
  return false;
}

// Describes which process holds a TCP port so a conflict can be resolved quickly.
function portOwner(port) {
  if (!IS_WIN) return "";
  const netstat = spawnSync("netstat", ["-ano", "-p", "TCP"], { encoding: "utf8" });
  const row = netstat.stdout
    ?.split(/\r?\n/)
    .find((line) => /LISTENING/.test(line) && new RegExp(`:${port}\\s`).test(line));
  const pid = row?.trim().split(/\s+/).pop();
  if (!pid) return "";
  const tasklist = spawnSync("tasklist", ["/FI", `PID eq ${pid}`, "/FO", "CSV", "/NH"], {
    encoding: "utf8",
  });
  const image = tasklist.stdout?.split(",")[0]?.replace(/"/g, "").trim();
  return ` by ${image || "unknown process"} (PID ${pid}; stop it with: Stop-Process -Id ${pid})`;
}

async function requireFreePort(port, what) {
  if (await portOpen(port)) {
    log.error(`Port ${port} for ${what} is already in use${portOwner(port)}`);
    await shutdown(1);
  }
}

function commandExists(command) {
  const probe = spawnSync(IS_WIN ? "where" : "which", [command], { stdio: "ignore" });
  return probe.status === 0;
}

/**
 * Windows processes keep the PATH they inherited when they were started. If Go was
 * installed or added to PATH afterward, discover the standard installation so the
 * dev runner works without requiring a reboot or a parent terminal restart.
 */
function ensureCommandOnPath(command) {
  if (commandExists(command)) return true;
  if (!IS_WIN) return false;

  const programFiles = process.env.ProgramFiles || "C:\\Program Files";
  const programFilesX86 = process.env["ProgramFiles(x86)"] || "C:\\Program Files (x86)";
  const candidates = [
    path.join(programFiles, "Go", "bin"),
    path.join(programFilesX86, "Go", "bin"),
  ];
  const installDir = candidates.find((directory) =>
    existsSync(path.join(directory, `${command}.exe`)),
  );
  if (!installDir) return false;

  process.env.PATH = `${installDir}${path.delimiter}${process.env.PATH ?? ""}`;
  log.info(`Found ${command} at ${installDir}; added it to PATH for this run.`);
  return commandExists(command);
}

function readEnvFile(file) {
  const values = {};
  for (const raw of readFileSync(file, "utf8").split(/\r?\n/)) {
    const match = raw.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    if (!match) continue;
    values[match[1]] = match[2].trim().replace(/^(['"])(.*)\1$/, "$2");
  }
  return values;
}

function openBrowser(url) {
  const [command, args] = IS_WIN
    ? ["cmd", ["/c", "start", "", url]]
    : [process.platform === "darwin" ? "open" : "xdg-open", [url]];
  spawn(command, args, { stdio: "ignore", detached: true }).unref();
}

// ---------- main ----------

async function main() {
  const useMail = !flags.has("--no-mail");
  const useApi = !flags.has("--no-api");
  const useWeb = !flags.has("--no-web");
  const started = Date.now();
  const urls = [];

  log.info(`Skill Swap dev stack · logs → ${path.relative(ROOT, LOG_DIR)}${path.sep}`);

  // Preflight: tools, config, dependencies, and ports.
  if (useApi && !ensureCommandOnPath("go")) {
    log.error("Go is not on PATH (Go 1.26 is required for the backend).");
    return shutdown(1);
  }

  let env = {};
  if (useApi || flags.has("--migrate") || flags.has("--seed")) {
    const envFile = path.join(BACKEND, ".env");
    if (!existsSync(envFile)) {
      log.error("backend/.env is missing; create it with the values from backend/README.md.");
      return shutdown(1);
    }
    env = readEnvFile(envFile);
    let dbHost = "";
    try {
      dbHost = new URL(env.DATABASE_URL ?? "").hostname;
    } catch {
      log.error("DATABASE_URL in backend/.env is not a valid URL.");
      return shutdown(1);
    }
    log.info(`Database host: ${dbHost}`);
    if (["localhost", "127.0.0.1", "::1"].includes(dbHost)) {
      const dbPort = Number(new URL(env.DATABASE_URL).port || 5432);
      if (!(await portOpen(dbPort))) {
        log.error(`Local PostgreSQL is not listening on port ${dbPort}.`);
        log.error("Start it as administrator, e.g.: Start-Service postgresql-x64-18");
        return shutdown(1);
      }
    }
  }

  const apiAddr = env.API_ADDR || "127.0.0.1:8080";
  const apiPort = Number(apiAddr.split(":").pop());
  const smtpPort = Number((env.SMTP_ADDR || "127.0.0.1:1025").split(":").pop());

  if (useApi) await requireFreePort(apiPort, "the API");
  if (useWeb) await requireFreePort(3000, "the frontend");
  if (shuttingDown) return;

  if (useWeb && !existsSync(path.join(FRONTEND, "node_modules", "next"))) {
    log.warn("frontend/node_modules is missing; running npm ci...");
    const npm = IS_WIN ? "npm.cmd" : "npm";
    const ok = await new Promise((resolve) => {
      const child = spawn(npm, ["ci"], { cwd: FRONTEND, stdio: "inherit", shell: IS_WIN });
      child.on("exit", (code) => resolve(code === 0));
    });
    if (!ok) {
      log.error("npm ci failed.");
      return shutdown(1);
    }
  }

  // 1. Mailpit (SMTP capture for verification and password-reset emails).
  if (useMail) {
    const mailpit = path.join(BACKEND, ".local", "mailpit", `mailpit${EXE}`);
    if (await portOpen(smtpPort)) {
      log.warn(`Port ${smtpPort} is already in use; assuming Mailpit is already running.`);
      urls.push(["Mailpit", "http://127.0.0.1:8025 (external)"]);
    } else if (!existsSync(mailpit)) {
      log.warn("Mailpit not found in backend/.local/mailpit; emails will fail to send.");
      log.warn(
        "Download instructions: backend/README.md → 'Run Mailpit on Windows without Docker'.",
      );
    } else {
      start("mail", mailpit, [], {
        cwd: BACKEND,
        env: { MP_UI_BIND_ADDR: "127.0.0.1:8025", MP_SMTP_BIND_ADDR: `127.0.0.1:${smtpPort}` },
      });
      if (await waitFor(() => portOpen(smtpPort), 10_000)) {
        urls.push(["Mailpit", "http://127.0.0.1:8025"]);
      } else if (!shuttingDown) {
        log.error("Mailpit did not open its SMTP port within 10s.");
        return shutdown(1);
      }
    }
  }

  // 2. Optional database maintenance.
  for (const [flag, command] of [
    ["--migrate", "up"],
    ["--seed", "seed"],
  ]) {
    if (!flags.has(flag) || shuttingDown) continue;
    log.info(`Running go run ./cmd/db ${command}...`);
    if (!(await run("db", "go", ["run", "./cmd/db", command], BACKEND))) {
      log.error(`go run ./cmd/db ${command} failed; see logs/db.log`);
      return shutdown(1);
    }
  }

  // 3. Go API: build first so compile errors are reported clearly and the process tree is simple.
  if (useApi && !shuttingDown) {
    const binary = path.join(BACKEND, ".local", "dev", `api${EXE}`);
    log.info("Building the API...");
    const buildStart = Date.now();
    if (!(await run("build", "go", ["build", "-o", binary, "./cmd/api"], BACKEND))) {
      log.error("go build failed; fix the errors above and restart.");
      return shutdown(1);
    }
    log.info(`API built in ${((Date.now() - buildStart) / 1000).toFixed(1)}s`);
    start("api", binary, [], { cwd: BACKEND });

    const readyUrl = `http://${apiAddr}/api/v1/ready`;
    let lastProblem = "";
    const ready = await waitFor(async () => {
      try {
        const response = await fetch(readyUrl, { signal: AbortSignal.timeout(3000) });
        if (response.ok) return true;
        lastProblem = `HTTP ${response.status}: ${(await response.text()).trim()}`;
      } catch (error) {
        lastProblem = error.cause?.code ?? error.message;
      }
      return false;
    }, 30_000);
    if (shuttingDown) return;
    if (ready) {
      log.ok("API is ready (database reachable).");
    } else {
      log.warn(`API is not ready after 30s (${lastProblem}); continuing, check the api logs.`);
    }
    urls.push(["API", `http://${apiAddr}/api/v1`]);
  }

  // 4. Next.js dev server (proxies /api/v1 to the Go API).
  if (useWeb && !shuttingDown) {
    const nextBin = path.join(FRONTEND, "node_modules", "next", "dist", "bin", "next");
    start("web", process.execPath, [nextBin, "dev"], {
      cwd: FRONTEND,
      env: color ? { FORCE_COLOR: "1" } : { NO_COLOR: "1" },
    });
    if (!(await waitFor(() => portOpen(3000), 60_000))) {
      if (!shuttingDown) log.warn("Next.js did not open port 3000 within 60s; check the web logs.");
    } else {
      urls.push(["Frontend", "http://localhost:3000"]);
    }
  }

  if (shuttingDown) return;
  const seconds = ((Date.now() - started) / 1000).toFixed(1);
  log.ok(`Everything is up in ${seconds}s. Press Ctrl+C to stop.`);
  const labelWidth = Math.max(...urls.map(([label]) => label.length));
  for (const [label, url] of urls) log.ok(`  ${label.padEnd(labelWidth)}  ${url}`);
  if (flags.has("--open") && useWeb) openBrowser("http://localhost:3000");
}

main().catch(async (error) => {
  log.error(error.stack ?? String(error));
  await shutdown(1);
});
