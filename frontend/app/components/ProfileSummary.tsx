import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { Profile } from "./OnboardingApi";
import { skillLevels } from "./ProfileApi";

export function ProfileSummary({ profile }: { profile: Profile }) {
  return (
    <aside className="profile-rail" aria-label="Попередній перегляд профілю">
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
          {profile.bio && <p className="profile-biography">{profile.bio}</p>}
        </div>
        <p className="profile-hint">Тут відображаються збережені дані профілю.</p>
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
