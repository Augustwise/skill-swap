import Link from "next/link";
import type { ReactNode } from "react";
import { studentHref } from "./AppRoutes";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import type { MutualMatch, StudentCard, StudentSummary, UserSkill } from "./DiscoveryApi";
import {
  avatarColor,
  countLabel,
  formatList,
  fullName,
  initials,
  skillForms,
} from "./DiscoveryFormat";
import { skillLevels } from "./ProfileApi";

export function SkillChips({
  skills,
  learn = false,
  label,
}: {
  skills: readonly UserSkill[];
  learn?: boolean;
  label: string;
}) {
  return (
    <ul className="dashboard-tags discovery-tags" aria-label={label}>
      {skills.map((skill) => (
        <li
          className={`dashboard-skill ${learn ? "dashboard-skill--learn" : ""}`}
          key={skill.skillId}
        >
          <Icon name={learn ? "learn-dot" : "teach-dot"} />
          <span>
            {skill.name}
            <span className="discovery-level"> · {skillLevels[skill.level]}</span>
          </span>
        </li>
      ))}
    </ul>
  );
}

export function MutualBadge({ children = "Взаємний збіг" }: { children?: string }) {
  return (
    <span className="dashboard-score">
      <Icon name="match-sparkles" />
      {children}
    </span>
  );
}

export function DiscoveryEmpty({
  icon,
  title,
  children,
}: {
  icon: DashboardIconName;
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className="dashboard-empty discovery-empty">
      <span className="dashboard-empty__icon" aria-hidden="true">
        <Icon name={icon} />
      </span>
      <p className="discovery-empty__title">{title}</p>
      {children}
    </div>
  );
}

function StudentHeading({ student, badge }: { student: StudentSummary; badge?: ReactNode }) {
  const place = [student.university.name, student.city].filter(Boolean).join(" · ");
  return (
    <div className="dashboard-match__person">
      <DashboardAvatar initials={initials(student)} color={avatarColor(student.id)} size={48} />
      <div className="dashboard-match__identity">
        <h3>{fullName(student)}</h3>
        <p>{place}</p>
      </div>
      {badge}
    </div>
  );
}

function ProfileLink({ student, from }: { student: StudentSummary; from: string }) {
  return (
    <div className="dashboard-match__actions">
      <Link
        className="dashboard-button dashboard-button--primary"
        href={studentHref(student.id, from)}
        aria-label={`Переглянути профіль: ${fullName(student)}`}
      >
        Переглянути профіль
        <Icon name="arrow-right" />
      </Link>
    </div>
  );
}

export function StudentResultCard({ card, from }: { card: StudentCard; from: string }) {
  return (
    <li className="dashboard-match discovery-card">
      <StudentHeading student={card.student} badge={card.mutual && <MutualBadge />} />
      <div className="dashboard-match__skills">
        <div>
          <h4>Навчає</h4>
          <SkillChips skills={card.teachingSkills} label="Навички, яких навчає студент" />
        </div>
      </div>
      <p className="discovery-meta">
        <span>Формат занять:</span> {formatList(card.formats, card.student.city)}
      </p>
      <ProfileLink student={card.student} from={from} />
    </li>
  );
}

export function MatchCard({ match, from }: { match: MutualMatch; from: string }) {
  const total = match.canTeachYou.length + match.wantsToLearn.length;
  return (
    <li className="dashboard-match discovery-card">
      <StudentHeading student={match.student} badge={<MutualBadge />} />
      <p className="discovery-meta">Збіг в обох напрямках · {countLabel(total, skillForms)}</p>
      <div className="dashboard-match__skills">
        <div>
          <h4>Навчить тебе</h4>
          <SkillChips skills={match.canTeachYou} label="Навички, яких студент може навчити тебе" />
        </div>
        <div>
          <h4>Хоче вивчити у тебе</h4>
          <SkillChips
            skills={match.wantsToLearn}
            learn
            label="Навички, які студент хоче вивчити у тебе"
          />
        </div>
      </div>
      <p className="discovery-meta">
        <span>Спільний формат:</span> {formatList(match.commonFormats, match.student.city)}
      </p>
      <ProfileLink student={match.student} from={from} />
    </li>
  );
}

export function Pagination({
  page,
  pages,
  hrefFor,
}: {
  page: number;
  pages: number;
  hrefFor: (page: number) => string;
}) {
  if (pages <= 1) return null;
  return (
    <nav className="discovery-pagination" aria-label="Сторінки результатів">
      {page > 1 ? (
        <Link className="dashboard-button" href={hrefFor(page - 1)} rel="prev">
          ← Попередня
        </Link>
      ) : (
        <span className="dashboard-button" aria-disabled="true">
          ← Попередня
        </span>
      )}
      <span className="discovery-pagination__current" aria-current="page">
        Сторінка {page} з {pages}
      </span>
      {page < pages ? (
        <Link className="dashboard-button" href={hrefFor(page + 1)} rel="next">
          Наступна →
        </Link>
      ) : (
        <span className="dashboard-button" aria-disabled="true">
          Наступна →
        </span>
      )}
    </nav>
  );
}
