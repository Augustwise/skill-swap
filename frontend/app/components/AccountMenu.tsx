"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { Profile } from "./OnboardingApi";

export function AccountMenu({
  profile,
  pending,
  onLogout,
}: {
  profile: Profile;
  pending: boolean;
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
        <Link
          href="/profile"
          aria-current={pathname === "/profile" ? "page" : undefined}
          onClick={() => setOpen(false)}
        >
          <Icon name="settings" /> Мій профіль
        </Link>
        <Link
          href="/main"
          aria-current={pathname === "/main" ? "page" : undefined}
          onClick={() => setOpen(false)}
        >
          <Icon name="dashboard" /> Головна
        </Link>
        <button type="button" disabled={pending} onClick={onLogout}>
          <Icon name="logout" /> {pending ? "Виходимо…" : "Вийти"}
        </button>
      </nav>
    </div>
  );
}
