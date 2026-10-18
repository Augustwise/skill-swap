"use client";

import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { safeAppPath, studentHref } from "./AppRoutes";
import { AppShell, type AppSection } from "./AppShell";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DiscoveryEmpty, MutualBadge, SkillChips } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import {
  matchOverview,
  studentProfile,
  type MatchDetails,
  type StudentProfile,
  type UserSkill,
} from "./DiscoveryApi";
import {
  avatarColor,
  countLabel,
  formatDate,
  formatList,
  formatRating,
  fullName,
  initials,
  reviewForms,
  studyDetails,
} from "./DiscoveryFormat";
import { skillLevels } from "./ProfileApi";
import { proposalOptions, requestsHref } from "./ExchangeApi";
import { ExchangeProposal } from "./ExchangeProposal";
import { SkeletonDetail } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

const backLinks: Record<string, { label: string; section?: AppSection }> = {
  "/search": { label: "До результатів пошуку", section: "search" },
  "/matches": { label: "До взаємних збігів", section: "matches" },
  "/main": { label: "На головну", section: "main" },
  "/requests": { label: "До запитів на обмін", section: "requests" },
  "/requests/": { label: "До запиту на обмін", section: "requests" },
  "/exchanges/": { label: "До обміну" },
};

function backLinkFor(path: string) {
  const { pathname } = new URL(path, "http://skillswap.local");
  const entity = /^\/(requests|exchanges)\/[^/]+$/.exec(pathname);
  return backLinks[entity ? `/${entity[1]}/` : pathname] ?? backLinks["/search"];
}

