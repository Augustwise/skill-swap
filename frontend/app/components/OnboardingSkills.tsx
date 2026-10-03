import Image from "next/image";
import type { Reference, SelectedSkill, Skill, SkillLevel } from "./OnboardingApi";

export function OnboardingIcon({ name, size }: { name: string; size: number }) {
  return <Image src={`/figma/onboarding/${name}.svg`} alt="" width={size} height={size} />;
}

export function OnboardingSkills({
  teaching,
  categories,
  category,
  query,
  skills,
  loading,
  catalogError,
  selected,
  onCategory,
  onQuery,
  onChange,
  onNotice,
  onRetry,
}: {
  teaching: boolean;
  categories: Reference[];
  category: string;
  query: string;
  skills: Skill[];
  loading: boolean;
  catalogError: string;
  selected: SelectedSkill[];
  onCategory: (id: string) => void;
  onQuery: (query: string) => void;
  onChange: (skills: SelectedSkill[]) => void;
  onNotice: (notice: string) => void;
  onRetry: () => void;
}) {
  function toggleSkill(skill: Skill) {
    onNotice("");
    if (selected.some((item) => item.id === skill.id))
      onChange(selected.filter((item) => item.id !== skill.id));
    else if (selected.length < 5) onChange([...selected, { ...skill, level: "BEGINNER" }]);
    else onNotice("Можна обрати не більше 5 навичок.");
  }

  function moveSkill(from: number, to: number) {
    if (from === to || from < 0 || to < 0 || from >= selected.length || to >= selected.length)
      return;
    const reordered = [...selected];
    const [item] = reordered.splice(from, 1);
    reordered.splice(to, 0, item);
    onChange(reordered);
  }

  const visibleSkills = skills.filter((skill) => !category || skill.categoryId === category);
  const categoryName = categories.find((item) => item.id === category)?.name;

  return (
    <>
      <div className="onboarding-intro">
        <h1 id="onboarding-title" tabIndex={-1}>
          {teaching ? "Чого ти можеш навчити?" : "Чого ти хочеш навчитися?"}
        </h1>
        <p>
          {teaching
            ? "Обери 1–5 навичок, якими можеш поділитися з іншими студентами, та вкажи свій рівень."
            : "Обери 1–5 навичок. Ми одразу покажемо студентів, які вміють це і водночас хочуть навчитися того, що вмієш ти."}
        </p>
      </div>
      <label className="onboarding-search">
        <OnboardingIcon name="search" size={18} />
        <span className="sr-only">Знайти навичку</span>
        <input
          type="search"
          value={query}
          maxLength={150}
          onChange={(event) => onQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") event.preventDefault();
          }}
          placeholder="Знайти навичку — наприклад, Photoshop"
        />
      </label>
      <div className="onboarding-categories" role="group" aria-label="Категорії навичок">
        {[{ id: "", name: "Усі" }, ...categories].map((item) => (
          <button
            type="button"
            className={`onboarding-category ${category === item.id ? "is-active" : ""}`}
            aria-pressed={category === item.id}
            onClick={() => onCategory(item.id)}
            key={item.id}
          >
            {item.name}
          </button>
        ))}
      </div>
      <div className="onboarding-skill-list" aria-busy={loading}>
        <h2>
          {query.trim()
            ? "Результати пошуку"
            : categoryName
              ? `Популярні навички в категорії «${categoryName}»`
              : "Популярні навички"}
        </h2>
        <div className="onboarding-skill-list__chips">
          {loading ? (
            <p className="onboarding-empty" role="status">
              Завантажуємо навички…
            </p>
          ) : catalogError ? (
            <div className="onboarding-catalog-error" role="alert">
              <p>{catalogError}</p>
              <button
                type="button"
                className="onboarding-button onboarding-button--secondary"
                onClick={onRetry}
              >
                Спробувати ще раз
              </button>
            </div>
          ) : (
            visibleSkills.map((skill) => {
              const isSelected = selected.some((item) => item.id === skill.id);
              return (
                <button
                  type="button"
                  className={`onboarding-skill ${isSelected ? "is-selected" : ""}`}
                  aria-pressed={isSelected}
                  onClick={() => toggleSkill(skill)}
                  key={skill.id}
                >
                  <OnboardingIcon name={isSelected ? "check-selected" : "plus"} size={15} />
                  {skill.name}
                </button>
              );
            })
          )}
          {!loading && !catalogError && !visibleSkills.length && (
            <p className="onboarding-empty">
              {query.trim()
                ? "Навичок не знайдено. Спробуй інший запит або категорію."
                : "У цій категорії навичок поки немає."}
            </p>
          )}
        </div>
      </div>
      <div className="onboarding-selected">
        <p aria-live="polite">
          Обрано {selected.length} з 5{!teaching && " · перетягни, щоб змінити пріоритет"}
        </p>
        {!selected.length && (
          <p className="onboarding-empty">
            {teaching
              ? "Додай навички, якими хочеш поділитися."
              : "Додай навички, які хочеш опанувати."}
          </p>
        )}
        {selected.map((skill, index) => (
          <div
            className={`onboarding-selected__row ${teaching ? "onboarding-selected__row--teaching" : ""}`}
            key={skill.id}
            draggable={!teaching}
            onDragStart={(event) =>
              event.dataTransfer.setData("application/x-skillswap-skill", skill.id)
            }
            onDragOver={(event) => {
              if (!teaching) event.preventDefault();
            }}
            onDrop={(event) => {
              if (teaching) return;
              event.preventDefault();
              const id = event.dataTransfer.getData("application/x-skillswap-skill");
              moveSkill(
                selected.findIndex((item) => item.id === id),
                index,
              );
            }}
          >
            <span className="onboarding-selected__rank">{index + 1}</span>
            <span className="onboarding-selected__name">{skill.name}</span>
            <label className="onboarding-level">
              <span className="sr-only" id={`onboarding-level-${skill.id}`}>
                Мій рівень для {skill.name}
              </span>
              <select
                aria-labelledby={`onboarding-level-${skill.id}`}
                value={skill.level}
                onChange={(event) =>
                  onChange(
                    selected.map((item) =>
                      item.id === skill.id
                        ? { ...item, level: event.target.value as SkillLevel }
                        : item,
                    ),
                  )
                }
              >
                <option value="BEGINNER">Мій рівень: Початковий</option>
                <option value="INTERMEDIATE">Мій рівень: Середній</option>
                <option value="ADVANCED">Мій рівень: Просунутий</option>
              </select>
              <OnboardingIcon name="chevron-down" size={14} />
            </label>
            {!teaching && (
              <span className="onboarding-reorder">
                <button
                  type="button"
                  aria-label={`Підняти ${skill.name}`}
                  disabled={index === 0}
                  onClick={() => moveSkill(index, index - 1)}
                >
                  ↑
                </button>
                <button
                  type="button"
                  aria-label={`Опустити ${skill.name}`}
                  disabled={index === selected.length - 1}
                  onClick={() => moveSkill(index, index + 1)}
                >
                  ↓
                </button>
              </span>
            )}
            <button
              className="onboarding-remove"
              type="button"
              aria-label={`Прибрати ${skill.name}`}
              onClick={() => toggleSkill(skill)}
            >
              <OnboardingIcon name="close" size={16} />
            </button>
          </div>
        ))}
      </div>
    </>
  );
}
