"use client";

import Image from "next/image";
import { useId, useState, type FormEvent, type ReactNode } from "react";
import {
  MAX_QUERY_LENGTH,
  emptyFilters,
  hasActiveFilters,
  type LessonFormat,
  type SearchFilters as Filters,
  type SkillCategory,
} from "./DiscoveryApi";
import { formatLabels } from "./DiscoveryFormat";
import type { SkillLevel } from "./OnboardingApi";
import { skillLevels } from "./ProfileApi";
import type { QueryState } from "./useApiQuery";

function Select({
  label,
  value,
  disabled,
  onChange,
  children,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <label className="register-field register-field--select">
      {label}
      <select value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
        {children}
      </select>
      <Image src="/figma/register/chevron-down.svg" alt="" width={16} height={16} />
    </label>
  );
}

// Selects apply at once; the text query applies on submit, together with all filters.
export function SearchFilters({
  filters,
  categories,
  onRetryCategories,
  onApply,
}: {
  filters: Filters;
  categories: QueryState<SkillCategory[]>;
  onRetryCategories: () => void;
  onApply: (filters: Filters) => void;
}) {
  const [draft, setDraft] = useState(filters);
  const hintId = useId();
  const change = (patch: Partial<Filters>) => {
    const next = { ...draft, ...patch };
    setDraft(next);
    onApply(next);
  };
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onApply(draft);
  }

  return (
    <form className="account-card discovery-filters" role="search" onSubmit={submit}>
      <div className="discovery-filters__query">
        <label className="register-field">
          Навичка або ім’я
          <input
            type="search"
            name="q"
            value={draft.q}
            maxLength={MAX_QUERY_LENGTH}
            placeholder="Наприклад, Photoshop або Ольга"
            aria-describedby={hintId}
            onChange={(event) => setDraft({ ...draft, q: event.target.value })}
          />
        </label>
        <button className="dashboard-button dashboard-button--primary" type="submit">
          Знайти
        </button>
      </div>
      <p className="profile-hint discovery-hint" id={hintId}>
        Шукаємо за назвою навички, якої студент навчає, або за ім’ям. Усі фільтри діють разом.
      </p>
      <div className="discovery-filters__grid">
        <Select
          label="Категорія"
          value={draft.categoryId}
          disabled={categories.status !== "ready"}
          onChange={(categoryId) => change({ categoryId })}
        >
          <option value="">
            {categories.status === "loading" ? "Завантажуємо категорії…" : "Усі категорії"}
          </option>
          {categories.status === "ready" &&
            categories.data.map((category) => (
              <option value={category.id} key={category.id}>
                {category.name}
              </option>
            ))}
        </Select>
        <Select
          label="Рівень навички"
          value={draft.level}
          onChange={(level) => change({ level: level as SkillLevel | "" })}
        >
          <option value="">Будь-який рівень</option>
          {Object.entries(skillLevels).map(([value, label]) => (
            <option value={value} key={value}>
              {label}
            </option>
          ))}
        </Select>
        <Select
          label="Формат занять"
          value={draft.format}
          onChange={(format) => change({ format: format as LessonFormat | "" })}
        >
          <option value="">Будь-який формат</option>
          {Object.entries(formatLabels).map(([value, label]) => (
            <option value={value} key={value}>
              {label}
            </option>
          ))}
        </Select>
        <label className="profile-format discovery-mutual">
          <input
            type="checkbox"
            checked={draft.mutual}
            onChange={(event) => change({ mutual: event.target.checked })}
          />
          Лише взаємні збіги
        </label>
      </div>
      {categories.status === "error" && (
        <p className="discovery-hint" role="alert">
          Не вдалося завантажити категорії.
          <button className="profile-text-button" type="button" onClick={onRetryCategories}>
            Спробувати ще раз
          </button>
        </p>
      )}
      {hasActiveFilters(filters) && (
        <div className="profile-actions">
          <button className="dashboard-button" type="button" onClick={() => onApply(emptyFilters)}>
            Скинути фільтри
          </button>
        </div>
      )}
    </form>
  );
}
