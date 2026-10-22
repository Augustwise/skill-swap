"use client";

import Link from "next/link";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { AdminCatalog } from "./AdminCatalog";
import { AdminReports } from "./AdminReports";
import { AdminReportOverview } from "./AdminReportOverview";
import { adminHref, adminTabs, type AdminTab } from "./AdminApi";
import { ModerationLog } from "./ModerationLog";
import { ModerateAccount } from "./ModerateAccount";
import { isUuid, safeAppPath } from "./AppRoutes";

export type AdministrationProps = {
  tab: AdminTab;
  page: number;
  status: string;
  q: string;
  userId: string;
  reportId?: string;
  from?: string;
};
export function Administration(props: AdministrationProps) {
  return (
    <AppGate
      page="admin"
      path={
        props.reportId
          ? safeAppPath(`/admin/reports/${props.reportId}`, "/admin")
          : adminHref(props.tab, { ...props, userId: isUuid(props.userId) ? props.userId : "" })
      }
    >
      {(context) => <AdminView {...context} {...props} />}
    </AppGate>
  );
}

function AdminView({
  profile,
  session,
  tab,
  page,
  status,
  q,
  userId,
  reportId,
  from,
}: AppContext & AdministrationProps) {
  const [version, setVersion] = useState(0);
  const allowed = session.user.role === "ADMIN";
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="admin"
      title="Модерація"
      subtitle="Безпечна спільнота та актуальний каталог навичок"
      contentId="admin-content"
      search={false}
    >
      {allowed ? (
        <>
          {!reportId && (
            <nav className="profile-tabs safety-tabs" aria-label="Розділи модерації">
              {(Object.entries(adminTabs) as [AdminTab, string][]).map(([key, label]) => (
                <Link
                  href={adminHref(key, key === "reports" ? { status: "OPEN" } : {})}
                  key={key}
                  aria-current={tab === key ? "page" : undefined}
                >
                  {label}
                </Link>
              ))}
            </nav>
          )}
          {reportId ? (
            <AdminReportOverview
              id={reportId}
              from={from ?? ""}
              viewerId={profile.id}
              csrfToken={session.csrfToken}
            />
          ) : tab === "reports" ? (
            <AdminReports status={status} page={page} />
          ) : tab === "skills" ? (
            <AdminCatalog q={q} page={page} csrfToken={session.csrfToken} />
          ) : (
            <div className="profile-sections">
              {userId &&
                (isUuid(userId) ? (
                  <ModerateAccount
                    userId={userId}
                    viewerId={profile.id}
                    csrfToken={session.csrfToken}
                    onChanged={() => setVersion((value) => value + 1)}
                  />
                ) : (
                  <p role="alert" className="safety-feedback">
                    Некоректний ідентифікатор користувача.
                  </p>
                ))}
              {(!userId || isUuid(userId)) && (
                <ModerationLog key={`${userId}/${version}`} page={page} userId={userId} />
              )}
            </div>
          )}
        </>
      ) : (
        <section className="account-card safety-empty" role="alert">
          <h2>Доступ лише для адміністраторів</h2>
          <p>Твій акаунт не має доступу до скарг інших студентів і журналу модерації.</p>
          <Link className="dashboard-button" href="/main">
            На головну
          </Link>
        </section>
      )}
    </AppShell>
  );
}