function SkillList({
  title,
  skills,
  highlighted,
  highlight,
  learn = false,
  empty,
}: {
  title: string;
  skills: UserSkill[];
  highlighted: Set<string>;
  highlight: string;
  learn?: boolean;
  empty: string;
}) {
  return (
    <div className="candidate-skills">
      <h3>{title}</h3>
      {skills.length === 0 ? (
        <p className="profile-hint">{empty}</p>
      ) : (
        <ul className="profile-skill-rows">
          {skills.map((skill) => (
            <li
              className={`profile-skill-row ${learn ? "profile-skill-row--learn" : ""}`}
              key={skill.skillId}
            >
              <span className="profile-skill-icon">
                <Image
                  src={`/figma/profile/${learn ? "learn" : "teach"}.svg`}
                  alt=""
                  width={17}
                  height={17}
                />
              </span>
              <strong>{skill.name}</strong>
              <span className="candidate-level">{skillLevels[skill.level]}</span>
              {highlighted.has(skill.skillId) && (
                <span className="candidate-hit">
                  <Icon name="match-sparkles" />
                  {highlight}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function MatchExplanation({
  student,
  overview,
}: {
  student: StudentProfile;
  overview: MatchDetails;
}) {
  if (student.mutual) {
    return (
      <section className="account-card candidate-match" aria-labelledby="candidate-match-title">
        <MutualBadge />
        <h2 id="candidate-match-title">Чому це взаємний збіг</h2>
        <div className="dashboard-match__skills">
          <div>
            <h4>Навчить тебе</h4>
            <SkillChips skills={student.mutual.canTeachYou} label="Навички, яких навчить тебе" />
          </div>
          <div>
            <h4>Хоче вивчити у тебе</h4>
            <SkillChips
              skills={student.mutual.wantsToLearn}
              learn
              label="Навички, які хоче вивчити у тебе"
            />
          </div>
        </div>
        <p className="discovery-meta">
          <span>Спільний формат:</span> {formatList(student.mutual.commonFormats, student.city)}
        </p>
      </section>
    );
  }
  const reasons = [
    overview.canTeachYou.length === 0 &&
      "серед навичок, яких тут навчають, немає жодної з твого списку «Вивчаю»",
    overview.wantsToLearn.length === 0 && "у списку «Хоче вивчити» немає навичок, яких навчаєш ти",
    overview.commonFormats.length === 0 &&
      "немає спільного формату занять (офлайн — лише в одному місті)",
  ].filter((reason): reason is string => Boolean(reason));
  return (
    <section className="account-card" aria-labelledby="candidate-match-title">
      <h2 id="candidate-match-title">Чому це не взаємний збіг</h2>
      {reasons.length > 0 ? (
        <>
          <p className="profile-hint">Порівняно з твоїм поточним профілем:</p>
          <ul className="candidate-reasons">
            {reasons.map((reason) => (
              <li key={reason}>{reason}</li>
            ))}
          </ul>
        </>
      ) : (
        <p className="profile-hint">
          Профіль студента ще не готовий до взаємного підбору. Обмін можна обговорити пізніше.
        </p>
      )}
      {overview.canTeachYou.length > 0 && (
        <div className="dashboard-match__skills">
          <div>
            <h4>Може навчити тебе</h4>
            <SkillChips skills={overview.canTeachYou} label="Навички, яких може навчити тебе" />
          </div>
        </div>
      )}
    </section>
  );
}

function Reviews({ student }: { student: StudentProfile }) {
  return (
    <section className="account-card" aria-labelledby="candidate-reviews-title">
      <div className="dashboard-section-heading">
        <h2 id="candidate-reviews-title">Відгуки</h2>
        {student.averageRating !== null && (
          <p className="dashboard-rating">
            <Icon name="star" />
            <strong>{formatRating(student.averageRating)}</strong>
            <span>· {countLabel(student.reviewCount, reviewForms)}</span>
          </p>
        )}
      </div>
      {student.reviews.length === 0 ? (
        <p className="profile-hint">Ще немає відгуків</p>
      ) : (
        <ul className="candidate-reviews">
          {student.reviews.map((review) => (
            <li key={review.id}>
              <div className="candidate-review__head">
                <strong>{fullName(review.author)}</strong>
                <span>Оцінка {review.rating} з 5</span>
                <time dateTime={review.createdAt}>{formatDate(review.createdAt)}</time>
              </div>
              {review.comment && <p className="profile-biography">{review.comment}</p>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CandidateView({
  profile,
  session,
  studentId,
  from,
}: AppContext & { studentId: string; from: string }) {
  const [modal, setModal] = useState(false);
  const back = safeAppPath(from, "/search");
  const backLink = backLinkFor(back);
  const path = studentHref(studentId, from || undefined);
  const result = useApiQuery(`students/${studentId}`, (signal) =>
    studentProfile(studentId, signal),
  );
  const student = result.state.status === "ready" ? result.state.data : null;
  const overview = student ? (student.mutual ?? matchOverview(profile, student)) : null;
  const options = overview ? proposalOptions(overview) : null;
  const canPropose = Boolean(
    options && options.teach.length && options.learn.length && options.formats.length,
  );

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section={backLink.section}
      title={pageShells.student.title}
      subtitle={pageShells.student.subtitle}
      contentId="student-content"
    >
      <Link className="discovery-back" href={back}>
        ← {backLink.label}
      </Link>
      {result.state.status === "loading" && (
        <>
          <p className="sr-only" role="status">
            Завантажуємо профіль студента…
          </p>
          <SkeletonDetail />
        </>
      )}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={path}
          onRetry={result.retry}
          notFound={
            <DiscoveryEmpty icon="nav-search" title="Профіль недоступний.">
              <p>Профіль не знайдено або його приховано від пошуку.</p>
              <Link className="dashboard-button dashboard-button--primary" href={back}>
                {backLink.label}
              </Link>
            </DiscoveryEmpty>
          }
        />
      )}
      {student && overview && (
        <div className="profile-columns">
          <div className="profile-sections">
            <section className="account-card" aria-labelledby="candidate-name">
              <div className="profile-identity candidate-identity">
                <DashboardAvatar
                  initials={initials(student)}
                  color={avatarColor(student.id)}
                  size={48}
                />
                <div>
                  <h2 id="candidate-name">{fullName(student)}</h2>
                  <p>{studyDetails(student)}</p>
                  {student.city && <p>{student.city}</p>}
                </div>
                {student.mutual && <MutualBadge />}
              </div>
              <p className="dashboard-rating">
                {student.averageRating === null ? (
                  "Ще немає відгуків"
                ) : (
                  <>
                    <Icon name="star" />
                    <strong>{formatRating(student.averageRating)}</strong>
                    <span>· {countLabel(student.reviewCount, reviewForms)}</span>
                  </>
                )}
              </p>
            </section>
            <section className="account-card" aria-labelledby="candidate-bio-title">
              <h2 id="candidate-bio-title">Про себе</h2>
              <p className={student.bio ? "profile-biography" : "profile-hint"}>
                {student.bio || "Опису поки немає."}
              </p>
            </section>
            <section className="account-card" aria-labelledby="candidate-skills-title">
              <h2 id="candidate-skills-title">Навички</h2>
              <SkillList
                title="Навчає"
                skills={student.teachingSkills}
                highlighted={new Set(overview.canTeachYou.map((skill) => skill.skillId))}
                highlight="Ти хочеш вивчити"
                empty="Навичок для викладання поки немає."
              />
              <SkillList
                title="Хоче вивчити"
                skills={student.learningSkills}
                highlighted={new Set(overview.wantsToLearn.map((skill) => skill.skillId))}
                highlight="Ти навчаєш"
                learn
                empty="Навичок для вивчення поки немає."
              />
            </section>
            <Reviews student={student} />
          </div>
          <aside className="profile-rail" aria-label="Сумісність і обмін">
            <MatchExplanation student={student} overview={overview} />
            <section className="account-card" aria-labelledby="candidate-formats-title">
              <h2 id="candidate-formats-title">Формат занять</h2>
              <p>{formatList(student.formats, student.city) || "Формат ще не вказано."}</p>
            </section>
            <section className="account-card profile-tip" aria-labelledby="candidate-propose-title">
              <h2 id="candidate-propose-title">Хочеш обмінятися?</h2>
              {canPropose ? (
                <>
                  <p>
                    Обери, чого навчиш і чого навчишся, формат і план занять. {student.firstName}{" "}
                    отримає запит і зможе прийняти або відхилити його.
                  </p>
                  <button
                    className="dashboard-button dashboard-button--primary"
                    type="button"
                    aria-haspopup="dialog"
                    onClick={() => setModal(true)}
                  >
                    Запропонувати обмін
                  </button>
                </>
              ) : (
                <>
                  <p>
                    Обмін можливий, коли кожен із вас навчає навички, яку хоче вивчити інший, і у
                    вас є спільний формат занять. Онови списки навичок, щоб запропонувати обмін.
                  </p>
                  <Link className="dashboard-button" href="/profile">
                    Перейти до мого профілю
                  </Link>
                </>
              )}
              <Link className="profile-tip__link" href={requestsHref({ direction: "outgoing" })}>
                Мої надіслані запити
              </Link>
            </section>
          </aside>
        </div>
      )}
      {modal && student && options && (
        <ExchangeProposal
          profile={profile}
          student={student}
          options={options}
          csrfToken={session.csrfToken}
          onClose={() => setModal(false)}
        />
      )}
    </AppShell>
  );
}

export function CandidateProfile({ studentId, from }: { studentId: string; from: string }) {
  return (
    <AppGate
      page="student"
      section={backLinkFor(safeAppPath(from, "/search")).section}
      path={studentHref(studentId, from || undefined)}
    >
      {(context) => <CandidateView {...context} studentId={studentId} from={from} />}
    </AppGate>
  );
}
