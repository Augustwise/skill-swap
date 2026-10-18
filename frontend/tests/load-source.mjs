import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

const cache = new Map();

// Compile the small API modules in memory; no browser or test framework is needed.
// The cache keeps one instance per module, so error classes match across imports.
export function loadSource(filename) {
  const resolved = path.resolve(filename);
  if (cache.has(resolved)) return cache.get(resolved).exports;
  const source = ts.transpileModule(fs.readFileSync(resolved, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  });
  const compiled = { exports: {} };
  cache.set(resolved, compiled);
  new Function("require", "module", "exports", source.outputText)(
    (name) => loadSource(path.resolve(path.dirname(resolved), `${name}.ts`)),
    compiled,
    compiled.exports,
  );
  return compiled.exports;
}
