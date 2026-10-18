import Image from "next/image";
import Link from "next/link";
import type { CSSProperties } from "react";
import { navigation, pageShells, type AppPage, type AppSection } from "./AppNavigation";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { Logo } from "./Logo";

type Size = number | string;

export function Skeleton({
  kind = "text",
  width,
  height,
  dark = false,
}: {
  kind?:
    | "text"
    | "small"
    | "title"
    | "avatar-32"
    | "avatar-40"
    | "avatar-48"
    | "icon"
    | "pill"
    | "chip"
    | "button"
    | "field"
    | "block";
  width?: Size;
  height?: Size;
  dark?: boolean;
}) {
  const style: CSSProperties = {};
  if (width !== undefined) style.width = width;
  if (height !== undefined) style.height = height;
  return (
    <span
      className={`skeleton skeleton--${kind} ${dark ? "skeleton--dark" : ""}`}
      style={style}
      aria-hidden="true"
    />
  );
}

function Lines({ widths, kind = "text" }: { widths: Size[]; kind?: "text" | "small" }) {
  return (
    <span className="skeleton-stack">
      {widths.map((width, index) => (
        <Skeleton kind={kind} width={width} key={index} />
      ))}
    </span>
  );
}

function Chips({ widths }: { widths: number[] }) {
  return (
    <span className="skeleton-chips">
      {widths.map((width, index) => (
        <Skeleton kind="chip" width={width} key={index} />
      ))}
    </span>
  );
}

function PersonRow({ avatar = 48, badge = true }: { avatar?: 32 | 40 | 48; badge?: boolean }) {
  return (
    <span className="skeleton-row">
      <Skeleton kind={`avatar-${avatar}`} />
      <Lines widths={["58%", "78%"]} />
      {badge && <Skeleton kind="pill" width={92} />}
    </span>
  );
}

export function SkeletonCard({ variant = "match" }: { variant?: "match" | "request" }) {
  return (
    <div className="dashboard-match skeleton-card" aria-hidden="true">
      <PersonRow />
      {variant === "request" ? (
        <>
          <Skeleton kind="small" width="52%" />
          <span className="skeleton-directions">
            <span className="skeleton-direction">
              <Skeleton kind="small" width="55%" />
              <Skeleton kind="chip" width={84} />
              <Skeleton kind="small" width="70%" />
            </span>
            <span className="skeleton-direction">
              <Skeleton kind="small" width="65%" />
              <Skeleton kind="chip" width={96} />
              <Skeleton kind="small" width="70%" />
            </span>
          </span>
          <Skeleton kind="block" height={52} />
        </>
      ) : (
        <>
          <Skeleton kind="small" width="40%" />
          <Skeleton kind="small" width="26%" />
          <Chips widths={[96, 78]} />
          <Skeleton kind="small" width="34%" />
          <Chips widths={[88]} />
          <Skeleton kind="small" width="62%" />
        </>
      )}
      <span className="skeleton-actions">
        <Skeleton kind="button" />
        {variant === "request" && <Skeleton kind="button" />}
      </span>
    </div>
  );
}

export function SkeletonCardGrid({
  count,
  variant = "match",
  columns = "auto",
}: {
  count: number;
  variant?: "match" | "request";
  columns?: "auto" | "two";
}) {
  return (
    <div
      className={
        columns === "two"
          ? "dashboard-match-grid"
          : `discovery-grid ${variant === "request" ? "exchange-grid" : ""}`
      }
      aria-hidden="true"
    >
      {Array.from({ length: count }, (_, index) => (
        <SkeletonCard variant={variant} key={index} />
      ))}
    </div>
  );
}

export function SkeletonPanelRows({
  count,
  variant,
}: {
  count: number;
  variant: "request" | "exchange";
}) {
  return (
    <div className="skeleton-panel-rows" aria-hidden="true">
      {Array.from({ length: count }, (_, index) =>
        variant === "request" ? (
          <span className="skeleton-panel-row" key={index}>
            <PersonRow avatar={32} badge />
            <Chips widths={[86, 72]} />
            <span className="skeleton-actions">
              <Skeleton kind="button" />
              <Skeleton kind="button" />
            </span>
          </span>
        ) : (
          <span className="skeleton-panel-row skeleton-panel-row--exchange" key={index}>
            <PersonRow avatar={40} badge={false} />
            <Chips widths={[90, 110]} />
            <Skeleton kind="small" width="40%" />
          </span>
        ),
      )}
    </div>
  );
}

