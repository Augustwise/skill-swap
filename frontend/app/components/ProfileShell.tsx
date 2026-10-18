import Image from "next/image";
import Link from "next/link";
import { useRef, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { authRequest, requestError } from "./AuthApi";
import { AccountMenu } from "./AccountMenu";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import { DashboardModal } from "./DashboardModal";
import { Logo } from "./Logo";
import type { Profile } from "./OnboardingApi";

const navigation: [string, DashboardIconName][] = [
  ["Пошук навичок", "nav-search"],
  ["Мої збіги", "nav-sparkles"],
  ["Запити на обмін", "inbox"],
  ["Активні обміни", "nav-swap"],
  ["Повідомлення", "nav-chat"],
  ["Календар", "calendar"],
];

export function ProfileShell({
  profile,
  csrfToken,
  children,
  hasUnsavedChanges,
}: {
  profile: Profile;
  csrfToken: string;
  children: ReactNode;
  hasUnsavedChanges: boolean;
}) {
  const router = useRouter();
  const [modal, setModal] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const logoutPending = useRef(false);
  const canLeave = () =>
    !hasUnsavedChanges || window.confirm("Є незбережені зміни. Залишити сторінку без збереження?");
  async function logout() {
    if (logoutPending.current || !canLeave()) return;
    logoutPending.current = true;
    setPending(true);
    setNotice("");
    try {
      await authRequest("auth/logout", { method: "POST" }, csrfToken);
      router.replace("/login");
    } catch (error) {
      setNotice(requestError(error));
      logoutPending.current = false;
      setPending(false);
    }
  }
  return (
    <div
      className="dashboard-shell profile-shell"
      onClickCapture={(event) => {
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
        const href = link?.getAttribute("href");
        if (
          href &&
          !href.startsWith("#") &&
          new URL(href, window.location.href).pathname !== window.location.pathname &&
          !canLeave()
        ) {
          event.preventDefault();
          event.stopPropagation();
        }
      }}
    >
      <a className="dashboard-skip" href="#profile-content">
        До вмісту
      </a>
      <aside className="dashboard-sidebar" aria-label="Бічна панель">
        <div className="dashboard-sidebar__brand">
          <Logo variant="app" />
        </div>
        <nav className="dashboard-nav" aria-label="Основна навігація">
          <Link className="dashboard-nav-item" href="/main">
            <Icon name="dashboard" />
            <span>Головна</span>
          </Link>
          {navigation.map(([label, icon]) => (
            <button
              className="dashboard-nav-item"
              type="button"
              key={label}
              onClick={() => setModal(label)}
              aria-haspopup="dialog"
            >
              <Icon name={icon} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="dashboard-sidebar__footer">
          <Link
            className="dashboard-nav-item dashboard-nav-item--active"
            href="/profile"
            aria-current="page"
            aria-label="Мій профіль"
          >
            <Image src="/figma/profile/settings.svg" alt="" width={20} height={20} />
            <span>Мій профіль</span>
          </Link>
          <button
            className="dashboard-nav-item"
            type="button"
            disabled={pending}
            onClick={() => void logout()}
            aria-label={pending ? "Виходимо…" : "Вийти"}
          >
            <Icon name="logout" />
            <span>{pending ? "Виходимо…" : "Вийти"}</span>
          </button>
          {notice && (
            <p className="dashboard-notice" role="alert">
              {notice}
            </p>
          )}
        </div>
      </aside>
      <div className="dashboard-main">
        <header className="dashboard-topbar">
          <div className="dashboard-topbar__title">
            <h1>Мій профіль</h1>
            <p>Дані, які бачать студенти у твоєму профілі</p>
          </div>
          <AccountMenu profile={profile} pending={pending} onLogout={() => void logout()} />
        </header>
        <main className="dashboard-content" id="profile-content" tabIndex={-1}>
          {children}
        </main>
      </div>
      {modal && <DashboardModal title={modal} onClose={() => setModal("")} />}
    </div>
  );
}
