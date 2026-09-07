import Image from "next/image";
import { Icon, type IconName } from "./components/icons";
import { Logo } from "./components/Logo";

function SkillPill({ type }: { type: "guitar" | "photoshop" }) {
  return (
    <div className={`skill-pill ${type === "guitar" ? "skill-pill--pink" : "skill-pill--blue"}`}>
      {type === "guitar" ? (
        <span className="guitar" aria-hidden="true">
          🎸
        </span>
      ) : (
        <span className="photoshop" aria-hidden="true">
          Ps
        </span>
      )}
      <strong>{type === "guitar" ? "Гітара" : "Photoshop"}</strong>
    </div>
  );
}

function FloatingSkill({ type }: { type: "guitar" | "photoshop" }) {
  return (
    <div className={`floating-skill floating-skill--${type}`}>
      {type === "guitar" ? (
        <span className="floating-skill__emoji" aria-hidden="true">
          🎸
        </span>
      ) : (
        <span className="photoshop photoshop--large" aria-hidden="true">
          Ps
        </span>
      )}
      <span>
        <small>{type === "guitar" ? "Можу навчити:" : "Хочу навчитись:"}</small>
        <strong>{type === "guitar" ? "Гітара" : "Photoshop"}</strong>
      </span>
    </div>
  );
}

function Avatar({ person }: { person: "andrii" | "olena" }) {
  return (
    <div
      className={`avatar avatar--${person}`}
      role="img"
      aria-label={person === "andrii" ? "Андрій" : "Олена"}
    />
  );
}

const steps: Array<{ icon: IconName; title: string; text: string }> = [
  {
    icon: "profile",
    title: "1. Додаєш свої навички",
    text: "Розкажи, що ти вмієш і чому можеш навчити інших.",
  },
  {
    icon: "list",
    title: "2. Вказуєш, що хочеш вивчити",
    text: "Обери навички, які хочеш опанувати.",
  },
  {
    icon: "people",
    title: "3. Отримуєш сумісний match",
    text: "Ми знаходимо студентів, чиї навички доповнюють твої.",
  },
];

const benefits: Array<{ icon: IconName; title: string; text: string }> = [
  {
    icon: "bolt",
    title: "Розумний matching",
    text: "Алгоритм знаходить студентів, які справді доповнюють одне одного.",
  },
  {
    icon: "sprout",
    title: "Безкоштовний розвиток",
    text: "Отримуй нові знання без фінансових витрат — лише через взаємодопомогу.",
  },
  {
    icon: "people",
    title: "Студентська спільнота",
    text: "Знайомся з однодумцями з різних університетів і розширюй коло спілкування.",
  },
];