function SkeletonSection({ lines = 3, title = "34%" }: { lines?: number; title?: Size }) {
  return (
    <div className="account-card skeleton-section" aria-hidden="true">
      <Skeleton kind="title" width={title} />
      {Array.from({ length: lines }, (_, index) => (
        <Skeleton width={index === lines - 1 ? "64%" : "100%"} key={index} />
      ))}
    </div>
  );
}

export function SkeletonDetail() {
  return (
    <div className="profile-columns" aria-hidden="true">
      <div className="profile-sections">
        <div className="account-card skeleton-section">
          <Skeleton kind="small" width="32%" />
          <PersonRow />
        </div>
        <div className="account-card skeleton-section">
          <Skeleton kind="title" width="30%" />
          <span className="skeleton-directions">
            <span className="skeleton-direction">
              <Skeleton kind="small" width="45%" />
              <Skeleton kind="chip" width={90} />
              <Skeleton kind="small" width="80%" />
              <Skeleton kind="small" width="70%" />
            </span>
            <span className="skeleton-direction">
              <Skeleton kind="small" width="55%" />
              <Skeleton kind="chip" width={104} />
              <Skeleton kind="small" width="80%" />
              <Skeleton kind="small" width="70%" />
            </span>
          </span>
          <span className="skeleton-facts">
            <Skeleton kind="block" height={58} />
            <Skeleton kind="block" height={58} />
            <Skeleton kind="block" height={58} />
          </span>
        </div>
        <SkeletonSection lines={2} title="26%" />
      </div>
      <div className="profile-rail">
        <div className="account-card skeleton-section">
          <Skeleton kind="title" width="40%" />
          <Skeleton kind="pill" width={88} />
          <Skeleton width="100%" />
          <Skeleton width="72%" />
          <span className="skeleton-actions">
            <Skeleton kind="button" />
            <Skeleton kind="button" />
          </span>
        </div>
        <SkeletonSection lines={3} title="48%" />
      </div>
    </div>
  );
}

function SkeletonFields({ count }: { count: number }) {
  return (
    <span className="skeleton-fields">
      {Array.from({ length: count }, (_, index) => (
        <span className="skeleton-field" key={index}>
          <Skeleton kind="small" width={index % 2 ? "28%" : "36%"} />
          <Skeleton kind="field" />
        </span>
      ))}
    </span>
  );
}

function SkeletonProfile() {
  return (
    <>
      <span className="skeleton-tabs" aria-hidden="true">
        <Skeleton width={128} />
        <Skeleton width={84} />
      </span>
      <div className="profile-columns" aria-hidden="true">
        <div className="profile-sections">
          <div className="account-card skeleton-section">
            <Skeleton kind="title" width="36%" />
            <Skeleton kind="small" width="52%" />
            <SkeletonFields count={6} />
          </div>
          <SkeletonSection lines={2} title="30%" />
        </div>
        <div className="profile-rail">
          <div className="account-card skeleton-section">
            <PersonRow badge={false} />
            <Chips widths={[84, 72, 96]} />
            <Skeleton width="80%" />
          </div>
          <SkeletonSection lines={4} title="44%" />
        </div>
      </div>
    </>
  );
}

function SkeletonDashboard() {
  return (
    <>
      <span className="skeleton-hero" aria-hidden="true" />
      <div className="dashboard-stats" aria-hidden="true">
        {Array.from({ length: 4 }, (_, index) => (
          <div className="dashboard-stat skeleton-card" key={index}>
            <Skeleton kind="icon" />
            <Lines widths={[32, "70%"]} />
          </div>
        ))}
      </div>
      <div className="dashboard-columns" aria-hidden="true">
        <div className="dashboard-left">
          <span className="skeleton-heading">
            <Skeleton kind="title" width={200} />
            <Skeleton kind="small" width={90} />
          </span>
          <SkeletonCardGrid count={2} columns="two" />
          <div className="dashboard-panel skeleton-card">
            <Skeleton kind="title" width={160} />
            <SkeletonPanelRows count={1} variant="exchange" />
          </div>
        </div>
        <div className="dashboard-right">
          <div className="dashboard-panel skeleton-card">
            <Skeleton kind="title" width={150} />
            <Skeleton kind="block" height={140} />
          </div>
          <div className="dashboard-panel skeleton-card">
            <Skeleton kind="title" width={130} />
            <SkeletonPanelRows count={1} variant="request" />
          </div>
        </div>
      </div>
    </>
  );
}

