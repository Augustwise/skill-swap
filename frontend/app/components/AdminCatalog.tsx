"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { authRequest } from "./AuthApi";
import { AppDialog } from "./AppDialog";
import {
  adminHref,
  catalogSkills,
  saveSkill,
  skillFields,
  type CatalogSkill,
  type SkillDraft,
} from "./AdminApi";
import { pageCount } from "./DiscoveryApi";
import { Pagination } from "./DiscoveryCards";
import type { Reference } from "./OnboardingApi";
import { FieldError, SafetyFeedback } from "./SafetyFeedback";
import { useApiQuery } from "./useApiQuery";
import { useSafetyMutation } from "./useSafetyMutation";

export function AdminCatalog({
  q,
  page,
  csrfToken,
}: {
  q: string;
  page: number;
  csrfToken: string;
}) {
  const router = useRouter();
  const path = adminHref("skills", { q, page });
  const query = useApiQuery(path, (signal) => catalogSkills(q, page, signal));
  const [editor, setEditor] = useState<CatalogSkill | "new" | null>(null);
  const [notice, setNotice] = useState("");
  const data = query.state.status === "ready" ? query.state.data : null;
  return (
    <section className="account-card safety-card">
      <div className="safety-row__heading">
        <h2>Каталог навичок{data ? ` · ${data.total}` : ""}</h2>
        <button
          type="button"
          className="dashboard-button dashboard-button--primary"
          aria-haspopup="dialog"
          onClick={() => setEditor("new")}
        >
          Додати навичку
        </button>
      </div>
      <p className="profile-hint">
        Приховані навички недоступні для нового підбору. Погоджені назви в чинних обмінах
        зберігаються.
      </p>
      <form
        className="safety-search"
        key={q}
        role="search"
        onSubmit={(event) => {
          event.preventDefault();
          const value = new FormData(event.currentTarget).get("q");
          router.push(adminHref("skills", { q: String(value ?? "") }));
        }}
      >
        <label className="safety-field">
          Пошук у каталозі
          <input
            type="search"
            name="q"
            defaultValue={q}
            maxLength={100}
            placeholder="Назва навички…"
          />
        </label>
        <button className="dashboard-button">Знайти</button>
      </form>
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {query.state.status === "loading" && <p role="status">Завантажуємо каталог…</p>}
      {query.state.status === "error" && (
        <SafetyFeedback error={query.state.error} path={path} onRetry={query.retry} />
      )}
      {data && (
        <>
          {!data.items.length && (
            <p className="safety-empty">
              Навичок на цій сторінці немає. Зміни пошук або додай нову навичку.
            </p>
          )}
          <ul className="safety-list">
            {data.items.map((skill) => (
              <li className="safety-row" key={skill.id}>
                <div>
                  <div className="safety-row__heading">
                    <h3>{skill.name}</h3>
                    <span
                      className={`safety-badge ${skill.isActive ? "safety-badge--resolved" : ""}`}
                    >
                      {skill.isActive ? "Активна" : "Прихована"}
                    </span>
                  </div>
                  <p className="profile-hint">
                    {skill.categoryName} · {skill.slug}
                  </p>
                  {skill.description && <p className="safety-text">{skill.description}</p>}
                </div>
                <button
                  className="dashboard-button"
                  type="button"
                  aria-label={`Редагувати ${skill.name}`}
                  aria-haspopup="dialog"
                  onClick={() => setEditor(skill)}
                >
                  Редагувати
                </button>
              </li>
            ))}
          </ul>
          <Pagination
            page={data.page}
            pages={Math.max(pageCount(data.total, data.pageSize), data.page)}
            hrefFor={(number) => adminHref("skills", { q, page: number })}
          />
        </>
      )}
      {editor && (
        <SkillEditor
          skill={editor === "new" ? null : editor}
          csrfToken={csrfToken}
          path={path}
          onClose={() => setEditor(null)}
          onDone={(name) => {
            setEditor(null);
            setNotice(`Навичку «${name}» збережено. Дію записано в журнал.`);
            query.retry();
          }}
        />
      )}
    </section>
  );
}

