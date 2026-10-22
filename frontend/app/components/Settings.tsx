"use client";

import Link from "next/link";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { AccountSecurity } from "./AccountSecurity";
import { PrivacySettings } from "./PrivacySettings";
import { SafetyLists } from "./SafetyLists";
import { settingsHref, settingsTabs, type SettingsTab } from "./SafetyApi";

export function Settings({ tab, page }: { tab: SettingsTab; page: number }) {
  return (
    <AppGate page="settings" path={settingsHref(tab, page)}>
      {(context) => <SettingsView {...context} tab={tab} page={page} />}
    </AppGate>
  );
}

function SettingsView({
  profile,
  session,
  tab,
  page,
}: AppContext & { tab: SettingsTab; page: number }) {
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="settings"
      title="Налаштування"
      subtitle="Твоя приватність, безпека та контроль взаємодії"
      contentId="settings-content"
      search={false}
    >
      <nav className="profile-tabs safety-tabs" aria-label="Розділи налаштувань">
        {(Object.entries(settingsTabs) as [SettingsTab, string][]).map(([key, label]) => (
          <Link href={settingsHref(key)} key={key} aria-current={tab === key ? "page" : undefined}>
            {label}
          </Link>
        ))}
      </nav>
      <div className="profile-columns safety-columns">
        <div className="profile-sections">
          {tab === "privacy" && <PrivacySettings csrfToken={session.csrfToken} />}
          {tab === "security" && <AccountSecurity csrfToken={session.csrfToken} />}
          {(tab === "blocks" || tab === "reports") && (
            <SafetyLists key={tab} tab={tab} page={page} csrfToken={session.csrfToken} />
          )}
        </div>
        <aside className="account-card safety-card safety-tip">
          <span className="safety-tip__mark" aria-hidden="true">
            ✓
          </span>
          <h2>Ти керуєш своїм простором</h2>
          <p>
            Електронна пошта та приватне листування недоступні в публічному профілі. Місце й
            посилання на заняття бачать лише його учасники.
          </p>
          <p>
            Якщо взаємодія викликає дискомфорт, можна заблокувати студента або повідомити
            адміністратора.
          </p>
          <Link href="/profile" className="profile-tip__link">
            Переглянути мій профіль →
          </Link>
        </aside>
      </div>
    </AppShell>
  );
}