function SkeletonHeading() {
  return (
    <span className="skeleton-heading" aria-hidden="true">
      <Skeleton kind="title" width={150} />
      <Skeleton kind="small" width={120} />
    </span>
  );
}

function SkeletonSearch() {
  return (
    <>
      <div className="account-card skeleton-section" aria-hidden="true">
        <span className="skeleton-row skeleton-row--end">
          <span className="skeleton-field">
            <Skeleton kind="small" width={120} />
            <Skeleton kind="field" />
          </span>
          <Skeleton kind="field" width={96} />
        </span>
        <Skeleton kind="small" width="60%" />
        <span className="skeleton-filters">
          {Array.from({ length: 4 }, (_, index) => (
            <span className="skeleton-field" key={index}>
              <Skeleton kind="small" width="50%" />
              <Skeleton kind="field" />
            </span>
          ))}
        </span>
      </div>
      <SkeletonHeading />
      <SkeletonCardGrid count={6} />
    </>
  );
}

function SkeletonMatches() {
  return (
    <>
      <span className="skeleton-notice" aria-hidden="true">
        <Skeleton width="100%" />
        <Skeleton width="76%" />
      </span>
      <SkeletonHeading />
      <SkeletonCardGrid count={4} />
    </>
  );
}

function SkeletonRequests() {
  return (
    <>
      <span className="skeleton-tabs" aria-hidden="true">
        <Skeleton width={72} />
        <Skeleton width={86} />
      </span>
      <Skeleton kind="small" width="54%" />
      <span className="skeleton-chips" aria-hidden="true">
        {[44, 82, 82, 86, 90].map((width, index) => (
          <Skeleton kind="pill" width={width} height={32} key={index} />
        ))}
      </span>
      <SkeletonHeading />
      <SkeletonCardGrid count={2} variant="request" />
    </>
  );
}

function SkeletonMessages() {
  return (
    <div className="chat-layout" aria-hidden="true">
      <div className="account-card chat-sidebar">
        <Skeleton kind="title" width={110} />
        {Array.from({ length: 4 }, (_, index) => (
          <span className="skeleton-row" key={index}>
            <Skeleton kind="avatar-40" />
            <Lines widths={["60%", "45%", "85%"]} />
          </span>
        ))}
      </div>
      <div className="account-card chat-panel">
        <span className="chat-skeleton">
          <PersonRow avatar={40} badge={false} />
          <span className="chat-skeleton__messages">
            <Skeleton kind="block" width="46%" height={44} />
            <Skeleton kind="block" width="38%" height={44} />
            <Skeleton kind="block" width="52%" height={62} />
          </span>
          <Skeleton kind="field" />
        </span>
      </div>
    </div>
  );
}

function SkeletonNotifications() {
  return (
    <>
      <span className="skeleton-tabs" aria-hidden="true">
        <Skeleton width={40} />
        <Skeleton width={110} />
      </span>
      <div className="account-card skeleton-section" aria-hidden="true">
        <Skeleton kind="title" width="30%" />
        {Array.from({ length: 5 }, (_, index) => (
          <span className="skeleton-row" key={index}>
            <Skeleton kind="icon" width={36} height={36} />
            <Lines widths={["50%", "85%"]} />
          </span>
        ))}
      </div>
    </>
  );
}

function SkeletonDetailPage() {
  return (
    <>
      <Skeleton kind="small" width={150} />
      <SkeletonDetail />
    </>
  );
}

const layouts = {
  dashboard: SkeletonDashboard,
  search: SkeletonSearch,
  matches: SkeletonMatches,
  requests: SkeletonRequests,
  detail: SkeletonDetailPage,
  messages: SkeletonMessages,
  notifications: SkeletonNotifications,
  profile: SkeletonProfile,
};

