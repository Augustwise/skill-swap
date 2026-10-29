#!/usr/bin/env node
// Очищує всі таблиці БД та заповнює їх тестовими університетами, факультетами, навичками,
// користувачами, запитами на обмін, активними/завершеними обмінами, чатами та скаргами.
// Використання: node scripts/seed.mjs [--migrate]

import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const BACKEND = path.join(ROOT, "backend");
const IS_WIN = process.platform === "win32";

function commandExists(command) {
  const probe = spawnSync(IS_WIN ? "where" : "which", [command], { stdio: "ignore" });
  return probe.status === 0;
}

function ensureGoOnPath() {
  if (commandExists("go")) return true;
  if (!IS_WIN) return false;
  const programFiles = process.env.ProgramFiles || "C:\\Program Files";
  const programFilesX86 = process.env["ProgramFiles(x86)"] || "C:\\Program Files (x86)";
  const candidates = [
    path.join(programFiles, "Go", "bin"),
    path.join(programFilesX86, "Go", "bin"),
  ];
  const installDir = candidates.find((dir) => existsSync(path.join(dir, "go.exe")));
  if (!installDir) return false;
  process.env.PATH = `${installDir}${path.delimiter}${process.env.PATH ?? ""}`;
  return commandExists("go");
}

function runDbCommand(subcommand) {
  const result = spawnSync("go", ["run", "./cmd/db", subcommand], {
    cwd: BACKEND,
    stdio: "inherit",
    env: process.env,
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

const flags = new Set(process.argv.slice(2));
if (flags.has("-h") || flags.has("--help")) {
  console.log("Usage: node scripts/seed.mjs [--migrate]");
  process.exit(0);
}

if (!ensureGoOnPath()) {
  console.error("Go was not found on PATH. Install Go 1.26+ first.");
  process.exit(1);
}

if (flags.has("--migrate")) {
  console.log("Applying database migrations...");
  runDbCommand("up");
}

console.log("Clearing existing database records and inserting test data...");
runDbCommand("seed");

console.log(`
Тестові акаунти створено (пароль для всіх: SkillSwapDemo2026):
  • andrii.melnyk@edu.kpi.ua       — Андрій Мельник (КПІ, основний профіль із активними обмінами, запитами, чатами й відгуком)
  • olena.kovalchuk@knu.ua         — Олена Ковальчук (КНУ, активний обмін з Андрієм та вихідний запит до Софії)
  • maksym.bondarenko@edu.kpi.ua   — Максим Бондаренко (КПІ, активний обмін з Андрієм на фінальному етапі)
  • daryna.tkachenko@ukma.edu.ua   — Дарина Ткаченко (НаУКМА, вхідний запит до Андрія)
  • sofiia.shevchenko@ucu.edu.ua   — Софія Шевченко (УКУ, вхідний запит від Олени)
  • nazar.kravets@lnu.edu.ua       — Назар Кравець (ЛНУ, завершений обмін з Андрієм)
  • iryna.savchuk@lpnu.ua          — Ірина Савчук (Львівська політехніка)
  • bohdan.lysenko@edu.kpi.ua      — Богдан Лисенко (КПІ)
  • kateryna.moroz@knu.ua          — Катерина Мороз (КНУ, лише офлайн у Києві)
  • roman.oliinyk@karazin.ua       — Роман Олійник (ХНУ ім. Каразіна)
  • yuliia.pavlenko@ukma.edu.ua    — Юлія Павленко (НаУКМА)
  • dmytro.hrytsenko@khpi.edu.ua   — Дмитро Гриценко (ХПІ, тимчасово не приймає запитів)
  • victoriia.petrenko@knu.ua      — Вікторія Петренко (КНУ, прихований профіль)
  • taras.zakharchuk@edu.kpi.ua    — Тарас Захарчук (КПІ, непідтверджена пошта)
  • oleh.gavryliuk@lnu.edu.ua      — Олег Гаврилюк (ЛНУ, заблокований Андрієм, відкрита скарга)
  • admin@edu.kpi.ua               — Адміністратор SkillSwap (роль ADMIN, модерація скарг і каталогу)
`);
