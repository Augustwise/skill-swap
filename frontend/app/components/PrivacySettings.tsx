"use client";

import { useState } from "react";
import { privacySettings, updatePrivacy, type Privacy } from "./SafetyApi";
import { SafetyFeedback } from "./SafetyFeedback";
import { useSafetyMutation } from "./useSafetyMutation";
import { useApiQuery } from "./useApiQuery";

export function PrivacySettings({ csrfToken }: { csrfToken: string }) {
  const query = useApiQuery("me/privacy", privacySettings);
  return (
    <section className="account-card safety-card">
      <h2>Видимість і нові запити</h2>
      <p className="profile-hint">Обирай, хто може знайти тебе та запропонувати обмін.</p>
      {query.state.status === "loading" && <p role="status">Завантажуємо налаштування…</p>}
      {query.state.status === "error" && (
        <SafetyFeedback error={query.state.error} path="/settings" onRetry={query.retry} />
      )}
      {query.state.status === "ready" && (
        <PrivacyForm initial={query.state.data} csrfToken={csrfToken} />
      )}
    </section>
  );
}

function PrivacyForm({ initial, csrfToken }: { initial: Privacy; csrfToken: string }) {
  const [value, setValue] = useState(initial);
  const [saved, setSaved] = useState(initial);
  const [notice, setNotice] = useState("");
  const mutation = useSafetyMutation();
  const dirty = JSON.stringify(value) !== JSON.stringify(saved);
  return (
    <form
      className="safety-form"
      onSubmit={async (event) => {
        event.preventDefault();
        setNotice("");
        const result = await mutation.run(
          () => updatePrivacy(value, csrfToken),
          event.currentTarget,
        );
        if (result.ok) {
          setValue(result.value);
          setSaved(result.value);
          setNotice("Налаштування приватності збережено.");
        }
      }}
    >
      <label className="safety-toggle">
        <span>
          <strong>Показувати мій профіль</strong>
          <span>У пошуку, взаємних збігах і для нових пропозицій обміну.</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          name="isDiscoverable"
          checked={value.isDiscoverable}
          disabled={mutation.pending}
          onChange={(event) => {
            setValue({ ...value, isDiscoverable: event.target.checked });
            setNotice("");
          }}
        />
      </label>
      {!value.isDiscoverable && (
        <p className="safety-info">
          Після збереження профіль зникне з пошуку й нові запити стануть недоступними. Чинні обміни,
          заняття та чати збережуться.
        </p>
      )}
      <label className="safety-toggle">
        <span>
          <strong>Приймати нові запити</strong>
          <span>Вимкни, якщо поки не плануєш нових обмінів.</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          name="acceptsRequests"
          checked={value.acceptsRequests}
          disabled={mutation.pending}
          onChange={(event) => {
            setValue({ ...value, acceptsRequests: event.target.checked });
            setNotice("");
          }}
        />
      </label>
      <label className="safety-field">
        Хто може надсилати запити
        <select
          name="requestScope"
          value={value.requestScope}
          disabled={mutation.pending}
          onChange={(event) => {
            setValue({ ...value, requestScope: event.target.value as Privacy["requestScope"] });
            setNotice("");
          }}
        >
          <option value="ANYONE">Усі студенти</option>
          <option value="SAME_UNIVERSITY">Лише студенти мого університету</option>
        </select>
      </label>
      <SafetyFeedback error={mutation.error} path="/settings" />
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      <div className="safety-actions">
        <button
          className="dashboard-button dashboard-button--primary"
          disabled={mutation.pending || !dirty}
        >
          {mutation.pending ? "Зберігаємо…" : "Зберегти налаштування"}
        </button>
      </div>
    </form>
  );
}
