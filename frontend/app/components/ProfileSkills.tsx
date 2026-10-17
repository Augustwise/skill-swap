import Image from "next/image";
import Link from "next/link";
import { useRef, useState } from "react";
import { AuthError, authRequest, requestError } from "./AuthApi";
import type { Profile, SkillLevel } from "./OnboardingApi";
import { changeProfileSkill, skillLevels, type SkillList } from "./ProfileApi";
import { ProfileSkillCatalog } from "./ProfileSkillCatalog";

export function ProfileSkills({
  profile,
  csrfToken,
  onSaved,
}: {
  profile: Profile;
  csrfToken: string;
  onSaved: (profile: Profile) => void;
}) {
  const [adding, setAdding] = useState<SkillList | null>(null);
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const [expired, setExpired] = useState(false);
  const saving = useRef(false);

  async function mutate(
    list: SkillList,
    method: "POST" | "PATCH" | "DELETE",
    id: string,
    level?: SkillLevel,
  ) {
    if (saving.current) return;
    saving.current = true;
    setPending(true);
    setNotice("");
    try {
      onSaved(await changeProfileSkill(list, method, id, csrfToken, level));
      if (method === "POST") setAdding(null);
      setNotice(method === "DELETE" ? "Навичку видалено." : "Навичку збережено.");
    } catch (error) {
      setExpired(error instanceof AuthError && error.status === 401);
      setNotice(
        error instanceof AuthError && error.status === 409
          ? "Ця навичка вже є в списку."
          : requestError(error),
      );
      // A response may be lost after a successful write; reconcile before a retry.
      try {
        const data = await authRequest<{ profile: Profile }>("me/profile");
        onSaved(data.profile);
      } catch {
        /* Keep the last confirmed profile if the server remains unavailable. */
      }
    } finally {
      saving.current = false;
      setPending(false);
    }
  }

  return (
    <div className="profile-sections" aria-busy={pending}>
      {notice && (
        <p className="account-card" role="status">
          {notice}
          {expired && <Link href="/login?next=/profile">Увійти знову</Link>}
        </p>
      )}
      {(["teaching", "learning"] as const).map((list) => {
        const teaching = list === "teaching";
        const skills = profile[teaching ? "teachingSkills" : "learningSkills"];
        return (
          <section className="account-card" key={list} aria-labelledby={`profile-${list}-title`}>
            <div>
              <h2 id={`profile-${list}-title`}>
                {teaching ? "Чого я навчаю" : "Що я хочу вивчити"}
              </h2>
              <p className="profile-hint">
                {teaching
                  ? "Навички, якими ти можеш поділитися зі студентами"
                  : "Навички, які хочеш опанувати, та твій поточний рівень"}
              </p>
            </div>
            <fieldset className="profile-fields profile-skill-rows" disabled={pending || expired}>
              <legend className="sr-only">
                {teaching ? "Пропоновані навички" : "Бажані навички"}
              </legend>
              {skills.length === 0 && (
                <p className="profile-hint">
                  {teaching
                    ? "Ти ще не додав навичок для викладання."
                    : "Ти ще не додав навичок для вивчення."}
                </p>
              )}
              {skills.map((skill) => (
                <div
                  className={`profile-skill-row ${teaching ? "" : "profile-skill-row--learn"}`}
                  key={skill.skillId}
                >
                  <span className="profile-skill-icon">
                    <Image
                      src={`/figma/profile/${teaching ? "teach" : "learn"}.svg`}
                      alt=""
                      width={17}
                      height={17}
                    />
                  </span>
                  <strong>{skill.name}</strong>
                  <label className="register-field">
                    <span className="sr-only">{`Рівень ${skill.name}: ${teaching ? "навчаю" : "вивчаю"}`}</span>
                    <select
                      value={skill.level}
                      onChange={(event) =>
                        void mutate(list, "PATCH", skill.skillId, event.target.value as SkillLevel)
                      }
                    >
                      {Object.entries(skillLevels).map(([value, label]) => (
                        <option value={value} key={value}>
                          {label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    type="button"
                    className="profile-remove-skill"
                    aria-label={`Видалити ${skill.name} зі списку ${teaching ? "навчаю" : "вивчаю"}`}
                    onClick={() => void mutate(list, "DELETE", skill.skillId)}
                  >
                    Видалити
                  </button>
                </div>
              ))}
              {adding !== list && (
                <button
                  className="profile-add-button"
                  type="button"
                  onClick={() => setAdding(list)}
                >
                  +{" "}
                  {teaching ? "Додати навичку, якої я можу навчити" : "Додати навичку для вивчення"}
                </button>
              )}
            </fieldset>
            {adding === list && (
              <ProfileSkillCatalog
                existing={skills.map((skill) => skill.skillId)}
                pending={pending || expired}
                onAdd={(id, level) => void mutate(list, "POST", id, level)}
                onClose={() => setAdding(null)}
              />
            )}
          </section>
        );
      })}
      <p className="profile-hint">
        Зміни навичок зберігаються одразу. Для взаємних збігів заповни обидва списки та обери формат
        занять.
      </p>
    </div>
  );
}
