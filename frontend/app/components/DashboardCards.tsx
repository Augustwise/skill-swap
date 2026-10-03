import type { DashboardExchange, DashboardMatch, DashboardRequest } from "./DashboardData";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import type { OpenDashboardModal } from "./DashboardModal";

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

function SkillTag({ label, learn = false }: { label: string; learn?: boolean }) {
  return (
    <span className={`dashboard-skill ${learn ? "dashboard-skill--learn" : ""}`}>
      <Icon name={learn ? "learn-dot" : "teach-dot"} />
      {label}
    </span>
  );
}

function StatusBadge({ active = false }: { active?: boolean }) {
  return (
    <span className={`dashboard-status ${active ? "dashboard-status--active" : ""}`}>
      <Icon name={active ? "active-dot" : "pending-dot"} />
      {active ? "Активний" : "Очікує"}
    </span>
  );
}

function SectionAction({
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

export function DashboardStats() {
  const stats: {
    icon: DashboardIconName;
    label: string;
    tone: string;
  }[] = [
    { icon: "stat-swap", label: "Активні обміни", tone: "purple" },
    {
      icon: "stat-sparkles",
      label: "Нові збіги за тиждень",
      tone: "purple",
    },
    { icon: "stat-clock", label: "Проведено занять", tone: "blue" },
    { icon: "award", label: "Твій рейтинг", tone: "amber" },
  ];

  return (
    <section className="dashboard-stats" aria-label="Статистика за тиждень — даних поки немає">
      {stats.map((stat) => (
        <article className="dashboard-stat" key={stat.label}>
          <div className="dashboard-stat__top">
            <span className={`dashboard-stat__icon dashboard-stat__icon--${stat.tone}`}>
              <Icon name={stat.icon} />
            </span>
          </div>
          <div>
            <p className="dashboard-stat__value" aria-label="Даних поки немає">
              —
            </p>
            <p className="dashboard-stat__label">{stat.label}</p>
          </div>
        </article>
      ))}
    </section>
  );
}

export function DashboardMatchCard({
  match,
  onOpenModal,
}: { match: DashboardMatch } & ModalActionProps) {
  return (
    <article className="dashboard-match">
      <div className="dashboard-match__person">
        <DashboardAvatar initials={match.initials} color={match.color} size={48} />
        <div className="dashboard-match__identity">
          <h3>{match.name}</h3>
          <p>{match.university}</p>
        </div>
        <span className="dashboard-score">
          <Icon name="match-sparkles" />
          {match.score}% збіг
        </span>
      </div>
      <div
        className="dashboard-rating"
        aria-label={`Рейтинг ${match.rating} з 5, ${match.reviews}`}
      >
        <span className="dashboard-rating__stars" aria-hidden="true">
          {Array.from({ length: 5 }, (_, index) => (
            <Icon name="star" key={index} />
          ))}
        </span>
        <strong>{match.rating}</strong>
        <span>({match.reviews})</span>
      </div>
      <div className="dashboard-match__skills">
        <div>
          <h4>Навчить</h4>
          <div className="dashboard-tags">
            {match.teaching.map((skill) => (
              <SkillTag label={skill} key={skill} />
            ))}
          </div>
        </div>
        <div>
          <h4>Хоче вивчити</h4>
          <div className="dashboard-tags">
            {match.learning.map((skill) => (
              <SkillTag label={skill} learn key={skill} />
            ))}
          </div>
        </div>
      </div>
      <div className="dashboard-match__actions">
        <button
          type="button"
          className="dashboard-button dashboard-button--primary"
          onClick={() =>
            onOpenModal({
              title: "Запропонувати обмін",
              description: `Обмін навичками з ${match.name} буде доступний згодом. Поки що це демонстраційний профіль.`,
            })
          }
          aria-haspopup="dialog"
        >
          Запропонувати обмін
        </button>
        <button
          type="button"
          className="dashboard-button dashboard-button--icon"
          onClick={() =>
            onOpenModal({
              title: `Повідомлення: ${match.name}`,
              description: "Чат ще в розробці. Можливість написати цій людині з’явиться згодом.",
            })
          }
          aria-haspopup="dialog"
          aria-label={`Написати: ${match.name}`}
        >
          <Icon name="chat" />
        </button>
      </div>
    </article>
  );
}

export function DashboardExchanges({
  onOpenModal,
  exchanges = [],
}: ModalActionProps & { exchanges?: readonly DashboardExchange[] }) {
  return (
    <section className="dashboard-panel dashboard-exchanges" aria-labelledby="exchanges-title">
      <div className="dashboard-section-heading">
        <h2 id="exchanges-title">Активні обміни</h2>
        <SectionAction arrow onOpenModal={onOpenModal}>
          Усі обміни
        </SectionAction>
      </div>
      {exchanges.length === 0 && (
        <DashboardEmptyState
          icon="nav-swap"
          message="Тут з’являться твої активні обміни навичками."
        />
      )}
      {exchanges.map((exchange) => (
        <article className="dashboard-exchange" key={exchange.name}>
          <DashboardAvatar initials={exchange.initials} color={exchange.color} />
          <div className="dashboard-exchange__person">
            <div className="dashboard-exchange__heading">
              <h3>{exchange.name}</h3>
              <StatusBadge active />
            </div>
            <div className="dashboard-tags">
              <SkillTag label="Ти: Гітара" />
              <Icon name="swap" />
              <SkillTag label={exchange.learning} learn />
            </div>
          </div>
          <div className="dashboard-exchange__progress">
            <span>
              {exchange.completed} з {exchange.total} занять
            </span>
            <progress
              value={exchange.completed}
              max={exchange.total}
              aria-label={`Прогрес обміну: ${exchange.name}`}
            />
            <p>
              <Icon name="clock" />
              {exchange.time}
            </p>
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

export function DashboardRequests({
  onOpenModal,
  requests = [],
}: ModalActionProps & { requests?: readonly DashboardRequest[] }) {
  return (
    <section
      id="dashboard-requests"
      className="dashboard-panel dashboard-requests"
      aria-labelledby="requests-title"
    >
      <div className="dashboard-section-heading">
        <h2 id="requests-title">Нові запити</h2>
        <SectionAction onOpenModal={onOpenModal}>Усі запити</SectionAction>
      </div>
      {requests.length === 0 && (
        <DashboardEmptyState icon="inbox" message="Тут з’являться нові запити на обмін." />
      )}
      {requests.map((request) => (
        <article className="dashboard-request" key={request.name}>
          <div className="dashboard-request__person">
            <DashboardAvatar initials={request.initials} color={request.color} size={32} />
            <div>
              <h3>{request.name}</h3>
              <p>{request.time}</p>
            </div>
            <StatusBadge />
          </div>
          <div className="dashboard-tags">
            <SkillTag label={request.skill} learn />
            <Icon name="swap" />
            <SkillTag label="Гітара" />
          </div>
          <div className="dashboard-request__actions">
            <button
              className="dashboard-button dashboard-button--primary"
              type="button"
              onClick={() =>
                onOpenModal({
                  title: "Прийняти запит",
                  description: `Прийняття запиту від ${request.name} буде доступне згодом. Поки що це демонстраційний запит.`,
                })
              }
              aria-haspopup="dialog"
            >
              Прийняти
            </button>
            <button
              className="dashboard-button"
              type="button"
              onClick={() =>
                onOpenModal({
                  title: "Відхилити запит",
                  description: `Відхилення запиту від ${request.name} буде доступне згодом. Поки що це демонстраційний запит.`,
                })
              }
              aria-haspopup="dialog"
            >
              Відхилити
            </button>
          </div>
        </article>
      ))}
    </section>
  );
}
