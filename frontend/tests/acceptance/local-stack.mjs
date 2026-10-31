import { spawn } from "node:child_process";
import { closeSync, existsSync, openSync, readFileSync } from "node:fs";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontend = fileURLToPath(new URL("../../", import.meta.url));
const backend = path.resolve(frontend, "../backend");
const suffix = process.platform === "win32" ? ".exe" : "";

async function freePort(port = 0) {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", resolve);
  });
  const selected = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return selected;
}

export default async function setup() {
  const bundled = path.join(backend, ".local/postgres-tools/pgsql/bin");
  const pgBin =
    process.env.E2E_PG_BIN || process.env.PG_BIN || (existsSync(bundled) ? bundled : "");
  const pg = (name) => (pgBin ? path.join(pgBin, `${name}${suffix}`) : name);
  const children = [];
  let postgresStarted = false;
  let directory;
  let logs = "";
  async function command(file, args, options = {}) {
    const filename = path.join(directory, "setup.log");
    const output = openSync(filename, "a");
    try {
      await new Promise((resolve, reject) => {
        const child = spawn(file, args, {
          windowsHide: true,
          stdio: ["ignore", output, output],
          ...options,
        });
        const timeout = setTimeout(() => {
          child.kill();
          reject(new Error(`${path.basename(file)} timed out`));
        }, 60_000);
        child.once("error", (error) => {
          clearTimeout(timeout);
          reject(error);
        });
        child.once("exit", (code) => {
          clearTimeout(timeout);
          if (code === 0) resolve();
          else
            reject(
              new Error(
                `${path.basename(file)} failed (${code}):\n${readFileSync(filename, "utf8").slice(-8000)}`,
              ),
            );
        });
      });
    } finally {
      closeSync(output);
    }
  }

  async function cleanup() {
    for (const child of children.reverse()) {
      if (!child.pid || child.exitCode !== null || child.signalCode !== null) continue;
      const exited = new Promise((resolve) => child.once("exit", resolve));
      child.kill();
      await exited;
    }
    if (postgresStarted && existsSync(path.join(directory, "postgres/postmaster.pid"))) {
      await command(pg("pg_ctl"), [
        "-D",
        path.join(directory, "postgres"),
        "-m",
        "fast",
        "-w",
        "stop",
      ]);
    }
    if (directory) {
      const resolved = path.resolve(directory);
      const parent = path.join(backend, ".local");
      if (
        path.dirname(resolved) !== parent ||
        !path.basename(resolved).startsWith("browser-acceptance-")
      )
        throw new Error("Unexpected acceptance workspace; cleanup refused");
      await rm(resolved, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
  }

  function start(file, args, options) {
    const child = spawn(file, args, {
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"],
      ...options,
    });
    child.on("error", (error) => {
      child.startupError = error;
      logs += `${error.message}\n`;
    });
    for (const stream of [child.stdout, child.stderr]) {
      stream.on("data", (chunk) => {
        logs = (logs + chunk).slice(-12_000);
      });
    }
    children.push(child);
  }

  async function ready(url) {
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      if (children.some((child) => child.startupError || child.exitCode !== null)) break;
      try {
        if ((await fetch(url, { signal: AbortSignal.timeout(1000) })).ok) return;
      } catch {
        /* Wait for the owned process to listen. */
      }
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    throw new Error(`Local acceptance server did not start: ${url}\n${logs}`);
  }

  try {
    await freePort(3000);
    await freePort(8080);
    const port = await freePort();
    await mkdir(path.join(backend, ".local"), { recursive: true });
    directory = await mkdtemp(path.join(backend, ".local/browser-acceptance-"));
    const pgData = path.join(directory, "postgres");
    await command(pg("initdb"), [
      "-D",
      pgData,
      "-U",
      "postgres",
      "-A",
      "trust",
      "--encoding=UTF8",
      "--locale-provider=icu",
      "--icu-locale=uk-UA",
    ]);
    postgresStarted = true;
    await command(pg("pg_ctl"), [
      "-D",
      pgData,
      "-l",
      path.join(directory, "postgres.log"),
      "-o",
      `-h 127.0.0.1 -p ${port}`,
      "-w",
      "start",
    ]);
    await command(pg("createdb"), [
      "-h",
      "127.0.0.1",
      "-p",
      String(port),
      "-U",
      "postgres",
      "skillswap_browser_acceptance",
    ]);
    const env = {
      ...process.env,
      DATABASE_URL: `postgres://postgres@127.0.0.1:${port}/skillswap_browser_acceptance?sslmode=disable`,
      API_ADDR: "127.0.0.1:8080",
      FRONTEND_ORIGIN: "http://localhost:3000",
      SMTP_ADDR: "127.0.0.1:1",
      SMTP_USERNAME: "",
      SMTP_PASSWORD: "",
      MAIL_FROM: "no-reply@students.example.test",
      ALLOWED_EMAIL_DOMAINS: "students.example.test",
    };
    const api = path.join(directory, `api${suffix}`);
    const database = path.join(directory, `db${suffix}`);
    await command("go", ["build", "-o", api, "./cmd/api"], { cwd: backend });
    await command("go", ["build", "-o", database, "./cmd/db"], { cwd: backend });
    await command(database, ["up"], { cwd: backend, env });
    await command(database, ["seed-test"], { cwd: backend, env });
    start(api, [], { cwd: backend, env });
    await ready("http://127.0.0.1:8080/api/v1/health");
    start(
      process.execPath,
      ["node_modules/next/dist/bin/next", "start", "--hostname", "127.0.0.1", "--port", "3000"],
      { cwd: frontend },
    );
    await ready("http://localhost:3000/login");
    console.log("Acceptance stack: production Next.js + real Go API + isolated PostgreSQL");
    return cleanup;
  } catch (error) {
    await cleanup();
    throw error;
  }
}
