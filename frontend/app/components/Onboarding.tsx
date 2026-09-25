"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

const asset = "/figma/onboarding";
const categories = ["Усі", "Дизайн", "Програмування", "Мови", "Музика", "Інше"];
const skills = [
  { name: "Adobe Photoshop", category: "Дизайн" },
  { name: "Figma", category: "Дизайн" },
  { name: "Illustrator", category: "Дизайн" },
  { name: "UI/UX-дизайн", category: "Дизайн" },
  { name: "Web-дизайн", category: "Дизайн" },
  { name: "Motion-дизайн", category: "Дизайн" },
  { name: "3D · Blender", category: "Дизайн" },
  { name: "Ілюстрація", category: "Дизайн" },
  { name: "Типографіка", category: "Дизайн" },
  { name: "Брендинг", category: "Дизайн" },
  { name: "Go", category: "Програмування" },
  { name: "JavaScript", category: "Програмування" },
  { name: "Англійська", category: "Мови" },
  { name: "Гітара", category: "Музика" },
];

type Verification = "ready" | "pending" | "verified" | "error";

function Icon({ name, size }: { name: string; size: number }) {
  return <Image src={`${asset}/${name}.svg`} alt="" width={size} height={size} />;
}

export function Onboarding() {
  const [verification, setVerification] = useState<Verification>("ready");
  const verificationStarted = useRef(false);
  const [category, setCategory] = useState("Дизайн");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(["Adobe Photoshop", "Figma"]);
  const [levels, setLevels] = useState<Record<string, string>>({});
  const [step, setStep] = useState<"skills" | "format">("skills");
  const [format, setFormat] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    if (verificationStarted.current) return;
    verificationStarted.current = true;
    const token = new URLSearchParams(window.location.search).get("token");
    if (!token) return;

    void (async () => {
      setVerification("pending");
      try {
        const response = await fetch("/api/v1/auth/verify-email", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });
        if (!response.ok) throw new Error("Verification failed");
        window.history.replaceState(null, "", "/onboarding");
        setVerification("verified");
      } catch {
        setVerification("error");
      }
    })();
  }, []);

  function toggleSkill(name: string) {
    setNotice("");
    if (selected.includes(name)) {
      setSelected((current) => current.filter((item) => item !== name));
    } else if (selected.length < 5) {
      setSelected((current) => [...current, name]);
    } else {
      setNotice("Можна обрати не більше 5 навичок.");
    }
  }

  function moveSkill(from: number, to: number) {
    if (from === to) return;
    setSelected((current) => {
      const reordered = [...current];
      const [item] = reordered.splice(from, 1);
      reordered.splice(to, 0, item);
      return reordered;
    });
  }

  const visibleSkills = skills.filter(
    (skill) =>
      (category === "Усі" || skill.category === category) &&
      skill.name.toLocaleLowerCase("uk").includes(query.trim().toLocaleLowerCase("uk")),
  );

  if (verification === "pending" || verification === "error") {
    return (
      <main className="onboarding-page onboarding-page--message">
        <div className="onboarding-message" role={verification === "error" ? "alert" : "status"}>
          <Image src={`${asset}/mark.svg`} alt="" width={30} height={30} />
          <h1>{verification === "pending" ? "Підтверджуємо пошту…" : "Посилання не працює"}</h1>
          <p>
            {verification === "pending"
              ? "Зачекай, поки ми перевіримо посилання з листа."
              : "Посилання недійсне або вже використане. Увійди в акаунт, щоб продовжити."}
          </p>
          {verification === "error" && <Link href="/login">Перейти до входу</Link>}
        </div>
      </main>
    );
  }

  return (
    <main className="onboarding-page">
      <header className="onboarding-header">
        <Link className="onboarding-logo" href="/" aria-label="SkillSwap — головна">
          <Image src={`${asset}/mark.svg`} alt="" width={30} height={30} />
          <span>SkillSwap</span>
        </Link>
        <span className="onboarding-header__step">Крок {step === "skills" ? "2" : "4"} з 4</span>
        <Link className="onboarding-skip" href="/">
          Пропустити
        </Link>
      </header>

      <div className="onboarding-layout">
        <section className="onboarding-card" aria-labelledby="onboarding-title">
          <ol className="onboarding-progress" aria-label="Етапи онбордингу">
            {[
              { label: "Профіль", state: "complete" },
              { label: "Що я вмію", state: "complete" },
              { label: "Що хочу вивчити", state: step === "skills" ? "active" : "complete" },
              { label: "Формат", state: step === "format" ? "active" : "future" },
            ].map((item, index) => (
              <li
                className={`onboarding-progress__item onboarding-progress__item--${item.state}`}
                key={item.label}
              >
                <span className="onboarding-progress__circle">
                  {item.state === "complete" ? <Icon name="check" size={14} /> : index + 1}
                </span>
                <span>{item.label}</span>
                {index < 3 && <span className="onboarding-progress__line" aria-hidden="true" />}
              </li>
            ))}
          </ol>

          {step === "skills" ? (
            <>
              <div className="onboarding-intro">
                <h1 id="onboarding-title">Чого ти хочеш навчитися?</h1>
                <p>
                  Обери 1–5 навичок. Ми одразу покажемо студентів, які вміють це і водночас хочуть
                  навчитися того, що вмієш ти.
                </p>
              </div>

              <label className="onboarding-search">
                <Icon name="search" size={18} />
                <span className="sr-only">Знайти навичку</span>
                <input
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Знайти навичку — наприклад, Photoshop"
                />
              </label>

              <div className="onboarding-categories" role="group" aria-label="Категорії навичок">
                {categories.map((item) => (
                  <button
                    type="button"
                    className={`onboarding-category ${category === item ? "is-active" : ""}`}
                    aria-pressed={category === item}
                    onClick={() => setCategory(item)}
                    key={item}
                  >
                    {item}
                  </button>
                ))}
              </div>

              <div className="onboarding-skill-list">
                <h2>
                  {query
                    ? "Результати пошуку"
                    : category === "Усі"
                      ? "Популярні навички"
                      : `Популярні навички в категорії «${category}»`}
                </h2>
                <div className="onboarding-skill-list__chips">
                  {visibleSkills.map((skill) => {
                    const isSelected = selected.includes(skill.name);
                    return (
                      <button
                        type="button"
                        className={`onboarding-skill ${isSelected ? "is-selected" : ""}`}
                        aria-pressed={isSelected}
                        onClick={() => toggleSkill(skill.name)}
                        key={skill.name}
                      >
                        <Icon name={isSelected ? "check-selected" : "plus"} size={15} />
                        {skill.name}
                      </button>
                    );
                  })}
                  {visibleSkills.length === 0 && (
                    <p className="onboarding-empty">У цій категорії навичок поки немає.</p>
                  )}
                </div>
              </div>

              <div className="onboarding-selected">
                <p>Обрано {selected.length} з 5 · перетягни, щоб змінити пріоритет</p>
                {selected.map((name, index) => (
                  <div
                    className="onboarding-selected__row"
                    key={name}
                    draggable
                    onDragStart={(event) => event.dataTransfer.setData("text/plain", String(index))}
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={(event) => {
                      event.preventDefault();
                      const from = Number(event.dataTransfer.getData("text/plain"));
                      if (Number.isInteger(from) && from >= 0 && from < selected.length) {
                        moveSkill(from, index);
                      }
                    }}
                  >
                    <span className="onboarding-selected__rank">{index + 1}</span>
                    <span className="onboarding-selected__name">{name}</span>
                    <label className="onboarding-level">
                      <span className="sr-only">Мій рівень для {name}</span>
                      <select
                        value={levels[name] ?? "Початковий"}
                        onChange={(event) =>
                          setLevels((current) => ({ ...current, [name]: event.target.value }))
                        }
                      >
                        <option value="Початковий">Мій рівень: Початковий</option>
                        <option value="Середній">Мій рівень: Середній</option>
                        <option value="Просунутий">Мій рівень: Просунутий</option>
                      </select>
                      <Icon name="chevron-down" size={14} />
                    </label>
                    <button
                      className="onboarding-remove"
                      type="button"
                      aria-label={`Прибрати ${name}`}
                      onClick={() => toggleSkill(name)}
                    >
                      <Icon name="close" size={16} />
                    </button>
                  </div>
                ))}
              </div>
              {notice && (
                <p className="onboarding-notice" role="alert">
                  {notice}
                </p>
              )}
              <div className="onboarding-actions">
                <Link className="onboarding-button onboarding-button--secondary" href="/register">
                  Назад
                </Link>
                <button
                  type="button"
                  className="onboarding-button onboarding-button--primary"
                  onClick={() => {
                    if (selected.length === 0) setNotice("Обери хоча б одну навичку.");
                    else setStep("format");
                  }}
                >
                  <Icon name="arrow-right" size={18} />
                  Далі — обрати формат
                </button>
              </div>
            </>
          ) : (
            <>
              <div className="onboarding-intro">
                <h1 id="onboarding-title">Як тобі зручно навчатися?</h1>
                <p>Обери формат зустрічей для обміну навичками.</p>
              </div>
              <div className="onboarding-formats" role="group" aria-label="Формат зустрічей">
                {["Онлайн", "Офлайн", "Обидва формати"].map((item) => (
                  <button
                    type="button"
                    className={`onboarding-format ${format === item ? "is-selected" : ""}`}
                    aria-pressed={format === item}
                    onClick={() => setFormat(item)}
                    key={item}
                  >
                    {item}
                  </button>
                ))}
              </div>
              <div className="onboarding-actions">
                <button
                  type="button"
                  className="onboarding-button onboarding-button--secondary"
                  onClick={() => setStep("skills")}
                >
                  Назад
                </button>
                <Link className="onboarding-button onboarding-button--primary" href="/">
                  Завершити
                </Link>
              </div>
            </>
          )}
        </section>

        <aside className="onboarding-sidebar" aria-label="Підказки для онбордингу">
          <div className="onboarding-matches">
            <div className="onboarding-matches__heading">
              <span className="onboarding-matches__icon">
                <Icon name="sparkles" size={16} />
              </span>
              <h2>Уже 12 збігів</h2>
            </div>
            <p>
              Стільки студентів вміють Photoshop або Figma і водночас хочуть навчитися гри на
              гітарі.
            </p>
            {[
              { initials: "ОК", name: "Олена Ковальчук", match: "94%" },
              { initials: "МГ", name: "Марія Гнатюк", match: "91%" },
              { initials: "ТЛ", name: "Тарас Левченко", match: "85%" },
            ].map((person, index) => (
              <div className="onboarding-match" key={person.name}>
                <span className={`onboarding-avatar onboarding-avatar--${index}`}>
                  {person.initials}
                </span>
                <span className="onboarding-match__name">{person.name}</span>
                <span className="onboarding-match__score">{person.match}</span>
              </div>
            ))}
            <small>+ ще 9 після завершення реєстрації</small>
          </div>
          <div className="onboarding-tip">
            <span className="onboarding-tip__icon">
              <Icon name="award" size={16} />
            </span>
            <div>
              <h2>Порада</h2>
              <p>Обирай 2–3 конкретні навички замість однієї загальної — так збігів буде більше.</p>
            </div>
          </div>
        </aside>
      </div>
      {verification === "verified" && (
        <span className="sr-only" role="status">
          Пошту підтверджено.
        </span>
      )}
    </main>
  );
}