function SkillEditor({
  skill,
  csrfToken,
  path,
  onClose,
  onDone,
}: {
  skill: CatalogSkill | null;
  csrfToken: string;
  path: string;
  onClose: () => void;
  onDone: (name: string) => void;
}) {
  const [draft, setDraft] = useState<SkillDraft>(() =>
    skill
      ? {
          categoryId: skill.categoryId,
          name: skill.name,
          slug: skill.slug,
          description: skill.description,
          isActive: skill.isActive,
        }
      : { categoryId: "", name: "", slug: "", description: "", isActive: true },
  );
  const categories = useApiQuery("skill-categories", (signal) =>
    authRequest<{ items: Reference[] }>("skill-categories", { signal }),
  );
  const mutation = useSafetyMutation();
  const fields = [
    { name: "name", label: "Назва навички" },
    { name: "slug", label: "Ідентифікатор навички" },
    { name: "description", label: "Опис (необов’язково)" },
  ] as const;
  return (
    <AppDialog
      title={skill ? "Редагувати навичку" : "Нова навичка"}
      description="Зміни застосовуються до спільного каталогу. Усі дії зберігаються в журналі."
      canClose={!mutation.pending}
      onClose={onClose}
    >
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          const form = event.currentTarget;
          if (!mutation.validate(form, skillFields(draft))) return;
          const result = await mutation.run(
            () => saveSkill(skill?.id ?? null, draft, csrfToken),
            form,
          );
          if (result.ok) onDone(result.value.skill.name);
        }}
      >
        <label className="safety-field">
          Категорія
          <select
            name="categoryId"
            aria-label="Категорія"
            value={draft.categoryId}
            disabled={mutation.pending || categories.state.status !== "ready"}
            aria-invalid={Boolean(mutation.fields.categoryId)}
            aria-describedby="skill-category-error"
            onChange={(event) => setDraft({ ...draft, categoryId: event.target.value })}
          >
            <option value="">Обери категорію</option>
            {skill &&
              categories.state.status === "ready" &&
              !categories.state.data.items.some((item) => item.id === skill.categoryId) && (
                <option value={skill.categoryId}>{skill.categoryName}</option>
              )}
            {categories.state.status === "ready" &&
              categories.state.data.items.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
          </select>
          <FieldError id="skill-category-error" message={mutation.fields.categoryId} />
        </label>
        {categories.state.status === "loading" && <p role="status">Завантажуємо категорії…</p>}
        {categories.state.status === "error" && (
          <SafetyFeedback error={categories.state.error} path={path} onRetry={categories.retry} />
        )}
        {fields.map(({ name, label }) => (
          <label className="safety-field" key={name}>
            {label}
            {name === "description" ? (
              <textarea
                name={name}
                rows={3}
                value={draft[name]}
                disabled={mutation.pending}
                aria-invalid={Boolean(mutation.fields[name])}
                aria-describedby={`skill-${name}-error`}
                onChange={(event) => setDraft({ ...draft, [name]: event.target.value })}
              />
            ) : (
              <input
                name={name}
                value={draft[name]}
                disabled={mutation.pending}
                aria-invalid={Boolean(mutation.fields[name])}
                aria-describedby={`skill-${name}-error${name === "slug" ? " slug-help" : ""}`}
                onChange={(event) => setDraft({ ...draft, [name]: event.target.value })}
              />
            )}
            {name === "slug" && (
              <span id="slug-help" className="profile-hint">
                Малі латинські літери, цифри й одинарні дефіси: web-design.
              </span>
            )}
            <FieldError id={`skill-${name}-error`} message={mutation.fields[name]} />
          </label>
        ))}
        <label className="safety-check">
          <input
            type="checkbox"
            checked={draft.isActive}
            disabled={mutation.pending}
            onChange={(event) => setDraft({ ...draft, isActive: event.target.checked })}
          />{" "}
          Показувати навичку в каталозі
        </label>
        <SafetyFeedback error={mutation.error} path={path} />
        <div className="safety-actions safety-actions--end">
          <button
            type="button"
            className="dashboard-button"
            data-autofocus
            disabled={mutation.pending}
            onClick={onClose}
          >
            Скасувати
          </button>
          <button
            className="dashboard-button dashboard-button--primary"
            disabled={mutation.pending || categories.state.status !== "ready"}
          >
            {mutation.pending ? "Зберігаємо…" : "Зберегти навичку"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
