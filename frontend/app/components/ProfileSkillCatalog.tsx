import { useEffect, useState, type FormEvent } from "react";
import { authRequest } from "./AuthApi";
import type { Skill, SkillLevel } from "./OnboardingApi";
import { skillLevels } from "./ProfileApi";

export function ProfileSkillCatalog({
  existing,
  pending,
  onAdd,
  onClose,
}: {
  existing: string[];
  pending: boolean;
  onAdd: (id: string, level: SkillLevel) => void;
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [skills, setSkills] = useState<Skill[]>([]);
  const [skillId, setSkillId] = useState("");
  const [level, setLevel] = useState<SkillLevel>("BEGINNER");
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const data = await authRequest<{ items: Skill[] }>(
          `skills?limit=100&q=${encodeURIComponent(query.trim())}`,
          {
            signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]),
          },
        );
        if (!controller.signal.aborted) {
          setSkills(data.items);
          setStatus("ready");
        }
      } catch {
        if (!controller.signal.aborted) setStatus("error");
      }
    }, 200);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query, attempt]);
  const available = skills.filter((skill) => !existing.includes(skill.id));
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!pending && available.some((skill) => skill.id === skillId)) onAdd(skillId, level);
  }
  return (
    <form className="profile-add-skill" onSubmit={submit} aria-label="Додати навичку">
      <fieldset className="profile-fields" disabled={pending}>
        <label className="register-field">
          Знайти навичку
          <input
            type="search"
            value={query}
            maxLength={150}
            placeholder="Наприклад, Figma"
            onChange={(event) => {
              setQuery(event.target.value);
              setSkillId("");
              setStatus("loading");
            }}
          />
        </label>
        {status === "loading" ? (
          <p role="status">Завантажуємо каталог…</p>
        ) : status === "error" ? (
          <div role="alert">
            <p>Не вдалося завантажити каталог.</p>
            <button
              className="profile-text-button"
              type="button"
              onClick={() => {
                setStatus("loading");
                setAttempt((value) => value + 1);
              }}
            >
              Спробувати ще раз
            </button>
          </div>
        ) : available.length ? (
          <div className="profile-add-skill__fields">
            <label className="register-field">
              Навичка зі спільного каталогу
              <select value={skillId} required onChange={(event) => setSkillId(event.target.value)}>
                <option value="">Обери навичку</option>
                {available.map((skill) => (
                  <option value={skill.id} key={skill.id}>
                    {skill.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="register-field">
              Мій поточний рівень
              <select
                value={level}
                onChange={(event) => setLevel(event.target.value as SkillLevel)}
              >
                {Object.entries(skillLevels).map(([value, label]) => (
                  <option value={value} key={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
          </div>
        ) : (
          <p role="status">
            {query.trim()
              ? "Навичок не знайдено. Спробуй іншу назву."
              : "Усі знайдені навички вже є в цьому списку."}
          </p>
        )}
        <div className="profile-actions">
          <button className="dashboard-button" type="button" onClick={onClose}>
            Скасувати
          </button>
          <button
            className="dashboard-button dashboard-button--primary"
            disabled={status !== "ready" || !available.some((skill) => skill.id === skillId)}
            type="submit"
          >
            {pending ? "Додаємо…" : "Додати"}
          </button>
        </div>
      </fieldset>
    </form>
  );
}
