import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import type { OpenDashboardModal } from "./DashboardModal";
import { Skeleton } from "./Skeleton";

type ModalActionProps = { onOpenModal: OpenDashboardModal };

export function DashboardEmptyState({
  icon,
  message,
}: {
  icon: DashboardIconName;
  message: string;
}) {
  return (
    <div className="dashboard-empty" role="status">
      <span className="dashboard-empty__icon" aria-hidden="true">
        <Icon name={icon} />
      </span>
      <p>{message}</p>
    </div>
  );
}

export function DashboardAvatar({
  initials,
  color = "purple",
  size = 40,
}: {
  initials: string;
  color?: string;
  size?: 32 | 40 | 48;
}) {
  return (
    <span
      className={`dashboard-avatar dashboard-avatar--${color} dashboard-avatar--${size}`}
      aria-hidden="true"
    >
      {initials}
    </span>
  );
}

export function SectionAction({
  children,
  arrow = false,
  onOpenModal,
}: { children: string; arrow?: boolean } & ModalActionProps) {
  return (
    <button
      className="dashboard-section-action"
      type="button"
      onClick={() => onOpenModal({ title: children })}
      aria-haspopup="dialog"
    >
      {children}
      {arrow && <Icon name="chevron-right" />}
    </button>
  );
}

export function DashboardStats({
  activeExchanges = null,
  loading = false,
}: {
  activeExchanges?: number | null;
  loading?: boolean;
}) {
  const stats: {
    icon: DashboardIconName;
    label: string;
    tone: string;
    value?: number | null;
  }[] = [
    { icon: "stat-swap", label: "Активні обміни", tone: "purple", value: activeExchanges },
    {
      icon: "stat-sparkles",
      label: "Нові збіги за тиждень",
      tone: "purple",
    },
    { icon: "stat-clock", label: "Проведено занять", tone: "blue" },
    { icon: "award", label: "Твій рейтинг", tone: "amber" },
  ];

  return (
    <section className="dashboard-stats" aria-label="Статистика">
      {stats.map((stat) => (
        <article className="dashboard-stat" key={stat.label}>
          <div className="dashboard-stat__top">
            <span className={`dashboard-stat__icon dashboard-stat__icon--${stat.tone}`}>
              <Icon name={stat.icon} />
            </span>
          </div>
          <div>
            {loading && stat.value !== undefined ? (
              <span className="dashboard-stat__value skeleton-stat">
                <Skeleton kind="title" width={36} height={30} />
              </span>
            ) : stat.value === undefined || stat.value === null ? (
              <p className="dashboard-stat__value" aria-label="Даних поки немає">
                —
              </p>
            ) : (
              <p className="dashboard-stat__value">{stat.value}</p>
            )}
            <p className="dashboard-stat__label">{stat.label}</p>
          </div>
        </article>
      ))}
    </section>
  );
}

export function DashboardSessions({ onOpenModal }: ModalActionProps) {
  return (
    <section className="dashboard-panel dashboard-sessions" aria-labelledby="sessions-title">
      <div className="dashboard-section-heading">
        <h2 id="sessions-title">Найближчі сесії</h2>
        <SectionAction onOpenModal={onOpenModal}>Календар</SectionAction>
      </div>
      <DashboardEmptyState icon="calendar" message="Тут з’являться заплановані сесії." />
    </section>
  );
}
