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
          <div>
            <h2 id="cta-title">Готовий знайти свій match?</h2>
            <p>
              Приєднуйся до SkillSwap і відкрий нові можливості разом зі студентами всієї України!
            </p>
          </div>
          <a className="button" href="#top">
            Спробувати <Icon name="arrow" size={20} />
          </a>
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