export default function Home() {
  return (
    <div id="top" className="page-shell">
      <header className="site-header">
        <div className="container header-inner">
          <Logo />
          <nav className="main-nav" aria-label="Основна навігація">
            <a href="#how">Як це працює</a>
            <a href="#benefits">Переваги</a>
            <a href="#match">Match</a>
            <a href="#reviews">Відгуки</a>
          </nav>
          <a className="button button--small" href="#match">
            Спробувати
          </a>
        </div>
      </header>

      <main>
        <section className="hero container" aria-labelledby="hero-title">
          <div className="hero-copy">
            <h1 id="hero-title">
              Обмінюйся
              <br />
              навичками
              <br />
              <span>зі студентами</span>
            </h1>
            <p>
              Вчися тому, що цікаво, і ділись тим, у чому ти сильний. SkillSwap — платформа для
              обміну знаннями між студентами.
            </p>
            <div className="hero-actions">
              <a className="button" href="#match">
                Знайти match <Icon name="arrow" size={20} />
              </a>
              <a className="button button--secondary" href="#how">
                Як це працює
              </a>
            </div>
          </div>
          <div className="hero-visual">
            <Image
              src="/skill-swap-hero.png"
              alt="Студенти обмінюються навичками в університетському просторі"
              fill
              priority
              sizes="(max-width: 760px) 100vw, 62vw"
            />
            <FloatingSkill type="guitar" />
            <FloatingSkill type="photoshop" />
          </div>
        </section>

        <section id="how" className="section container" aria-labelledby="how-title">
          <div className="section-heading">
            <h2 id="how-title">Як це працює</h2>
            <p>Три прості кроки до нових знань і цікавих знайомств</p>
          </div>
          <div className="steps-grid">
            {steps.map((step) => (
              <article className="info-card" key={step.title}>
                <span className="icon-circle">
                  <Icon name={step.icon} />
                </span>
                <h3>{step.title}</h3>
                <p>{step.text}</p>
              </article>
            ))}
          </div>
        </section>

        <section id="match" className="match-section container" aria-labelledby="match-title">
          <div className="section-heading section-heading--compact">
            <h2 id="match-title">Приклад match</h2>
            <p>Студенти обмінюються навичками та навчаються одне в одного.</p>
          </div>
          <div className="match-flow">
            <article className="profile-card">
              <div className="profile-head">
                <Avatar person="andrii" />
                <div>
                  <h3>Андрій</h3>
                  <p>Студент КПІ</p>
                </div>
              </div>
              <div className="skill-row">
                <span>Можу навчити:</span>
                <SkillPill type="guitar" />
              </div>
              <div className="skill-row">
                <span>Хочу навчитись:</span>
                <SkillPill type="photoshop" />
              </div>
            </article>
            <div className="swap-icon" aria-label="Взаємний обмін навичками">
              <span>→</span>
              <span>←</span>
            </div>
            <article className="profile-card">
              <div className="profile-head">
                <Avatar person="olena" />
                <div>
                  <h3>Олена</h3>
                  <p>Студентка КНУ</p>
                </div>
              </div>
              <div className="skill-row">
                <span>Можу навчити:</span>
                <SkillPill type="photoshop" />
              </div>
              <div className="skill-row">
                <span>Хочу навчитись:</span>
                <SkillPill type="guitar" />
              </div>
            </article>
          </div>
        </section>

        <section
          id="benefits"
          className="section benefits container"
          aria-labelledby="benefits-title"
        >
          <div className="section-heading section-heading--compact">
            <h2 id="benefits-title">Чому SkillSwap?</h2>
          </div>
          <div className="benefits-grid">
            {benefits.map((benefit) => (
              <article className="benefit-card" key={benefit.title}>
                <span className="icon-circle">
                  <Icon name={benefit.icon} />
                </span>
                <div>
                  <h3>{benefit.title}</h3>
                  <p>{benefit.text}</p>
                </div>
              </article>
            ))}
          </div>
        </section>

        <section id="reviews" className="cta container" aria-labelledby="cta-title">
          <div className="cta-left">
            <div className="cta-plane-badge" aria-hidden="true">
              <svg
                className="cta-plane-svg"
                viewBox="0 0 72 60"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <line
                  x1="18"
                  y1="14"
                  x2="8"
                  y2="8"
                  stroke="#8c8ff8"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                />
                <line
                  x1="14"
                  y1="30"
                  x2="3"
                  y2="30"
                  stroke="#8c8ff8"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                />
                <line
                  x1="18"
                  y1="46"
                  x2="8"
                  y2="52"
                  stroke="#8c8ff8"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                />

                {/* Origami folded paper airplane */}
                <polygon points="68,11 25,26 43,38" fill="#5c3ef7" />
                <polygon points="68,11 43,38 55,41" fill="#4929ea" />
                <polygon points="43,38 35,50 46,44" fill="#3619ce" />
              </svg>
            </div>

            <div className="cta-content">
              <h2 id="cta-title">
                Готовий знайти свій <span>SkillSwap?</span>
              </h2>
              <p>
                Приєднуйся вже сьогодні та відкрий нові можливості разом зі студентами з усієї
                України!
              </p>
            </div>
          </div>

          <div className="cta-right">
            <a className="cta-btn" href="#match">
              <span>Знайти свій match</span>
              <Icon name="arrow" size={17} />
            </a>

            <div className="cta-note" aria-hidden="true">
              <svg
                className="cta-note-burst"
                viewBox="0 0 20 34"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <line
                  x1="17"
                  y1="8"
                  x2="4"
                  y2="4"
                  stroke="#5439f5"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                />
                <line
                  x1="19"
                  y1="17"
                  x2="3"
                  y2="17"
                  stroke="#5439f5"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                />
                <line
                  x1="17"
                  y1="26"
                  x2="4"
                  y2="30"
                  stroke="#5439f5"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                />
              </svg>
              <div className="cta-note-text">
                <span>Нові навички.</span>
                <span>Нові друзі.</span>
                <span>Яскравіше майбутнє!</span>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="site-footer container">
        <Logo />
        <nav aria-label="Навігація в підвалі">
          <a href="#how">Як це працює</a>
          <a href="#benefits">Переваги</a>
          <a href="#match">Match</a>
          <a href="#reviews">Відгуки</a>
        </nav>
        <p>© 2026 SkillSwap. Всі права захищені.</p>
      </footer>
    </div>
  );
}
