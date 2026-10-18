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
] as const;

export function AccountMenu({
  profile,
  pending,
  incoming = 0,
  onLogout,
}: {
  profile: Profile;
  pending: boolean;
  incoming?: number;
  onLogout: () => void;
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
        {links.map((link) => (
          <Link
            href={link.href}
            aria-current={pathname === link.href ? "page" : undefined}
            onClick={() => setOpen(false)}
            key={link.href}
          >
            <Icon name={link.icon} />
            <span className="account-menu__label">{link.label}</span>
            {link.href === "/requests" && incoming > 0 && (
              <span className="dashboard-nav-badge">
                <span className="sr-only">, очікують відповіді: </span>
                {incoming > 99 ? "99+" : incoming}
              </span>
            )}
          </Link>
        ))}
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
