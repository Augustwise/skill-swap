import Image from "next/image";
import type { ProfileFields, Reference } from "./OnboardingApi";

export function OnboardingProfile({
  values,
  university,
  faculties,
  errors,
  onChange,
}: {
  values: ProfileFields;
  university: string;
  faculties: Reference[];
  errors: Record<string, string>;
  onChange: (field: keyof ProfileFields, value: string) => void;
}) {
  function textField(field: "firstName" | "lastName" | "city", label: string, placeholder: string) {
    return (
      <label className="register-field">
        <span id={`onboarding-${field}-label`}>{label}</span>
        <input
          name={field}
          aria-labelledby={`onboarding-${field}-label`}
          value={values[field]}
          placeholder={placeholder}
          autoComplete={
            field === "city"
              ? "address-level2"
              : field === "firstName"
                ? "given-name"
                : "family-name"
          }
          maxLength={100}
          required={field !== "city"}
          aria-invalid={Boolean(errors[field])}
          aria-describedby={errors[field] ? `onboarding-${field}-error` : undefined}
          onChange={(event) => onChange(field, event.target.value)}
        />
        {errors[field] && (
          <small id={`onboarding-${field}-error`} className="register-field__error">
            {errors[field]}
          </small>
        )}
      </label>
    );
  }

  return (
    <>
      <div className="onboarding-intro">
        <h1 id="onboarding-title" tabIndex={-1}>
          Розкажи про себе
        </h1>
        <p>Заповни профіль, щоб іншим студентам було простіше познайомитися з тобою.</p>
      </div>
      <div className="onboarding-profile">
        <div className="register-form__name-row">
          {textField("firstName", "Ім’я", "Андрій")}
          {textField("lastName", "Прізвище", "Мельник")}
        </div>
        <label className="register-field">
          <span id="onboarding-university-label">Університет</span>
          <input
            value={university}
            readOnly
            aria-labelledby="onboarding-university-label"
            aria-describedby="onboarding-university-hint"
          />
          <small id="onboarding-university-hint" className="onboarding-field-hint">
            Університет, вказаний під час реєстрації.
          </small>
        </label>
        <div className="register-form__name-row">
          <label className="register-field register-field--select">
            <span id="onboarding-faculty-label">
              Факультет <span className="onboarding-optional">· необов’язково</span>
            </span>
            <select
              name="facultyId"
              aria-labelledby="onboarding-faculty-label"
              value={values.facultyId}
              onChange={(event) => onChange("facultyId", event.target.value)}
              aria-invalid={Boolean(errors.facultyId)}
              aria-describedby={errors.facultyId ? "onboarding-facultyId-error" : undefined}
            >
              <option value="">Обери факультет</option>
              {faculties.map((faculty) => (
                <option value={faculty.id} key={faculty.id}>
                  {faculty.name}
                </option>
              ))}
            </select>
            <Image src="/figma/register/chevron-down.svg" width={16} height={16} alt="" />
            {errors.facultyId && (
              <small id="onboarding-facultyId-error" className="register-field__error">
                {errors.facultyId}
              </small>
            )}
          </label>
          <label className="register-field register-field--select">
            <span id="onboarding-course-label">
              Курс <span className="onboarding-optional">· необов’язково</span>
            </span>
            <select
              name="course"
              aria-labelledby="onboarding-course-label"
              value={values.course}
              onChange={(event) => onChange("course", event.target.value)}
              aria-invalid={Boolean(errors.course)}
              aria-describedby={errors.course ? "onboarding-course-error" : undefined}
            >
              <option value="">Обери курс</option>
              {[1, 2, 3, 4, 5, 6].map((course) => (
                <option value={course} key={course}>
                  {course} курс
                </option>
              ))}
            </select>
            <Image src="/figma/register/chevron-down.svg" width={16} height={16} alt="" />
            {errors.course && (
              <small id="onboarding-course-error" className="register-field__error">
                {errors.course}
              </small>
            )}
          </label>
        </div>
        {textField("city", "Місто · необов’язково", "Наприклад, Київ")}
        <label className="register-field">
          <span id="onboarding-bio-label">
            Про мене <span className="onboarding-optional">· необов’язково</span>
          </span>
          <textarea
            name="bio"
            aria-labelledby="onboarding-bio-label"
            value={values.bio}
            maxLength={600}
            rows={4}
            placeholder="Розкажи про свої інтереси та досвід обміну навичками…"
            onChange={(event) => onChange("bio", event.target.value)}
            aria-invalid={Boolean(errors.bio)}
            aria-describedby="onboarding-bio-hint"
          />
          <small
            id="onboarding-bio-hint"
            className={errors.bio ? "register-field__error" : "onboarding-field-hint"}
          >
            {errors.bio || `${values.bio.length} / 600 символів`}
          </small>
        </label>
      </div>
    </>
  );
}
