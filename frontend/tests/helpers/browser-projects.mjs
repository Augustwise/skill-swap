export function browserProjects() {
  const projects = [
    { name: "chrome", use: { browserName: "chromium", channel: "chrome" } },
    { name: "edge", use: { browserName: "chromium", channel: "msedge" } },
    { name: "firefox", use: { browserName: "firefox", channel: undefined } },
  ];
  for (const [name, browserName] of [
    ["chrome", "chromium"],
    ["edge", "chromium"],
    ["firefox", "firefox"],
  ]) {
    const executablePath = process.env[`PLAYWRIGHT_${name.toUpperCase()}_PREVIOUS`];
    if (executablePath) {
      projects.push({
        name: `${name}-previous`,
        use: { browserName, channel: undefined, launchOptions: { executablePath } },
      });
    }
  }
  return projects;
}
