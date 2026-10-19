"use client";

import Link from "next/link";
import { exchangeHref, type Exchange } from "./ExchangeApi";
import { sessionHref, type LessonSession } from "./SessionApi";
import { SessionFacts } from "./SessionFacts";

export function ExchangeLessons({
  exchange,
  lessons,
  onRefresh,
}: {
  exchange: Exchange;
  lessons: LessonSession[];
  onRefresh: () => void;
}) {
  return (
    <section className="account-card" aria-labelledby="exchange-lessons">
      <div className="dashboard-section-heading">
        <h2 id="exchange-lessons">План занять</h2>
        <button className="dashboard-button" onClick={onRefresh}>
          Оновити
        </button>
      </div>
      {lessons.length ? (
        <ol className="session-list session-plan">
          {lessons.map((lesson) => (
            <li key={lesson.id}>
              <span className="session-number" aria-label={`Заняття ${lesson.planOrder}`}>
                {lesson.planOrder}
              </span>
              <div className="session-plan__body">
                <h3>{lesson.title}</h3>
                <SessionFacts lesson={lesson} />
                <Link
                  className="dashboard-button"
                  href={sessionHref(lesson.id, exchangeHref(exchange.id))}
                >
                  {lesson.status === "DRAFT" && exchange.status === "ACTIVE"
                    ? "Узгодити дату"
                    : "Деталі заняття"}{" "}
                  →
                </Link>
              </div>
            </li>
          ))}
        </ol>
      ) : (
        <p className="profile-hint">У цьому обміні ще немає занять.</p>
      )}
    </section>
  );
}
