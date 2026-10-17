import Image from "next/image";
import Link from "next/link";
import { useRef, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { authRequest, requestError } from "./AuthApi";
import { DashboardAvatar } from "./DashboardCards";
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
}: {
  profile: Profile;
  csrfToken: string;
  children: ReactNode;
}) {
  const router = useRouter();
  const [modal, setModal] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const logoutPending = useRef(false);
  async function logout() {
    if (logoutPending.current) return;
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
    <div className="dashboard-shell profile-shell">
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
          >
            <Image src="/figma/profile/settings.svg" alt="" width={20} height={20} />
            <span>Мій профіль</span>
          </Link>
          <button
            className="dashboard-nav-item"
            type="button"
            disabled={pending}
            onClick={() => void logout()}
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
          <Link
            className="dashboard-user profile-topbar-user"
            href="/profile"
            aria-label="Мій профіль"
          >
            <DashboardAvatar
              initials={`${profile.firstName.charAt(0)}${profile.lastName.charAt(0)}`}
            />
            <div className="dashboard-user__identity">
              <p>
                {profile.firstName} {profile.lastName}
              </p>
              <span>{profile.university.name}</span>
            </div>
          </Link>
        </header>
        <main className="dashboard-content" id="profile-content" tabIndex={-1}>
          {children}
        </main>
      </div>
      {modal && <DashboardModal title={modal} onClose={() => setModal("")} />}
    </div>
  );
}
