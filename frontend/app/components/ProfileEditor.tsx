import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { AuthError, authRequest, requestError } from "./AuthApi";
import { profileFields, type Profile, type ProfileFields, type Reference } from "./OnboardingApi";
import { OnboardingProfile } from "./OnboardingProfile";
import { profileErrors, saveProfile, validateProfile, type ProfileDraft } from "./ProfileApi";

export function ProfileEditor({
  profile,
  csrfToken,
  onSaved,
}: {
  profile: Profile;
  csrfToken: string;
  onSaved: (profile: Profile) => void;
}) {
  const initial = (): ProfileDraft => toDraft(profile);
  const [draft, setDraft] = useState(initial);
  const [universities, setUniversities] = useState<Reference[]>([profile.university]);
  const [faculties, setFaculties] = useState<Reference[]>(profile.faculty ? [profile.faculty] : []);
  const [facultyStatus, setFacultyStatus] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [expired, setExpired] = useState(false);
  const saving = useRef(false);
  const dirty = JSON.stringify(draft) !== JSON.stringify(initial());

  useEffect(() => {
    const controller = new AbortController();
    authRequest<{ items: Reference[] }>("universities", {
      signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]),
    })
      .then((data) => setUniversities(data.items))
      .catch(() => {});
    return () => controller.abort();
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    authRequest<{ items: Reference[] }>(`universities/${draft.universityId}/faculties`, {
      signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]),
    })
      .then((data) => {
        if (!controller.signal.aborted) {
          setFaculties(data.items);
          setFacultyStatus("ready");
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) setFacultyStatus("error");
      });
    return () => controller.abort();
  }, [draft.universityId, attempt]);

  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  function changeUniversity(universityId: string) {
    setDraft((current) => ({ ...current, universityId, facultyId: "" }));
    setFaculties([]);
    setFacultyStatus("loading");
    setErrors((current) => ({ ...current, universityId: "", facultyId: "" }));
    setNotice("");
  }

  function change(field: keyof ProfileFields, value: string) {
    setDraft((current) => ({ ...current, [field]: value }));
    setErrors((current) => ({ ...current, [field]: "" }));
    setNotice("");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving.current) return;
    const invalid = validateProfile(draft);
    setErrors(invalid);
    setNotice("");
    if (Object.keys(invalid).length) {
      setNotice("Перевір виділені поля перед збереженням.");
      (event.currentTarget.elements.namedItem(Object.keys(invalid)[0]) as HTMLElement)?.focus();
      return;
    }
    saving.current = true;
    setPending(true);
    try {
      const saved = await saveProfile(draft, csrfToken);
      onSaved(saved);
      setDraft(toDraft(saved));
      setNotice("Зміни збережено.");
    } catch (error) {
      setErrors(profileErrors(error));
      setExpired(error instanceof AuthError && error.status === 401);
      setNotice(
        error instanceof AuthError && error.status === 422
          ? "Перевір виділені поля перед збереженням."
          : requestError(error),
      );
    } finally {
      saving.current = false;
      setPending(false);
    }
  }

  return (
    <form
      className="profile-editor"
      noValidate
      onSubmit={submit}
      aria-label="Редагування профілю"
      aria-busy={pending}
    >
      <section className="account-card">
        <div>
          <h2>Основна інформація</h2>
          <p className="profile-hint">Ці дані бачать студенти у твоєму профілі</p>
        </div>
        {facultyStatus !== "ready" && (
          <div role="status" className="profile-hint">
            {facultyStatus === "loading"
              ? "Завантажуємо факультети…"
              : "Не вдалося завантажити факультети."}
            {facultyStatus === "error" && (
              <button
                type="button"
                className="profile-text-button"
                onClick={() => {
                  setFacultyStatus("loading");
                  setAttempt((value) => value + 1);
                }}
              >
                Спробувати ще раз
              </button>
            )}
          </div>
        )}
        <fieldset className="profile-fields" disabled={pending || expired}>
          <legend className="sr-only">Особисті дані та опис</legend>
          <OnboardingProfile
            embedded
            values={draft}
            university={profile.university.name}
            universityPicker={{
              options: universities,
              value: draft.universityId,
              onChange: changeUniversity,
            }}
            faculties={faculties}
            errors={errors}
            onChange={change}
          />
        </fieldset>
      </section>
      <div className="account-card profile-save-bar">
        <p role="status" aria-live="polite">
          {notice || (dirty ? "Є незбережені зміни" : "Усі зміни збережено")}
        </p>
        {expired && <Link href="/login?next=/profile">Увійти знову</Link>}
        <div className="profile-actions">
          <button
            className="dashboard-button"
            type="button"
            disabled={pending || !dirty}
            onClick={() => {
              setDraft(initial());
              setErrors({});
              setNotice("");
            }}
          >
            Скасувати
          </button>
          <button
            className="dashboard-button dashboard-button--primary"
            type="submit"
            disabled={pending || expired || !dirty}
          >
            {pending ? "Зберігаємо…" : "Зберегти зміни"}
          </button>
        </div>
      </div>
    </form>
  );
}

function toDraft(profile: Profile): ProfileDraft {
  return {
    ...profileFields(profile),
    universityId: profile.university.id,
    formats: profile.formats,
  };
}
