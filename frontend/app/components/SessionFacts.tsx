import type { LessonSession, SessionStatus } from "./SessionApi";
import { meetingLink, sessionTime } from "./SessionTime";
export const sessionStatusLabels: Record<SessionStatus, string> = {
  DRAFT: "Не заплановано",
  PROPOSED: "Очікує погодження",
  SCHEDULED: "Погоджено",
  COMPLETED: "Проведено",
  CANCELLED: "Скасовано",
};

export function SessionFacts({ lesson, timezone }: { lesson: LessonSession; timezone?: string }) {
  const link = meetingLink(lesson.meetingUrl);
  return (
    <div className="session-facts">
      <span className={`session-status session-status--${lesson.status.toLowerCase()}`}>
        {sessionStatusLabels[lesson.status]}
      </span>
      <p>
        {lesson.skill.name} · {lesson.role === "TEACHER" ? "Ти навчаєш" : "Ти вивчаєш"}
      </p>
      {lesson.startsAt && (
        <p>
          <time dateTime={lesson.startsAt}>{sessionTime(lesson.startsAt, timezone)}</time>
          {lesson.endsAt && (
            <>
              {" "}
              — <time dateTime={lesson.endsAt}>{sessionTime(lesson.endsAt, timezone)}</time>
            </>
          )}
        </p>
      )}
      <p className="profile-hint">
        {lesson.format === "ONLINE" ? "Онлайн" : "Офлайн"} ·{" "}
        {lesson.durationMinutes ?? lesson.plannedDurationMinutes} хв.
      </p>
      {lesson.meetingPlace && <p className="profile-biography">{lesson.meetingPlace}</p>}
      {link && (
        <a
          className="dashboard-button dashboard-button--primary"
          href={link}
          target="_blank"
          rel="noopener noreferrer"
        >
          Відкрити зустріч ↗
        </a>
      )}
    </div>
  );
}
