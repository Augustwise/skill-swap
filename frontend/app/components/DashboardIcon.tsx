import Image from "next/image";

// Keep the original Figma geometry for each icon and color variant.
const assets = {
  dashboard: ["icon-dashboard", 20],
  "nav-search": ["icon-search1", 20],
  "nav-sparkles": ["icon-sparkles", 20],
  inbox: ["icon-inbox", 20],
  "nav-swap": ["icon-swap", 20],
  "nav-chat": ["icon-chat", 20],
  calendar: ["icon-calendar", 20],
  settings: ["icon-settings", 20],
  logout: ["icon-logout", 20],
  search: ["icon-search", 18],
  bell: ["icon-bell", 20],
  "notification-dot": ["ellipse4", 9],
  "chevron-down": ["icon-chevron-down", 16],
  "hero-sparkles": ["icon-sparkles1", 24],
  "arrow-right": ["icon-arrow-right", 17],
  "stat-swap": ["icon-swap1", 20],
  "stat-sparkles": ["icon-sparkles2", 20],
  "stat-clock": ["icon-clock", 20],
  award: ["icon-award", 20],
  trending: ["icon-trending", 13],
  "chevron-right": ["icon-chevron-right", 15],
  "match-sparkles": ["icon-sparkles3", 14],
  star: ["icon-star", 15],
  chat: ["icon-chat1", 18],
  swap: ["icon-swap2", 15],
  clock: ["icon-clock1", 13],
  video: ["icon-video", 13],
  pin: ["icon-pin", 13],
  "teach-dot": ["ellipse", 7],
  "learn-dot": ["ellipse1", 7],
  "pending-dot": ["ellipse2", 6],
  "active-dot": ["ellipse3", 6],
} as const;

export type DashboardIconName = keyof typeof assets;

export function DashboardIcon({ name }: { name: DashboardIconName }) {
  const [file, size] = assets[name];
  return (
    <Image
      className="dashboard-icon"
      src={`/figma/main/${file}.svg`}
      alt=""
      width={size}
      height={size}
    />
  );
}
