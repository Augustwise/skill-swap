import { completionMessage } from "./CompletionApi";
import { CompletionConfirmations } from "./CompletionFeedback";
import { formatDateTime } from "./ExchangeFormat";
import type { LessonSession } from "./SessionApi";
import "../styles/completion.css";

export function SessionCompletion({
  lesson,
  pending,
  onConfirm,
}: {
  lesson: LessonSession;
  pending: boolean;
  onConfirm: () => void;
}) {
  if (!["SCHEDULED", "COMPLETED"].includes(lesson.status)) return null;
  return (
    <section className="completion-panel" aria-labelledby="lesson-completion-title">
      <h3 id="lesson-completion-title">Підтвердження проведення</h3>
      <p>{completionMessage(lesson, "lesson")}</p>
      <CompletionConfirmations item={lesson} />
      {lesson.completedAt && (
        <p className="profile-hint">
          Зараховано <time dateTime={lesson.completedAt}>{formatDateTime(lesson.completedAt)}</time>
        </p>
      )}
      {lesson.canConfirmCompletion && (
        <button
          className="dashboard-button dashboard-button--primary"
          disabled={pending}
          onClick={onConfirm}
        >
          {pending ? "Підтверджуємо…" : "Заняття відбулося"}
        </button>
      )}
      <p className="profile-hint">
        Заняття впливає на прогрес лише після підтвердження обох учасників.
      </p>
    </section>
  );
}
