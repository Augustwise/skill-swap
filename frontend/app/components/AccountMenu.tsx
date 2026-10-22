"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { Profile } from "./OnboardingApi";

const links = [
  { href: "/profile", icon: "settings", label: "Мій профіль" },
  { href: "/main", icon: "dashboard-muted", label: "Головна" },
  { href: "/search", icon: "nav-search", label: "Пошук навичок" },
  { href: "/matches", icon: "nav-sparkles", label: "Мої збіги" },
  { href: "/requests", icon: "inbox", label: "Запити на обмін" },
  { href: "/exchanges", icon: "nav-swap", label: "Активні обміни" },
  { href: "/schedule", icon: "calendar", label: "Розклад" },
  { href: "/messages", icon: "nav-chat", label: "Повідомлення" },
  { href: "/settings", icon: "settings", label: "Налаштування" },
] as const;

const badgeLabels: Record<string, string> = {
  "/requests": "очікують відповіді",
  "/messages": "непрочитані",
};

export function AccountMenu({
  profile,
  pending,
  badges = {},
  onLogout,
  isAdmin = false,
}: {
  profile: Profile;
  pending: boolean;
  badges?: Record<string, number>;
  onLogout: () => void;
  isAdmin?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const container = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const id = useId();
  const pathname = usePathname();
  const name = `${profile.firstName} ${profile.lastName}`.trim();
  const university = `${profile.university.name}${profile.course ? ` · ${profile.course} курс` : ""}`;
  const initials = `${profile.firstName.charAt(0)}${profile.lastName.charAt(0)}`.toLocaleUpperCase(
    "uk",
  );

  useEffect(() => {
    if (!open) return;
    function dismiss(event: PointerEvent) {
      if (event.target instanceof Node && !container.current?.contains(event.target))
        setOpen(false);
    }
    document.addEventListener("pointerdown", dismiss);
    return () => document.removeEventListener("pointerdown", dismiss);
  }, [open]);

  return (
    <div
      className="account-menu"
      ref={container}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.preventDefault();
          setOpen(false);
          trigger.current?.focus();
        }
      }}
    >
      <button
        className="dashboard-user account-menu__trigger"
        type="button"
        ref={trigger}
        aria-label="Меню користувача"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((value) => !value)}
      >
        <DashboardAvatar initials={initials || "СС"} />
        <span className="dashboard-user__identity">
          <span className="account-menu__name" title={name}>
            {name}
          </span>
          <span title={university}>{university}</span>
        </span>
        <Icon name="chevron-down" />
      </button>
      <nav className="account-menu__panel" id={id} hidden={!open} aria-label="Меню акаунта">
        <div className="account-menu__header">
          <DashboardAvatar initials={initials || "СС"} />
          <span className="account-menu__identity">
            <span className="account-menu__name">{name}</span>
            <span>{profile.university.name}</span>
            {profile.course ? <span>{profile.course} курс</span> : null}
          </span>
        </div>
        <hr className="account-menu__divider" />
        {links.map((link) => {
          const badge = badges[link.href] ?? 0;
          return (
            <Link
              href={link.href}
              aria-current={pathname === link.href ? "page" : undefined}
              onClick={() => setOpen(false)}
              key={link.href}
            >
              <Icon name={link.icon} />
              <span className="account-menu__label">{link.label}</span>
              {badge > 0 && (
                <span className="dashboard-nav-badge">
                  <span className="sr-only">, {badgeLabels[link.href]}: </span>
                  {badge > 99 ? "99+" : badge}
                </span>
              )}
            </Link>
          );
        })}
        {isAdmin && (
          <Link href="/admin?status=OPEN" onClick={() => setOpen(false)}>
            <Icon name="inbox" />
            <span className="account-menu__label">Модерація</span>
          </Link>
        )}
        <hr className="account-menu__divider" />
        <button
          className="account-menu__logout"
          type="button"
          disabled={pending}
          onClick={onLogout}
        >
          <Icon name="logout" />
          <span className="account-menu__label">{pending ? "Виходимо…" : "Вийти"}</span>
        </button>
      </nav>
    </div>
  );
}
