import setup from "../frontend/tests/acceptance/local-stack.mjs";

process.env.E2E_MAILPIT = "1";
const cleanup = await setup();
console.log(`Skill Swap: http://localhost:3000\nMailpit: ${process.env.E2E_MAILPIT_URL}`);
console.log("Isolated temporary database. Press Ctrl+C after all participants finish.");
await new Promise((resolve) => {
  process.once("SIGINT", resolve);
  process.once("SIGTERM", resolve);
});
await cleanup();
