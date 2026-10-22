import Image from "next/image";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { Profile } from "./OnboardingApi";
import { skillLevels } from "./ProfileApi";
import { ProfileReputation } from "./ReceivedReviews";

export function ProfileSummary({ profile }: { profile: Profile }) {
  const checks = [
    [
      "Вказано основні дані",
      Boolean(profile.firstName && profile.lastName && profile.university.id),
    ],
    ["Додано навички для викладання", profile.teachingSkills.length > 0],
    ["Додано цілі навчання", profile.learningSkills.length > 0],
    ["Заповнено «Про себе»", Boolean(profile.bio)],
    [
      "Обрано формат занять",
      profile.formats.length > 0 && (!profile.formats.includes("OFFLINE") || Boolean(profile.city)),
    ],
  ] as const;
  const percent = checks.filter(([, complete]) => complete).length * 20;
  return (
    <aside className="profile-rail" aria-label="Попередній перегляд профілю">
      <ProfileReputation />
      <section className="account-card">
        <h2>Так тебе бачать студенти</h2>
        <div className="profile-preview">
          <div className="profile-identity">
            <DashboardAvatar
              initials={`${profile.firstName.charAt(0)}${profile.lastName.charAt(0)}`}
              size={48}
            />
            <div>
              <h3>
                {profile.firstName} {profile.lastName}
              </h3>
              <p>
                {[
                  profile.university.name,
                  profile.faculty?.name,
                  profile.course && `${profile.course} курс`,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </p>
            </div>
          </div>
          <div className="profile-tags">
            {(["teachingSkills", "learningSkills"] as const).flatMap((list) =>
              profile[list].map((skill) => (
                <span
                  className={`dashboard-skill ${list === "learningSkills" ? "dashboard-skill--learn" : ""}`}
                  key={`${list}-${skill.skillId}`}
                  title={`${list === "teachingSkills" ? "Навчаю" : "Вивчаю"}: ${skillLevels[skill.level]}`}
                >
                  <Icon name={list === "teachingSkills" ? "teach-dot" : "learn-dot"} />
                  {skill.name}
                </span>
              )),
            )}
          </div>
          {profile.city && <p>{profile.city}</p>}
          <p>
            {profile.formats
              .map((format) => (format === "ONLINE" ? "Онлайн" : "Офлайн"))
              .join(" · ")}
          </p>
          {profile.bio && <p className="profile-biography">{profile.bio}</p>}
        </div>
        <p className="profile-hint">Тут відображаються збережені дані профілю.</p>
      </section>
      <section className="account-card">
        <h2>Заповненість профілю</h2>
        <div
          className="profile-progress"
          role="progressbar"
          aria-label="Заповненість профілю"
          aria-valuenow={percent}
          aria-valuemin={0}
          aria-valuemax={100}
          style={{ background: `conic-gradient(var(--purple) ${percent}%, var(--purple-soft) 0)` }}
        >
          <span>{percent}%</span>
        </div>
        <ul className="profile-checklist">
          {checks.map(([label, complete]) => (
            <li key={label}>
              <span
                className={complete ? "is-complete" : ""}
                aria-label={complete ? "Готово" : "Ще не заповнено"}
              >
                <Image
                  src={`/figma/profile/${complete ? "check" : "plus"}.svg`}
                  alt=""
                  width={11}
                  height={11}
                />
              </span>
              {label}
            </li>
          ))}
        </ul>
      </section>
      <section className="account-card profile-tip">
        <h2>Розкажи про себе</h2>
        <p>
          Додай навички, свій рівень і зручний формат занять, щоб інші студенти могли знайти спільні
          інтереси.
        </p>
      </section>
    </aside>
  );
}