// The signed-in layout with the real navigation and title while the page data loads.
export function AppSkeleton({ page, section }: { page: AppPage; section?: AppSection }) {
  const shell = pageShells[page];
  const active = section ?? shell.section;
  const Content = layouts[shell.layout];
  return (
    <div className="dashboard-shell">
      <aside className="dashboard-sidebar" aria-label="Бічна панель">
        <div className="dashboard-sidebar__brand">
          <Logo variant="app" />
        </div>
        <nav className="dashboard-nav" aria-label="Основна навігація">
          {navigation.map((item) =>
            item.href ? (
              <Link
                className={`dashboard-nav-item ${item.section === active ? "dashboard-nav-item--active" : ""}`}
                href={item.href}
                aria-current={item.section === active ? "page" : undefined}
                aria-label={item.label}
                key={item.label}
              >
                <Icon name={item.icon} />
                <span>{item.label}</span>
              </Link>
            ) : (
              <span className="dashboard-nav-item" aria-hidden="true" key={item.label}>
                <Icon name={item.icon} />
                <span>{item.label}</span>
              </span>
            ),
          )}
        </nav>
        <div className="dashboard-sidebar__footer">
          <Link
            className={`dashboard-nav-item ${active === "profile" ? "dashboard-nav-item--active" : ""}`}
            href="/profile"
            aria-label="Мій профіль"
          >
            <Icon name="settings" />
            <span>Мій профіль</span>
          </Link>
        </div>
      </aside>
      <div className="dashboard-main">
        <header className="dashboard-topbar">
          <div className="dashboard-topbar__title">
            <h1>{shell.title}</h1>
            <p>{shell.subtitle}</p>
          </div>
          <span className="dashboard-search skeleton-search" aria-hidden="true">
            <Image src="/figma/main/icon-search.svg" alt="" width={18} height={18} />
            <Skeleton width="60%" />
          </span>
          <span className="dashboard-notifications" aria-hidden="true">
            <Icon name="bell" />
          </span>
          <span className="account-menu" aria-hidden="true">
            <span className="dashboard-user">
              <Skeleton kind="avatar-40" />
              <span className="dashboard-user__identity skeleton-stack">
                <Skeleton width={110} />
                <Skeleton kind="small" width={140} />
              </span>
            </span>
          </span>
        </header>
        <main className="dashboard-content skeleton-content" aria-busy="true">
          <p className="sr-only" role="status">
            Завантажуємо {shell.subject}…
          </p>
          <Content />
        </main>
      </div>
    </div>
  );
}

export function OnboardingSkeleton() {
  return (
    <main className="onboarding-page" aria-busy="true">
      <header className="onboarding-header">
        <span className="onboarding-logo">
          <Image src="/figma/onboarding/mark.svg" alt="" width={30} height={30} />
          <span>SkillSwap</span>
        </span>
        <Skeleton width={80} />
      </header>
      <p className="sr-only" role="status">
        Завантажуємо профіль…
      </p>
      <div className="onboarding-layout" aria-hidden="true">
        <div className="onboarding-card">
          <span className="skeleton-steps">
            {Array.from({ length: 4 }, (_, index) => (
              <span className="skeleton-row" key={index}>
                <Skeleton kind="avatar-32" />
                <Skeleton kind="small" width={64} />
              </span>
            ))}
          </span>
          <Skeleton kind="title" width="48%" height={26} />
          <Skeleton width="72%" />
          <SkeletonFields count={4} />
          <span className="skeleton-actions skeleton-actions--end">
            <Skeleton kind="field" width={110} />
            <Skeleton kind="field" width={210} />
          </span>
        </div>
        <div className="onboarding-sidebar">
          <div className="onboarding-matches">
            <span className="skeleton-row">
              <span className="onboarding-matches__icon" />
              <Skeleton kind="title" width="62%" dark />
            </span>
            <Skeleton width="100%" dark />
            <Skeleton width="78%" dark />
            <span className="onboarding-matches__placeholder">
              <Skeleton width="80%" dark />
            </span>
          </div>
          <div className="onboarding-tip">
            <Skeleton kind="icon" width={34} height={34} />
            <Lines widths={["40%", "100%", "70%"]} />
          </div>
        </div>
      </div>
    </main>
  );
}

export function SkillChipsSkeleton({ count = 8, label }: { count?: number; label: string }) {
  return (
    <span className="skeleton-chips" role="status">
      <span className="sr-only">{label}</span>
      {Array.from({ length: count }, (_, index) => (
        <Skeleton kind="chip" width={[84, 112, 70, 96, 128, 76, 104, 90][index % 8]} key={index} />
      ))}
    </span>
  );
}
