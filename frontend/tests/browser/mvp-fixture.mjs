export const BASE_URL = "http://127.0.0.1:3100";
export const MAILPIT_URL = "http://127.0.0.1:8025";
export const DEMO_PASSWORD = "SkillSwapDemo2026";

export const IDS = {
  universities: {
    kpi: "10000000-0000-4000-8000-000000000001",
    knu: "10000000-0000-4000-8000-000000000002",
  },
  faculties: {
    fiot: "11000000-0000-4000-8000-000000000001",
    ipsa: "11000000-0000-4000-8000-000000000002",
    fcsc: "11000000-0000-4000-8000-000000000003",
  },
  categories: {
    tech: "20000000-0000-4000-8000-000000000001",
    design: "20000000-0000-4000-8000-000000000002",
    music: "20000000-0000-4000-8000-000000000003",
    lang: "20000000-0000-4000-8000-000000000004",
  },
  skills: {
    guitar: "30000000-0000-4000-8000-000000000001",
    photoshop: "30000000-0000-4000-8000-000000000002",
    english: "30000000-0000-4000-8000-000000000003",
    go: "30000000-0000-4000-8000-000000000004",
    figma: "30000000-0000-4000-8000-000000000005",
    python: "30000000-0000-4000-8000-000000000006",
  },
};

function makeUuid(prefix, seq) {
  return `${prefix}-0000-4000-8000-${String(seq).padStart(12, "0")}`;
}

function makeToken(prefix, seq) {
  return `${prefix}_${String(seq).padStart(39, "0")}`.slice(0, 43);
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function parseSessionCookie(cookieHeader) {
  if (!cookieHeader) return "";
  for (const part of cookieHeader.split(";")) {
    const [rawName, ...rest] = part.trim().split("=");
    if (rawName === "skillswap_session") return rest.join("=").trim();
  }
  return "";
}

export function kyivLocalInput(isoOrMs) {
  const date = typeof isoOrMs === "number" ? new Date(isoOrMs) : new Date(isoOrMs);
  const formatted = new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Europe/Kyiv",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
  return formatted.replace(" ", "T");
}

export function createMvpWorld(options = {}) {
  let seq = 1;
  let clockOffsetMs = 0;
  const nextSeq = () => seq++;
  const nowMs = () => Date.now() + clockOffsetMs;
  const nowIso = () => new Date(nowMs()).toISOString();

  const universities = [
    {
      id: IDS.universities.kpi,
      name: "КПІ ім. Ігоря Сікорського",
      emailDomain: "students.example.test",
    },
    {
      id: IDS.universities.knu,
      name: "КНУ ім. Тараса Шевченка",
      emailDomain: "knu.example.test",
    },
  ];

  const facultiesByUniversity = new Map([
    [
      IDS.universities.kpi,
      [
        { id: IDS.faculties.fiot, name: "ФІОТ" },
        { id: IDS.faculties.ipsa, name: "ІПСА" },
      ],
    ],
    [IDS.universities.knu, [{ id: IDS.faculties.fcsc, name: "ФКНК" }]],
  ]);

  const categories = [
    { id: IDS.categories.tech, name: "Технології", slug: "technology" },
    { id: IDS.categories.design, name: "Дизайн", slug: "design" },
    { id: IDS.categories.music, name: "Музика", slug: "music" },
    { id: IDS.categories.lang, name: "Мови", slug: "languages" },
  ];

  const skills = [
    {
      id: IDS.skills.guitar,
      categoryId: IDS.categories.music,
      categoryName: "Музика",
      name: "Гітара",
      slug: "guitar",
      description: "Акустична та електрогітара",
      isActive: true,
    },
    {
      id: IDS.skills.photoshop,
      categoryId: IDS.categories.design,
      categoryName: "Дизайн",
      name: "Photoshop",
      slug: "photoshop",
      description: "Ретуш і графіка",
      isActive: true,
    },
    {
      id: IDS.skills.english,
      categoryId: IDS.categories.lang,
      categoryName: "Мови",
      name: "Англійська",
      slug: "english",
      description: "Розмовна практика",
      isActive: true,
    },
    {
      id: IDS.skills.go,
      categoryId: IDS.categories.tech,
      categoryName: "Технології",
      name: "Go",
      slug: "golang",
      description: "Бекенд розробка на Go",
      isActive: true,
    },
    {
      id: IDS.skills.figma,
      categoryId: IDS.categories.design,
      categoryName: "Дизайн",
      name: "Figma",
      slug: "figma",
      description: "Проектування інтерфейсів",
      isActive: true,
    },
    {
      id: IDS.skills.python,
      categoryId: IDS.categories.tech,
      categoryName: "Технології",
      name: "Python",
      slug: "python",
      description: "Аналіз даних і скрипти",
      isActive: true,
    },
  ];

  const users = new Map();
  const sessions = new Map();
  const verifyTokens = new Map();
  const resetTokens = new Map();
  const failedLogins = new Map();
  const mailbox = [];
  const requests = new Map();
  const exchanges = new Map();
  const lessons = new Map();
  const conversations = new Map();
  const messages = new Map();
  const notifications = new Map();
  const notificationKeys = new Set();
  const reviews = new Map();
  const blocks = [];
  const reports = new Map();
  const moderationActions = [];
  const faultRules = [];
  const requestLog = [];

  const config = {
    mailAvailable: true,
    pollIntervalSeconds: options.pollIntervalSeconds ?? 5,
    enforceCsrf: options.enforceCsrf ?? true,
  };

  function findSkill(skillId) {
    return skills.find((item) => item.id === skillId) ?? null;
  }

  function findUniversity(universityId) {
    return universities.find((item) => item.id === universityId) ?? universities[0];
  }

  function findFaculty(universityId, facultyId) {
    if (!facultyId) return null;
    const list = facultiesByUniversity.get(universityId) ?? [];
    return list.find((item) => item.id === facultyId) ?? null;
  }

  function personView(user) {
    return {
      id: user.id,
      firstName: user.firstName,
      lastName: user.lastName,
    };
  }

  function statusHistoryView(history) {
    return history.map((change) => {
      const author = change.changedBy ? users.get(change.changedBy.id) : null;
      return { ...change, changedBy: author ? personView(author) : change.changedBy };
    });
  }

  function summaryView(user) {
    const uni = findUniversity(user.universityId);
    return {
      id: user.id,
      firstName: user.firstName,
      lastName: user.lastName,
      university: { id: uni.id, name: uni.name },
      city: user.city,
    };
  }

  function isEligible(user) {
    return (
      !user.deleted &&
      user.accountStatus === "ACTIVE" &&
      user.teachingSkills.length > 0 &&
      user.learningSkills.length > 0 &&
      user.formats.length > 0 &&
      (!user.formats.includes("OFFLINE") || user.city.trim() !== "")
    );
  }

  function profileView(user) {
    const uni = findUniversity(user.universityId);
    const fac = findFaculty(user.universityId, user.facultyId);
    return {
      id: user.id,
      firstName: user.firstName,
      lastName: user.lastName,
      university: { id: uni.id, name: uni.name },
      faculty: fac ? { id: fac.id, name: fac.name } : null,
      course: user.course,
      city: user.city,
      bio: user.bio,
      formats: [...user.formats],
      teachingSkills: user.teachingSkills.map((item) => ({ ...item })),
      learningSkills: user.learningSkills.map((item) => ({ ...item })),
      eligibleForMatching: isEligible(user),
    };
  }

  function hasBlockBetween(userAId, userBId) {
    return blocks.some(
      (entry) =>
        (entry.blockerId === userAId && entry.blockedId === userBId) ||
        (entry.blockerId === userBId && entry.blockedId === userAId),
    );
  }

  function isBlockedBy(blockerId, blockedId) {
    return blocks.some((entry) => entry.blockerId === blockerId && entry.blockedId === blockedId);
  }

  function commonFormatsFor(viewer, candidate) {
    const cityNorm = (val) => val.trim().toLocaleLowerCase("uk");
    const sameCity =
      cityNorm(viewer.city) !== "" && cityNorm(viewer.city) === cityNorm(candidate.city);
    return candidate.formats.filter(
      (fmt) => viewer.formats.includes(fmt) && (fmt !== "OFFLINE" || sameCity),
    );
  }

  function computeMutual(viewer, candidate) {
    if (!isEligible(viewer) || !isEligible(candidate)) return null;
    const wanted = new Set(viewer.learningSkills.map((s) => s.skillId));
    const offered = new Set(viewer.teachingSkills.map((s) => s.skillId));
    const canTeachYou = candidate.teachingSkills
      .filter((s) => wanted.has(s.skillId))
      .map((s) => ({ ...s }));
    const wantsToLearn = candidate.learningSkills
      .filter((s) => offered.has(s.skillId))
      .map((s) => ({ ...s }));
    const commonFormats = commonFormatsFor(viewer, candidate);
    if (!canTeachYou.length || !wantsToLearn.length || !commonFormats.length) return null;
    return { canTeachYou, wantsToLearn, commonFormats };
  }

  function userReviewsList(targetUserId) {
    return [...reviews.values()]
      .filter((rev) => rev.targetUserId === targetUserId)
      .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
      .map((rev) => ({
        id: rev.id,
        exchangeId: rev.exchangeId,
        author: personView(users.get(rev.authorId)),
        rating: rev.rating,
        comment: rev.comment,
        createdAt: rev.createdAt,
      }));
  }

  function ratingSummary(targetUserId) {
    const list = userReviewsList(targetUserId);
    if (!list.length) return { averageRating: null, reviewCount: 0, list };
    const sum = list.reduce((acc, item) => acc + item.rating, 0);
    return {
      averageRating: Math.round((sum / list.length) * 10) / 10,
      reviewCount: list.length,
      list,
    };
  }

  function addNotification({
    recipientId,
    actorId,
    eventKey,
    eventType,
    title,
    body = null,
    target,
    requestId = null,
    exchangeId = null,
    conversationId = null,
    messageId = null,
    sessionId = null,
    reviewId = null,
  }) {
    if (eventKey && notificationKeys.has(eventKey)) return null;
    if (eventKey) notificationKeys.add(eventKey);
    const id = makeUuid("47000000", nextSeq());
    const actor = actorId ? users.get(actorId) : null;
    const item = {
      id,
      recipientId,
      eventType,
      title,
      body,
      actor: actor ? personView(actor) : null,
      target,
      requestId,
      exchangeId,
      conversationId,
      messageId,
      sessionId,
      reviewId,
      isRead: false,
      readAt: null,
      createdAt: nowIso(),
    };
    notifications.set(id, item);
    return item;
  }

  function ensureConversationForRequest(req) {
    for (const conv of conversations.values()) {
      if (conv.requestId === req.id) return conv;
    }
    const id = makeUuid("45000000", nextSeq());
    const conv = {
      id,
      type: "REQUEST",
      requestId: req.id,
      exchangeId: req.exchangeId,
      format: req.format,
      requesterId: req.requesterId,
      recipientId: req.recipientId,
      requesterTeaches: structuredClone(req.requesterTeaches),
      recipientTeaches: structuredClone(req.recipientTeaches),
      createdAt: req.createdAt,
    };
    conversations.set(id, conv);
    messages.set(id, []);
    return conv;
  }

  function conversationForExchange(ex) {
    for (const conv of conversations.values()) {
      if (conv.exchangeId === ex.id || (ex.requestId && conv.requestId === ex.requestId)) {
        conv.type = "EXCHANGE";
        conv.exchangeId = ex.id;
        return conv;
      }
    }
    const id = makeUuid("45000000", nextSeq());
    const conv = {
      id,
      type: "EXCHANGE",
      requestId: ex.requestId,
      exchangeId: ex.id,
      format: ex.format,
      requesterId: ex.requesterId,
      recipientId: ex.recipientId,
      requesterTeaches: structuredClone(ex.requesterTeaches),
      recipientTeaches: structuredClone(ex.recipientTeaches),
      createdAt: ex.createdAt,
    };
    conversations.set(id, conv);
    messages.set(id, []);
    return conv;
  }

  function messageView(msg) {
    const author = msg.authorId ? users.get(msg.authorId) : null;
    return {
      id: msg.id,
      conversationId: msg.conversationId,
      author: author ? personView(author) : null,
      type: msg.type,
      body: msg.body,
      createdAt: msg.createdAt,
      editedAt: msg.editedAt,
      readAt: msg.readAt,
    };
  }

  function conversationView(conv, viewerId) {
    const requester = users.get(conv.requesterId);
    const recipient = users.get(conv.recipientId);
    const partner = conv.requesterId === viewerId ? recipient : requester;
    const req = conv.requestId ? requests.get(conv.requestId) : null;
    const ex = conv.exchangeId ? exchanges.get(conv.exchangeId) : null;
    const list = messages.get(conv.id) ?? [];
    const last = list.at(-1) ?? null;
    const unreadCount = list.filter(
      (m) => m.authorId && m.authorId !== viewerId && !m.readAt,
    ).length;
    const blocked = hasBlockBetween(conv.requesterId, conv.recipientId);
    const unavailable =
      requester.deleted ||
      recipient.deleted ||
      requester.accountStatus !== "ACTIVE" ||
      recipient.accountStatus !== "ACTIVE";
    return {
      id: conv.id,
      type: conv.exchangeId ? "EXCHANGE" : "REQUEST",
      requestId: conv.requestId,
      exchangeId: conv.exchangeId,
      requestStatus: req ? req.status : null,
      exchangeStatus: ex ? ex.status : null,
      format: conv.format,
      partner: summaryView(partner),
      requester: summaryView(requester),
      recipient: summaryView(recipient),
      requesterTeaches: structuredClone(conv.requesterTeaches),
      recipientTeaches: structuredClone(conv.recipientTeaches),
      lastMessage: last
        ? {
            id: last.id,
            author: last.authorId ? personView(users.get(last.authorId)) : null,
            body: last.body,
            createdAt: last.createdAt,
          }
        : null,
      lastMessageAt: last ? last.createdAt : null,
      unreadCount,
      canSend: !blocked && !unavailable,
      blockedByMe: isBlockedBy(viewerId, partner.id),
      createdAt: conv.createdAt,
    };
  }

  function requestView(req) {
    return {
      id: req.id,
      status: req.status,
      format: req.format,
      totalSessions: req.totalSessions,
      message: req.message,
      requester: summaryView(users.get(req.requesterId)),
      recipient: summaryView(users.get(req.recipientId)),
      requesterTeaches: structuredClone(req.requesterTeaches),
      recipientTeaches: structuredClone(req.recipientTeaches),
      exchangeId: req.exchangeId,
      createdAt: req.createdAt,
      respondedAt: req.respondedAt,
    };
  }

  function exchangeLessonsList(exchangeId) {
    return [...lessons.values()]
      .filter((item) => item.exchangeId === exchangeId)
      .sort((a, b) => a.planOrder - b.planOrder);
  }

  function computeExchangeProgress(ex) {
    const items = exchangeLessonsList(ex.id);
    const completed = items.filter((s) => s.status === "COMPLETED");
    const requesterCompleted = completed.filter((s) => s.teacherId === ex.requesterId).length;
    const recipientCompleted = completed.filter((s) => s.teacherId === ex.recipientId).length;
    const requesterPlanned = ex.requesterTeaches.sessions;
    const recipientPlanned = ex.recipientTeaches.sessions;
    const scheduled = items.filter((s) => s.status === "SCHEDULED");
    const awaiting = scheduled.filter(
      (s) => (s.endsAt && Date.parse(s.endsAt) <= nowMs()) || s.completionConfirmedBy.size > 0,
    ).length;
    const completedCount = completed.length;
    const planCompleted =
      requesterCompleted >= requesterPlanned && recipientCompleted >= recipientPlanned;
    return {
      completedSessions: completedCount,
      totalSessions: ex.totalSessions,
      requesterCompletedSessions: requesterCompleted,
      requesterPlannedSessions: requesterPlanned,
      recipientCompletedSessions: recipientCompleted,
      recipientPlannedSessions: recipientPlanned,
      scheduledSessions: scheduled.length,
      proposedSessions: items.filter((s) => s.status === "PROPOSED").length,
      draftSessions: items.filter((s) => s.status === "DRAFT").length,
      cancelledSessions: items.filter((s) => s.status === "CANCELLED").length,
      percent: Math.min(100, Math.round((completedCount / Math.max(ex.totalSessions, 1)) * 100)),
      awaitingConfirmationSessions: awaiting,
      planCompleted,
    };
  }

  function exchangeView(ex, viewerId) {
    const requester = users.get(ex.requesterId);
    const recipient = users.get(ex.recipientId);
    const partner = ex.requesterId === viewerId ? recipient : requester;
    const progress = computeExchangeProgress(ex);
    const upcomingScheduled = exchangeLessonsList(ex.id)
      .filter((s) => s.status === "SCHEDULED" && s.startsAt && Date.parse(s.endsAt) > nowMs())
      .sort((a, b) => Date.parse(a.startsAt) - Date.parse(b.startsAt));
    const next = upcomingScheduled[0] ?? null;
    const blockedOrDeleted =
      hasBlockBetween(ex.requesterId, ex.recipientId) ||
      requester.deleted ||
      recipient.deleted ||
      requester.accountStatus !== "ACTIVE" ||
      recipient.accountStatus !== "ACTIVE";
    const confirmedByMe = ex.completionConfirmedBy.has(viewerId);
    const confirmedByPartner = ex.completionConfirmedBy.has(partner.id);
    const myRev = [...reviews.values()].find(
      (r) => r.exchangeId === ex.id && r.authorId === viewerId,
    );
    const partnerRev = [...reviews.values()].find(
      (r) => r.exchangeId === ex.id && r.authorId === partner.id,
    );
    return {
      id: ex.id,
      requestId: ex.requestId,
      status: ex.status,
      format: ex.format,
      totalSessions: ex.totalSessions,
      partner: summaryView(partner),
      requester: summaryView(requester),
      recipient: summaryView(recipient),
      requesterTeaches: structuredClone(ex.requesterTeaches),
      recipientTeaches: structuredClone(ex.recipientTeaches),
      progress,
      nextSession: next
        ? {
            id: next.id,
            title: next.title,
            startsAt: next.startsAt,
            endsAt: next.endsAt,
            meetingPlace: next.meetingPlace,
            meetingUrl: next.meetingUrl,
          }
        : null,
      completionConfirmedByMe: confirmedByMe,
      completionConfirmedByPartner: confirmedByPartner,
      canConfirmCompletion:
        ex.status === "ACTIVE" && progress.planCompleted && !confirmedByMe && !blockedOrDeleted,
      canCancel: ex.status === "ACTIVE",
      reviewedByMe: Boolean(myRev),
      reviewedByPartner: Boolean(partnerRev),
      canReview: ex.status === "COMPLETED" && !myRev && !partner.deleted,
      startedAt: ex.startedAt,
      completedAt: ex.completedAt,
      cancelledAt: ex.cancelledAt,
      cancellationReason: ex.cancellationReason,
      createdAt: ex.createdAt,
    };
  }

  function proposalView(prop) {
    return {
      id: prop.id,
      sessionId: prop.sessionId,
      status: prop.status,
      proposedBy: personView(users.get(prop.proposedById)),
      startsAt: prop.startsAt,
      endsAt: prop.endsAt,
      format: prop.format,
      meetingPlace: prop.meetingPlace,
      meetingUrl: prop.meetingUrl,
      respondedBy: prop.respondedById ? personView(users.get(prop.respondedById)) : null,
      respondedAt: prop.respondedAt,
      createdAt: prop.createdAt,
    };
  }

  function lessonView(lesson, viewerId) {
    const ex = exchanges.get(lesson.exchangeId);
    const teacher = users.get(lesson.teacherId);
    const learner = users.get(lesson.learnerId);
    const partner =
      ex.requesterId === viewerId ? users.get(ex.recipientId) : users.get(ex.requesterId);
    const activeProp = lesson.proposals.find((p) => p.id === lesson.activeProposalId) ?? null;
    const finished = Boolean(lesson.endsAt && Date.parse(lesson.endsAt) <= nowMs());
    const blockedOrInactive =
      ex.status !== "ACTIVE" ||
      hasBlockBetween(ex.requesterId, ex.recipientId) ||
      teacher.deleted ||
      learner.deleted ||
      teacher.accountStatus !== "ACTIVE" ||
      learner.accountStatus !== "ACTIVE";
    const confirmedByMe = lesson.completionConfirmedBy.has(viewerId);
    const confirmedByPartner = lesson.completionConfirmedBy.has(partner.id);
    return {
      id: lesson.id,
      exchangeId: lesson.exchangeId,
      planOrder: lesson.planOrder,
      title: lesson.title,
      description: lesson.description,
      status: lesson.status,
      format: lesson.format,
      skill: { ...lesson.skill },
      plannedDurationMinutes: lesson.plannedDurationMinutes,
      teacher: summaryView(teacher),
      learner: summaryView(learner),
      partner: summaryView(partner),
      role: lesson.teacherId === viewerId ? "TEACHER" : "LEARNER",
      startsAt: lesson.startsAt,
      endsAt: lesson.endsAt,
      durationMinutes: lesson.durationMinutes,
      meetingPlace: lesson.meetingPlace,
      meetingUrl: lesson.meetingUrl,
      activeProposal: activeProp ? proposalView(activeProp) : null,
      scheduleConfirmedByMe: lesson.scheduleConfirmedBy.has(viewerId),
      scheduleConfirmedByPartner: lesson.scheduleConfirmedBy.has(partner.id),
      canRespondToProposal:
        !blockedOrInactive &&
        lesson.status === "PROPOSED" &&
        Boolean(activeProp && activeProp.proposedById !== viewerId),
      canCancel: ex.status === "ACTIVE" && ["PROPOSED", "SCHEDULED"].includes(lesson.status),
      completionConfirmedByMe: confirmedByMe,
      completionConfirmedByPartner: confirmedByPartner,
      canConfirmCompletion:
        !blockedOrInactive && lesson.status === "SCHEDULED" && finished && !confirmedByMe,
      completedAt: lesson.completedAt,
      cancelledAt: lesson.cancelledAt,
      cancellationReason: lesson.cancellationReason,
    };
  }

  function hasScheduleConflict(userIds, startsAt, endsAt, ignoreLessonId) {
    const startMs = Date.parse(startsAt);
    const endMs = Date.parse(endsAt);
    for (const item of lessons.values()) {
      if (
        item.id === ignoreLessonId ||
        item.status !== "SCHEDULED" ||
        !item.startsAt ||
        !item.endsAt
      )
        continue;
      const ex = exchanges.get(item.exchangeId);
      const participants = [ex.requesterId, ex.recipientId];
      if (!userIds.some((uid) => participants.includes(uid))) continue;
      if (Date.parse(item.startsAt) < endMs && startMs < Date.parse(item.endsAt)) {
        return true;
      }
    }
    return false;
  }

  function cancelRelationshipsForUser(userId, reasonText) {
    let closedRequests = 0;
    let cancelledExchanges = 0;
    for (const req of requests.values()) {
      if (req.status === "PENDING" && (req.requesterId === userId || req.recipientId === userId)) {
        req.status = "WITHDRAWN";
        req.respondedAt = nowIso();
        req.history.push({ status: "WITHDRAWN", changedBy: null, createdAt: nowIso() });
        closedRequests++;
      }
    }
    for (const ex of exchanges.values()) {
      if (ex.status === "ACTIVE" && (ex.requesterId === userId || ex.recipientId === userId)) {
        ex.status = "CANCELLED";
        ex.cancelledAt = nowIso();
        ex.cancellationReason = reasonText;
        ex.history.push({
          status: "CANCELLED",
          changedBy: null,
          reason: reasonText,
          createdAt: nowIso(),
        });
        for (const les of exchangeLessonsList(ex.id)) {
          if (les.status !== "COMPLETED") {
            les.status = "CANCELLED";
            les.cancelledAt = nowIso();
            les.cancellationReason = reasonText;
          }
        }
        cancelledExchanges++;
      }
    }
    return { closedRequests, cancelledExchanges };
  }

  function cancelRelationshipsBetween(userAId, userBId, reasonText) {
    let closedRequests = 0;
    let cancelledExchanges = 0;
    const isPair = (a, b) => (a === userAId && b === userBId) || (a === userBId && b === userAId);
    for (const req of requests.values()) {
      if (req.status === "PENDING" && isPair(req.requesterId, req.recipientId)) {
        req.status = "WITHDRAWN";
        req.respondedAt = nowIso();
        req.history.push({ status: "WITHDRAWN", changedBy: null, createdAt: nowIso() });
        closedRequests++;
      }
    }
    for (const ex of exchanges.values()) {
      if (ex.status === "ACTIVE" && isPair(ex.requesterId, ex.recipientId)) {
        ex.status = "CANCELLED";
        ex.cancelledAt = nowIso();
        ex.cancellationReason = reasonText;
        ex.history.push({
          status: "CANCELLED",
          changedBy: null,
          reason: reasonText,
          createdAt: nowIso(),
        });
        for (const les of exchangeLessonsList(ex.id)) {
          if (les.status !== "COMPLETED") {
            les.status = "CANCELLED";
            les.cancelledAt = nowIso();
            les.cancellationReason = reasonText;
          }
        }
        cancelledExchanges++;
      }
    }
    return { closedRequests, cancelledExchanges };
  }

  function revokeUserSessions(userId, exceptSessionId = "") {
    for (const [sid, session] of sessions.entries()) {
      if (session.userId === userId && sid !== exceptSessionId) {
        sessions.delete(sid);
      }
    }
  }

  function issueMailToken(kind, user) {
    const pool = kind === "verify" ? verifyTokens : resetTokens;
    for (const existing of pool.values()) {
      if (existing.userId === user.id) existing.used = true;
    }
    const token = makeToken(kind === "verify" ? "vrf" : "rst", nextSeq());
    pool.set(token, { token, userId: user.id, used: false, createdAt: nowIso() });
    const url =
      kind === "verify"
        ? `${BASE_URL}/verify-email?token=${token}`
        : `${BASE_URL}/reset-password?token=${token}`;
    const subject =
      kind === "verify" ? "Підтвердження пошти SkillSwap" : "Відновлення пароля SkillSwap";
    const mail = {
      id: makeUuid("49000000", nextSeq()),
      to: user.email,
      kind,
      subject,
      token,
      url,
      createdAt: nowIso(),
    };
    mailbox.unshift(mail);
    return mail;
  }

  function mapUserSkills(entries = []) {
    return entries.map((entry) => {
      const catSkill = findSkill(entry.skillId);
      return {
        skillId: entry.skillId,
        categoryId: entry.categoryId ?? catSkill?.categoryId ?? IDS.categories.tech,
        name: entry.name ?? catSkill?.name ?? "Навичка",
        level: entry.level ?? "INTERMEDIATE",
      };
    });
  }

  function seedUser(input) {
    const id = input.id ?? makeUuid("40000000", nextSeq());
    const email = input.email.trim().toLowerCase();
    const domain = email.split("@")[1] ?? "students.example.test";
    const matchedUni =
      universities.find((u) => u.emailDomain.toLowerCase() === domain) ?? universities[0];
    const user = {
      id,
      email,
      password: input.password ?? DEMO_PASSWORD,
      firstName: input.firstName ?? "Студент",
      lastName: input.lastName ?? "Тестовий",
      role: input.role ?? "STUDENT",
      emailVerified: input.emailVerified ?? true,
      accountStatus: input.accountStatus ?? "ACTIVE",
      deleted: false,
      universityId: input.universityId ?? matchedUni.id,
      facultyId: input.facultyId ?? null,
      course: input.course ?? null,
      city: input.city ?? "",
      bio: input.bio ?? "",
      formats: input.formats ? [...input.formats] : [],
      teachingSkills: mapUserSkills(input.teachingSkills),
      learningSkills: mapUserSkills(input.learningSkills),
      privacy: {
        isDiscoverable: true,
        acceptsRequests: true,
        requestScope: "ANYONE",
        ...(input.privacy ?? {}),
      },
    };
    users.set(id, user);
    return user;
  }

  function createSessionForUser(user) {
    const id = makeUuid("41000000", nextSeq());
    const csrfToken = `csrf_${id}`;
    const session = { id, userId: user.id, csrfToken };
    sessions.set(id, session);
    return session;
  }

  async function signIn(target, emailOrUser) {
    const context = "context" in target ? target.context() : target;
    const email =
      typeof emailOrUser === "string" ? emailOrUser.trim().toLowerCase() : emailOrUser.email;
    const user = [...users.values()].find((u) => u.email === email);
    if (!user) throw new Error(`Unknown user for signIn: ${email}`);
    const session = createSessionForUser(user);
    await context.addCookies([
      {
        name: "skillswap_session",
        value: session.id,
        domain: "127.0.0.1",
        path: "/",
        httpOnly: true,
        sameSite: "Lax",
      },
    ]);
    return session;
  }

  function rotateCsrf(sessionId) {
    const session = sessions.get(sessionId);
    if (session) session.csrfToken = `csrf_rotated_${nextSeq()}`;
    return session?.csrfToken ?? "";
  }

  function addFault(rule) {
    const entry = {
      method: rule.method ?? "*",
      path: rule.path,
      times: rule.times ?? 1,
      kind: rule.kind ?? "status",
      status: rule.status ?? 500,
      code: rule.code ?? "internal_error",
      fields: rule.fields ?? undefined,
      hold: rule.hold ?? null,
    };
    faultRules.push(entry);
    return () => {
      const idx = faultRules.indexOf(entry);
      if (idx >= 0) faultRules.splice(idx, 1);
    };
  }

  function matchFault(method, path) {
    for (let i = 0; i < faultRules.length; i++) {
      const rule = faultRules[i];
      const methodOk = rule.method === "*" || rule.method === method;
      const pathOk =
        typeof rule.path === "string"
          ? rule.path === path
          : rule.path instanceof RegExp
            ? rule.path.test(path)
            : false;
      if (methodOk && pathOk && rule.times > 0) {
        rule.times -= 1;
        if (rule.times <= 0) faultRules.splice(i, 1);
        return rule;
      }
    }
    return null;
  }

  function paginate(items, url, pageSize = 20) {
    const page = Math.max(1, Number(url.searchParams.get("page")) || 1);
    return {
      items: items.slice((page - 1) * pageSize, page * pageSize),
      page,
      pageSize,
      total: items.length,
    };
  }

  function adminReportView(rep) {
    const reported = users.get(rep.reportedUserId);
    return {
      id: rep.id,
      reporter: personView(users.get(rep.reporterId)),
      reportedUser: {
        ...personView(reported),
        accountStatus: reported.accountStatus === "SUSPENDED" ? "SUSPENDED" : "ACTIVE",
      },
      reason: rep.reason,
      status: rep.status,
      createdAt: rep.createdAt,
      resolvedAt: rep.resolvedAt,
      decision: rep.decision,
    };
  }

  async function resolveSession(context, request) {
    const allHeaders = await request.allHeaders().catch(() => request.headers());
    let sid = parseSessionCookie(allHeaders.cookie ?? request.headers().cookie ?? "");
    if (!sid) {
      const cookies = await context.cookies(BASE_URL);
      sid = cookies.find((c) => c.name === "skillswap_session")?.value ?? "";
    }
    if (!sid) return null;
    const session = sessions.get(sid);
    if (!session) return null;
    const user = users.get(session.userId);
    if (!user || user.deleted || user.accountStatus !== "ACTIVE") {
      sessions.delete(sid);
      return null;
    }
    return { session, user };
  }

  async function attach(target) {
    const context = "context" in target ? target.context() : target;

    await target.route(`${MAILPIT_URL}/**`, async (route) => {
      const url = new URL(route.request().url());
      const filterTo = url.searchParams.get("to")?.trim().toLowerCase() ?? "";
      const visible = filterTo ? mailbox.filter((m) => m.to === filterTo) : mailbox;
      const rows = visible
        .map(
          (m) => `
          <article data-mail-id="${escapeHtml(m.id)}" data-mail-to="${escapeHtml(m.to)}">
            <h2>${escapeHtml(m.subject)}</h2>
            <p>Кому: <span>${escapeHtml(m.to)}</span></p>
            <p><a href="${escapeHtml(m.url)}">Відкрити посилання для ${escapeHtml(m.to)}</a></p>
          </article>`,
        )
        .join("\n");
      return route.fulfill({
        status: 200,
        contentType: "text/html; charset=utf-8",
        body: `<!doctype html>
<html lang="uk">
  <head><meta charset="utf-8" /><title>Mailpit — SkillSwap</title></head>
  <body>
    <main>
      <h1>Локальна пошта Mailpit</h1>
      ${rows || "<p>Листів поки немає.</p>"}
    </main>
  </body>
</html>`,
      });
    });

    await target.route("**/api/v1/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname.replace(/^\/api\/v1\//, "");
      const method = request.method();
      const body = (() => {
        try {
          return request.postData() ? request.postDataJSON() : {};
        } catch {
          return {};
        }
      })();

      requestLog.push({ method, path, search: url.search, body });

      const send = (json, status = 200, extraHeaders = {}) =>
        route.fulfill({
          status,
          json,
          headers: { "cache-control": "no-store", ...extraHeaders },
        });

      const problem = (status, code, fields, extraHeaders = {}) =>
        send(
          { error: { code, message: code, ...(fields ? { fields } : {}) } },
          status,
          extraHeaders,
        );

      const fault = matchFault(method, path);
      if (fault) {
        if (fault.hold) await fault.hold;
        if (fault.kind === "network" || fault.kind === "abort") return route.abort("failed");
        if (fault.kind !== "hold") return problem(fault.status, fault.code, fault.fields);
      }

      if (path === "universities" && method === "GET") {
        return send({ items: universities });
      }
      const uniFacMatch = /^universities\/([^/]+)\/faculties$/.exec(path);
      if (uniFacMatch && method === "GET") {
        return send({ items: facultiesByUniversity.get(uniFacMatch[1]) ?? [] });
      }

      if (path === "auth/register" && method === "POST") {
        const firstName = String(body.firstName ?? "").trim();
        const lastName = String(body.lastName ?? "").trim();
        const email = String(body.email ?? "")
          .trim()
          .toLowerCase();
        const password = String(body.password ?? "");
        const fields = {};
        if (!firstName || Array.from(firstName).length > 100) fields.firstName = "invalid";
        if (!lastName || Array.from(lastName).length > 100) fields.lastName = "invalid";
        const domain = email.split("@")[1] ?? "";
        const uni = universities.find((u) => u.emailDomain.toLowerCase() === domain);
        if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) fields.email = "invalid";
        else if (!uni) fields.email = "domain not registered";
        if (Array.from(password).length < 12) fields.password = "min 12 chars";
        else if (Buffer.byteLength(password, "utf8") > 72) fields.password = "max 72 bytes";
        if (Object.keys(fields).length) return problem(422, "validation_failed", fields);
        const existing = [...users.values()].find((u) => !u.deleted && u.email === email);
        if (existing) return problem(409, "email_taken");
        const user = seedUser({
          email,
          password,
          firstName,
          lastName,
          emailVerified: false,
          universityId: uni.id,
        });
        if (config.mailAvailable) {
          issueMailToken("verify", user);
          return send({ verificationEmailSent: true }, 201);
        }
        return send({ verificationEmailSent: false }, 201);
      }

      if (path === "auth/verify-email" && method === "POST") {
        const record = verifyTokens.get(String(body.token ?? ""));
        if (!record || record.used) return problem(400, "invalid_token");
        const user = users.get(record.userId);
        if (!user || user.deleted) return problem(400, "invalid_token");
        record.used = true;
        user.emailVerified = true;
        return send({ status: "verified" });
      }

      if (path === "auth/forgot-password" && method === "POST") {
        if (!config.mailAvailable) return problem(503, "mail_unavailable");
        const email = String(body.email ?? "")
          .trim()
          .toLowerCase();
        const user = [...users.values()].find(
          (u) => !u.deleted && u.accountStatus === "ACTIVE" && u.email === email,
        );
        if (user) issueMailToken("reset", user);
        return send({ status: "accepted" });
      }

      if (path === "auth/reset-password" && method === "POST") {
        const password = String(body.password ?? "");
        if (Array.from(password).length < 12 || Buffer.byteLength(password, "utf8") > 72) {
          return problem(422, "validation_failed", { password: "invalid length or 72 bytes" });
        }
        const record = resetTokens.get(String(body.token ?? ""));
        if (!record || record.used) return problem(400, "invalid_token");
        const user = users.get(record.userId);
        if (!user || user.deleted) return problem(400, "invalid_token");
        record.used = true;
        user.password = password;
        failedLogins.delete(user.email);
        revokeUserSessions(user.id);
        return send({ status: "password_reset" });
      }

      if (path === "auth/login" && method === "POST") {
        const email = String(body.email ?? "")
          .trim()
          .toLowerCase();
        const password = String(body.password ?? "");
        const failures = failedLogins.get(email) ?? 0;
        if (failures >= 5) {
          return problem(429, "login_locked", undefined, { "retry-after": "60" });
        }
        const user = [...users.values()].find((u) => !u.deleted && u.email === email);
        if (!user || user.password !== password) {
          const nextFailures = failures + 1;
          failedLogins.set(email, nextFailures);
          if (nextFailures >= 5) {
            return problem(429, "login_locked", undefined, { "retry-after": "60" });
          }
          return problem(401, "invalid_credentials");
        }
        if (user.accountStatus === "SUSPENDED") {
          return problem(403, "account_suspended");
        }
        failedLogins.delete(email);
        const session = createSessionForUser(user);
        return send(
          {
            user: {
              id: user.id,
              email: user.email,
              emailVerified: user.emailVerified,
              role: user.role,
            },
            csrfToken: session.csrfToken,
          },
          200,
          {
            "set-cookie": `skillswap_session=${session.id}; Path=/; HttpOnly; SameSite=Lax`,
          },
        );
      }

      const auth = await resolveSession(context, request);
      if (!auth) return problem(401, "unauthenticated");
      const { session, user: viewer } = auth;

      if (path === "auth/me" && method === "GET") {
        return send({
          user: {
            id: viewer.id,
            email: viewer.email,
            emailVerified: viewer.emailVerified,
            role: viewer.role,
          },
          csrfToken: session.csrfToken,
        });
      }

      if (method !== "GET" && config.enforceCsrf) {
        const headerToken = request.headers()["x-csrf-token"] ?? "";
        if (headerToken !== session.csrfToken) {
          return problem(403, "csrf_invalid");
        }
      }

      if (path === "auth/logout" && method === "POST") {
        sessions.delete(session.id);
        await context.clearCookies({ name: "skillswap_session" });
        return route.fulfill({
          status: 204,
          headers: {
            "cache-control": "no-store",
            "set-cookie": "skillswap_session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax",
          },
        });
      }

      if (path === "auth/resend-verification" && method === "POST") {
        if (viewer.emailVerified) return problem(409, "email_already_verified");
        if (!config.mailAvailable) return problem(503, "mail_unavailable");
        issueMailToken("verify", viewer);
        return send({ status: "sent" });
      }

      if (!viewer.emailVerified) {
        return problem(403, "email_not_verified");
      }

      if (path === "skill-categories" && method === "GET") {
        return send({ items: categories });
      }

      if (path === "skills" && method === "GET") {
        const q = (url.searchParams.get("q") ?? "").trim().toLocaleLowerCase("uk");
        const active = skills
          .filter((s) => s.isActive && (!q || s.name.toLocaleLowerCase("uk").includes(q)))
          .map((s) => ({ id: s.id, name: s.name, categoryId: s.categoryId }));
        return send({ items: active });
      }

      if (path === "me/profile" && method === "GET") {
        return send({ profile: profileView(viewer) });
      }

      if (path === "me/profile" && method === "PATCH") {
        const nextUniId = body.universityId ?? viewer.universityId;
        const nextFirst =
          body.firstName !== undefined ? String(body.firstName).trim() : viewer.firstName;
        const nextLast =
          body.lastName !== undefined ? String(body.lastName).trim() : viewer.lastName;
        const nextCity = body.city !== undefined ? String(body.city).trim() : viewer.city;
        const nextBio = body.bio !== undefined ? String(body.bio).trim() : viewer.bio;
        const nextCourse = body.course !== undefined ? body.course : viewer.course;
        const nextFacultyId =
          body.facultyId !== undefined
            ? body.facultyId
            : nextUniId === viewer.universityId
              ? viewer.facultyId
              : null;
        const nextFormats = body.formats !== undefined ? body.formats : viewer.formats;

        const fields = {};
        if (!nextFirst || Array.from(nextFirst).length > 100) fields.firstName = "invalid";
        if (!nextLast || Array.from(nextLast).length > 100) fields.lastName = "invalid";
        if (!universities.some((u) => u.id === nextUniId)) fields.universityId = "invalid";
        if (nextFacultyId && !findFaculty(nextUniId, nextFacultyId)) fields.facultyId = "invalid";
        if (
          nextCourse !== null &&
          (!Number.isInteger(nextCourse) || nextCourse < 1 || nextCourse > 6)
        ) {
          fields.course = "invalid";
        }
        if (Array.from(nextBio).length > 600) fields.bio = "invalid";
        if (body.formats !== undefined && (!Array.isArray(nextFormats) || !nextFormats.length)) {
          fields.formats = "invalid";
        }
        if (Array.isArray(nextFormats) && nextFormats.includes("OFFLINE") && !nextCity) {
          fields.city = "city required for offline";
        }
        if (Object.keys(fields).length) return problem(422, "validation_failed", fields);

        viewer.firstName = nextFirst;
        viewer.lastName = nextLast;
        viewer.universityId = nextUniId;
        viewer.facultyId = nextFacultyId || null;
        viewer.course = nextCourse ?? null;
        viewer.city = nextCity;
        viewer.bio = nextBio;
        viewer.formats = [...nextFormats];
        return send({ profile: profileView(viewer) });
      }

      const skillListMatch = /^me\/(teaching|learning)-skills(?:\/([^/]+))?$/.exec(path);
      if (skillListMatch) {
        const listKey = skillListMatch[1] === "teaching" ? "teachingSkills" : "learningSkills";
        const targetSkillId = skillListMatch[2] ? decodeURIComponent(skillListMatch[2]) : "";
        const list = viewer[listKey];

        if (method === "POST") {
          const catSkill = findSkill(body.skillId);
          if (!catSkill || !catSkill.isActive) return problem(404, "skill_not_found");
          if (list.some((item) => item.skillId === catSkill.id)) {
            return problem(409, "skill_already_added");
          }
          if (list.length >= 5) {
            return problem(422, "validation_failed", { skillId: "max 5 skills" });
          }
          list.push({
            skillId: catSkill.id,
            categoryId: catSkill.categoryId,
            name: catSkill.name,
            level: body.level ?? "BEGINNER",
          });
          return send({ profile: profileView(viewer) });
        }

        if (method === "PATCH") {
          const entry = list.find((item) => item.skillId === targetSkillId);
          if (!entry) return problem(404, "skill_not_found");
          entry.level = body.level;
          return send({ profile: profileView(viewer) });
        }

        if (method === "DELETE") {
          viewer[listKey] = list.filter((item) => item.skillId !== targetSkillId);
          return send({ profile: profileView(viewer) });
        }
      }

      if (path === "students" && method === "GET") {
        const q = (url.searchParams.get("q") ?? "").trim().toLocaleLowerCase("uk");
        const categoryId = url.searchParams.get("categoryId") ?? "";
        const level = url.searchParams.get("level") ?? "";
        const format = url.searchParams.get("format") ?? "";
        const mutualOnly = url.searchParams.get("mutual") === "true";

        const cards = [...users.values()]
          .filter(
            (u) =>
              u.id !== viewer.id &&
              !u.deleted &&
              u.emailVerified &&
              u.accountStatus === "ACTIVE" &&
              u.privacy.isDiscoverable &&
              u.teachingSkills.length > 0 &&
              !hasBlockBetween(viewer.id, u.id),
          )
          .filter((u) => {
            if (q) {
              const full = `${u.firstName} ${u.lastName}`.toLocaleLowerCase("uk");
              const skillHit = u.teachingSkills.some((s) =>
                s.name.toLocaleLowerCase("uk").includes(q),
              );
              if (!full.includes(q) && !skillHit) return false;
            }
            if (categoryId || level) {
              const matchingTeaching = u.teachingSkills.some(
                (s) =>
                  (!categoryId || s.categoryId === categoryId) && (!level || s.level === level),
              );
              if (!matchingTeaching) return false;
            }
            if (format && !u.formats.includes(format)) return false;
            const mutual = Boolean(computeMutual(viewer, u));
            if (mutualOnly && !mutual) return false;
            return true;
          })
          .map((u) => ({
            student: summaryView(u),
            teachingSkills: u.teachingSkills.map((s) => ({ ...s })),
            formats: [...u.formats],
            mutual: Boolean(computeMutual(viewer, u)),
          }));

        return send(paginate(cards, url, 20));
      }

      if (path === "me/matches" && method === "GET") {
        if (!isEligible(viewer)) {
          return send({ items: [], page: 1, pageSize: 20, total: 0, eligibleForMatching: false });
        }
        const items = [...users.values()]
          .filter(
            (u) =>
              u.id !== viewer.id &&
              !u.deleted &&
              u.emailVerified &&
              u.accountStatus === "ACTIVE" &&
              u.privacy.isDiscoverable &&
              !hasBlockBetween(viewer.id, u.id),
          )
          .map((u) => {
            const mutual = computeMutual(viewer, u);
            return mutual ? { student: summaryView(u), ...mutual } : null;
          })
          .filter(Boolean);
        return send({ ...paginate(items, url, 20), eligibleForMatching: true });
      }

      const studentReviewsMatch = /^students\/([^/]+)\/reviews$/.exec(path);
      if (studentReviewsMatch && method === "GET") {
        const target = users.get(decodeURIComponent(studentReviewsMatch[1]));
        if (
          !target ||
          target.id === viewer.id ||
          target.deleted ||
          target.accountStatus !== "ACTIVE" ||
          !target.privacy.isDiscoverable ||
          hasBlockBetween(viewer.id, target.id)
        ) {
          return problem(404, "student_not_found");
        }
        const { averageRating, reviewCount, list } = ratingSummary(target.id);
        return send({ ...paginate(list, url, 20), averageRating, reviewCount });
      }

      if (path === "me/reviews" && method === "GET") {
        const { averageRating, reviewCount, list } = ratingSummary(viewer.id);
        return send({ ...paginate(list, url, 20), averageRating, reviewCount });
      }

      const studentBlockMatch = /^students\/([^/]+)\/block$/.exec(path);
      if (studentBlockMatch) {
        const targetId = decodeURIComponent(studentBlockMatch[1]);
        const target = users.get(targetId);
        if (!target || target.id === viewer.id || target.deleted) {
          return problem(404, "student_not_found");
        }
        if (method === "POST") {
          if (!isBlockedBy(viewer.id, targetId)) {
            blocks.unshift({ blockerId: viewer.id, blockedId: targetId, createdAt: nowIso() });
          }
          const { closedRequests, cancelledExchanges } = cancelRelationshipsBetween(
            viewer.id,
            targetId,
            "Один з учасників заблокував іншого",
          );
          return send({
            block: { user: personView(target), createdAt: nowIso() },
            cancelledExchanges,
            closedRequests,
          });
        }
        if (method === "DELETE") {
          const idx = blocks.findIndex(
            (b) => b.blockerId === viewer.id && b.blockedId === targetId,
          );
          if (idx >= 0) blocks.splice(idx, 1);
          return route.fulfill({ status: 204, headers: { "cache-control": "no-store" } });
        }
      }

      const studentProfileMatch = /^students\/([^/]+)$/.exec(path);
      if (studentProfileMatch && method === "GET") {
        const target = users.get(decodeURIComponent(studentProfileMatch[1]));
        if (
          !target ||
          target.id === viewer.id ||
          target.deleted ||
          !target.emailVerified ||
          target.accountStatus !== "ACTIVE" ||
          !target.privacy.isDiscoverable ||
          hasBlockBetween(viewer.id, target.id)
        ) {
          return problem(404, "student_not_found");
        }
        const { averageRating, reviewCount, list } = ratingSummary(target.id);
        const uni = findUniversity(target.universityId);
        const fac = findFaculty(target.universityId, target.facultyId);
        return send({
          student: {
            id: target.id,
            firstName: target.firstName,
            lastName: target.lastName,
            university: { id: uni.id, name: uni.name },
            faculty: fac ? { id: fac.id, name: fac.name } : null,
            course: target.course,
            city: target.city,
            bio: target.bio,
            formats: [...target.formats],
            teachingSkills: target.teachingSkills.map((s) => ({ ...s })),
            learningSkills: target.learningSkills.map((s) => ({ ...s })),
            averageRating,
            reviewCount,
            reviews: list.slice(0, 5),
            mutual: computeMutual(viewer, target),
          },
        });
      }

      if (path === "exchange-requests" && method === "POST") {
        const recipient = users.get(body.recipientId);
        if (
          !recipient ||
          recipient.id === viewer.id ||
          recipient.deleted ||
          recipient.accountStatus !== "ACTIVE" ||
          !recipient.privacy.isDiscoverable ||
          hasBlockBetween(viewer.id, recipient.id)
        ) {
          return problem(404, "student_not_found");
        }
        if (!recipient.privacy.acceptsRequests) return problem(409, "requests_closed");
        if (
          recipient.privacy.requestScope === "SAME_UNIVERSITY" &&
          recipient.universityId !== viewer.universityId
        ) {
          return problem(409, "same_university_only");
        }

        const viewerTeaches = viewer.teachingSkills.find((s) => s.skillId === body.teachSkillId);
        const recipientWants = recipient.learningSkills.find(
          (s) => s.skillId === body.teachSkillId,
        );
        const recipientTeaches = recipient.teachingSkills.find(
          (s) => s.skillId === body.learnSkillId,
        );
        const viewerWants = viewer.learningSkills.find((s) => s.skillId === body.learnSkillId);
        const common = commonFormatsFor(viewer, recipient);

        const fields = {};
        if (!viewerTeaches || !recipientWants) fields.teachSkillId = "invalid";
        if (!recipientTeaches || !viewerWants || body.teachSkillId === body.learnSkillId) {
          fields.learnSkillId = "invalid";
        }
        if (!common.includes(body.format)) fields.format = "invalid";
        if (
          Number(body.teachSessions) * Number(body.teachDurationMinutes) !==
          Number(body.learnSessions) * Number(body.learnDurationMinutes)
        ) {
          fields.learnSessions = "minutes must match";
        }
        const msg = body.message ? String(body.message).trim() : null;
        if (msg && Array.from(msg).length > 500) fields.message = "too long";
        if (Object.keys(fields).length) return problem(422, "validation_failed", fields);

        const duplicate = [...requests.values()].some((r) => {
          if (r.status !== "PENDING") return false;
          const samePair =
            (r.requesterId === viewer.id &&
              r.recipientId === recipient.id &&
              r.requesterTeaches.skill.skillId === body.teachSkillId &&
              r.recipientTeaches.skill.skillId === body.learnSkillId) ||
            (r.requesterId === recipient.id &&
              r.recipientId === viewer.id &&
              r.requesterTeaches.skill.skillId === body.learnSkillId &&
              r.recipientTeaches.skill.skillId === body.teachSkillId);
          return samePair;
        });
        if (duplicate) return problem(409, "duplicate_request");

        const id = makeUuid("42000000", nextSeq());
        const createdAt = nowIso();
        const req = {
          id,
          status: "PENDING",
          format: body.format,
          totalSessions: Number(body.teachSessions) + Number(body.learnSessions),
          message: msg,
          requesterId: viewer.id,
          recipientId: recipient.id,
          requesterTeaches: {
            skill: {
              skillId: viewerTeaches.skillId,
              categoryId: viewerTeaches.categoryId,
              name: viewerTeaches.name,
            },
            teacherLevel: viewerTeaches.level,
            learnerLevel: recipientWants.level,
            sessions: Number(body.teachSessions),
            durationMinutes: Number(body.teachDurationMinutes),
          },
          recipientTeaches: {
            skill: {
              skillId: recipientTeaches.skillId,
              categoryId: recipientTeaches.categoryId,
              name: recipientTeaches.name,
            },
            teacherLevel: recipientTeaches.level,
            learnerLevel: viewerWants.level,
            sessions: Number(body.learnSessions),
            durationMinutes: Number(body.learnDurationMinutes),
          },
          exchangeId: null,
          createdAt,
          respondedAt: null,
          history: [{ status: "PENDING", changedBy: personView(viewer), createdAt }],
        };
        requests.set(id, req);
        const conv = ensureConversationForRequest(req);
        addNotification({
          recipientId: recipient.id,
          actorId: viewer.id,
          eventKey: `request:${req.id}`,
          eventType: "EXCHANGE_REQUEST",
          title: `${viewer.firstName} ${viewer.lastName} пропонує обмін навичками`,
          body: `${req.requesterTeaches.skill.name} ⇄ ${req.recipientTeaches.skill.name}`,
          target: {
            kind: "EXCHANGE_REQUEST",
            id: req.id,
            requestId: req.id,
            exchangeId: null,
            conversationId: conv.id,
            sessionId: null,
          },
          requestId: req.id,
          conversationId: conv.id,
        });
        return send({ request: requestView(req) }, 201);
      }

      if (path === "me/exchange-requests" && method === "GET") {
        const direction =
          url.searchParams.get("direction") === "outgoing" ? "outgoing" : "incoming";
        const status = url.searchParams.get("status") ?? "";
        const items = [...requests.values()]
          .filter((r) =>
            direction === "outgoing" ? r.requesterId === viewer.id : r.recipientId === viewer.id,
          )
          .filter((r) => !status || r.status === status)
          .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
          .map(requestView);
        return send(paginate(items, url, 20));
      }

      const reqConvMatch = /^exchange-requests\/([^/]+)\/conversation$/.exec(path);
      if (reqConvMatch && method === "GET") {
        const req = requests.get(decodeURIComponent(reqConvMatch[1]));
        if (!req || (req.requesterId !== viewer.id && req.recipientId !== viewer.id)) {
          return problem(404, "request_not_found");
        }
        const conv = ensureConversationForRequest(req);
        return send({ conversation: conversationView(conv, viewer.id) });
      }

      const reqActionMatch = /^exchange-requests\/([^/]+)\/(accept|decline|withdraw)$/.exec(path);
      if (reqActionMatch && method === "POST") {
        const req = requests.get(decodeURIComponent(reqActionMatch[1]));
        const action = reqActionMatch[2];
        if (!req || (req.requesterId !== viewer.id && req.recipientId !== viewer.id)) {
          return problem(404, "request_not_found");
        }
        if (action === "withdraw" && req.requesterId !== viewer.id) {
          return problem(403, "action_not_allowed");
        }
        if ((action === "accept" || action === "decline") && req.recipientId !== viewer.id) {
          return problem(403, "action_not_allowed");
        }

        if (action === "accept" && req.status === "ACCEPTED" && req.exchangeId) {
          const existingEx = exchanges.get(req.exchangeId);
          return send({
            request: requestView(req),
            exchange: exchangeView(existingEx, viewer.id),
          });
        }
        if (req.status !== "PENDING") {
          return problem(409, "request_not_pending");
        }

        const stamp = nowIso();
        if (action === "withdraw" || action === "decline") {
          req.status = action === "withdraw" ? "WITHDRAWN" : "DECLINED";
          req.respondedAt = stamp;
          req.history.push({ status: req.status, changedBy: personView(viewer), createdAt: stamp });
          if (action === "decline") {
            const conv = ensureConversationForRequest(req);
            addNotification({
              recipientId: req.requesterId,
              actorId: viewer.id,
              eventKey: `request-decision:${req.id}`,
              eventType: "REQUEST_RESPONSE",
              title: `${viewer.firstName} ${viewer.lastName} відхилив(-ла) запит на обмін`,
              body: `${req.requesterTeaches.skill.name} ⇄ ${req.recipientTeaches.skill.name}`,
              target: {
                kind: "EXCHANGE_REQUEST",
                id: req.id,
                requestId: req.id,
                exchangeId: null,
                conversationId: conv.id,
                sessionId: null,
              },
              requestId: req.id,
              conversationId: conv.id,
            });
          }
          return send({ request: requestView(req) });
        }

        const exchangeId = makeUuid("43000000", nextSeq());
        req.status = "ACCEPTED";
        req.exchangeId = exchangeId;
        req.respondedAt = stamp;
        req.history.push({ status: "ACCEPTED", changedBy: personView(viewer), createdAt: stamp });

        const ex = {
          id: exchangeId,
          requestId: req.id,
          status: "ACTIVE",
          format: req.format,
          totalSessions: req.totalSessions,
          requesterId: req.requesterId,
          recipientId: req.recipientId,
          requesterTeaches: structuredClone(req.requesterTeaches),
          recipientTeaches: structuredClone(req.recipientTeaches),
          completionConfirmedBy: new Set(),
          startedAt: stamp,
          completedAt: null,
          cancelledAt: null,
          cancellationReason: null,
          createdAt: stamp,
          history: [
            { status: "ACTIVE", changedBy: personView(viewer), reason: null, createdAt: stamp },
          ],
        };
        exchanges.set(exchangeId, ex);

        let order = 1;
        for (let i = 0; i < ex.requesterTeaches.sessions; i++) {
          const lid = makeUuid("44000000", nextSeq());
          lessons.set(lid, {
            id: lid,
            exchangeId,
            planOrder: order,
            title: `Заняття ${order}: ${ex.requesterTeaches.skill.name}`,
            description: null,
            status: "DRAFT",
            format: ex.format,
            skill: { ...ex.requesterTeaches.skill },
            plannedDurationMinutes: ex.requesterTeaches.durationMinutes,
            teacherId: ex.requesterId,
            learnerId: ex.recipientId,
            startsAt: null,
            endsAt: null,
            durationMinutes: null,
            meetingPlace: null,
            meetingUrl: null,
            activeProposalId: null,
            proposals: [],
            scheduleConfirmedBy: new Set(),
            completionConfirmedBy: new Set(),
            completedAt: null,
            cancelledAt: null,
            cancellationReason: null,
          });
          order++;
        }
        for (let i = 0; i < ex.recipientTeaches.sessions; i++) {
          const lid = makeUuid("44000000", nextSeq());
          lessons.set(lid, {
            id: lid,
            exchangeId,
            planOrder: order,
            title: `Заняття ${order}: ${ex.recipientTeaches.skill.name}`,
            description: null,
            status: "DRAFT",
            format: ex.format,
            skill: { ...ex.recipientTeaches.skill },
            plannedDurationMinutes: ex.recipientTeaches.durationMinutes,
            teacherId: ex.recipientId,
            learnerId: ex.requesterId,
            startsAt: null,
            endsAt: null,
            durationMinutes: null,
            meetingPlace: null,
            meetingUrl: null,
            activeProposalId: null,
            proposals: [],
            scheduleConfirmedBy: new Set(),
            completionConfirmedBy: new Set(),
            completedAt: null,
            cancelledAt: null,
            cancellationReason: null,
          });
          order++;
        }

        const conv = conversationForExchange(ex);
        addNotification({
          recipientId: req.requesterId,
          actorId: viewer.id,
          eventKey: `request-decision:${req.id}`,
          eventType: "REQUEST_RESPONSE",
          title: `${viewer.firstName} ${viewer.lastName} прийняв(-ла) запит на обмін`,
          body: `${ex.requesterTeaches.skill.name} ⇄ ${ex.recipientTeaches.skill.name}`,
          target: {
            kind: "EXCHANGE",
            id: ex.id,
            requestId: req.id,
            exchangeId: ex.id,
            conversationId: conv.id,
            sessionId: null,
          },
          requestId: req.id,
          exchangeId: ex.id,
          conversationId: conv.id,
        });

        return send({ request: requestView(req), exchange: exchangeView(ex, viewer.id) });
      }

      const reqDetailMatch = /^exchange-requests\/([^/]+)$/.exec(path);
      if (reqDetailMatch && method === "GET") {
        const req = requests.get(decodeURIComponent(reqDetailMatch[1]));
        if (!req || (req.requesterId !== viewer.id && req.recipientId !== viewer.id)) {
          return problem(404, "request_not_found");
        }
        return send({ request: requestView(req), history: statusHistoryView(req.history) });
      }

      if (path === "me/exchanges" && method === "GET") {
        const status = url.searchParams.get("status") ?? "ACTIVE";
        const items = [...exchanges.values()]
          .filter((ex) => ex.requesterId === viewer.id || ex.recipientId === viewer.id)
          .filter((ex) => !status || ex.status === status)
          .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
          .map((ex) => exchangeView(ex, viewer.id));
        return send(paginate(items, url, 20));
      }

      const exConvMatch = /^exchanges\/([^/]+)\/conversation$/.exec(path);
      if (exConvMatch && method === "GET") {
        const ex = exchanges.get(decodeURIComponent(exConvMatch[1]));
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "exchange_not_found");
        }
        const conv = conversationForExchange(ex);
        return send({ conversation: conversationView(conv, viewer.id) });
      }

      const exSessionsMatch = /^exchanges\/([^/]+)\/sessions$/.exec(path);
      if (exSessionsMatch && method === "GET") {
        const ex = exchanges.get(decodeURIComponent(exSessionsMatch[1]));
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "exchange_not_found");
        }
        const items = exchangeLessonsList(ex.id).map((les) => lessonView(les, viewer.id));
        return send({ exchange: exchangeView(ex, viewer.id), items });
      }

      const exReviewsMatch = /^exchanges\/([^/]+)\/reviews$/.exec(path);
      if (exReviewsMatch) {
        const ex = exchanges.get(decodeURIComponent(exReviewsMatch[1]));
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "exchange_not_found");
        }
        const partnerId = ex.requesterId === viewer.id ? ex.recipientId : ex.requesterId;
        const partner = users.get(partnerId);
        if (method === "GET") {
          const myRev = [...reviews.values()].find(
            (r) => r.exchangeId === ex.id && r.authorId === viewer.id,
          );
          const partnerRev = [...reviews.values()].find(
            (r) => r.exchangeId === ex.id && r.authorId === partnerId,
          );
          const toDto = (r) =>
            r
              ? {
                  id: r.id,
                  exchangeId: r.exchangeId,
                  author: personView(users.get(r.authorId)),
                  rating: r.rating,
                  comment: r.comment,
                  createdAt: r.createdAt,
                }
              : null;
          return send({
            myReview: toDto(myRev),
            partnerReview: toDto(partnerRev),
            canReview: ex.status === "COMPLETED" && !myRev && !partner.deleted,
          });
        }
        if (method === "POST") {
          if (ex.status !== "COMPLETED") return problem(409, "exchange_not_completed");
          const existing = [...reviews.values()].find(
            (r) => r.exchangeId === ex.id && r.authorId === viewer.id,
          );
          if (existing) return problem(409, "review_already_exists");
          const rating = Number(body.rating);
          const comment = String(body.comment ?? "").trim();
          const fields = {};
          if (!Number.isInteger(rating) || rating < 1 || rating > 5) fields.rating = "1..5";
          if (Array.from(comment).length > 1000) fields.comment = "max 1000";
          if (Object.keys(fields).length) return problem(422, "validation_failed", fields);
          const id = makeUuid("48000000", nextSeq());
          const createdAt = nowIso();
          const rev = {
            id,
            exchangeId: ex.id,
            authorId: viewer.id,
            targetUserId: partnerId,
            rating,
            comment,
            createdAt,
          };
          reviews.set(id, rev);
          addNotification({
            recipientId: partnerId,
            actorId: viewer.id,
            eventKey: `review:${id}`,
            eventType: "REVIEW_RECEIVED",
            title: `${viewer.firstName} ${viewer.lastName} залишив(-ла) відгук про обмін`,
            body: comment || `Оцінка: ${rating} з 5`,
            target: {
              kind: "EXCHANGE",
              id: ex.id,
              requestId: ex.requestId,
              exchangeId: ex.id,
              conversationId: conversationForExchange(ex).id,
              sessionId: null,
            },
            exchangeId: ex.id,
            reviewId: id,
          });
          return send(
            {
              review: {
                id,
                exchangeId: ex.id,
                author: personView(viewer),
                rating,
                comment,
                createdAt,
              },
              exchange: exchangeView(ex, viewer.id),
            },
            201,
          );
        }
      }

      const exActionMatch = /^exchanges\/([^/]+)\/(complete|cancel)$/.exec(path);
      if (exActionMatch && method === "POST") {
        const ex = exchanges.get(decodeURIComponent(exActionMatch[1]));
        const action = exActionMatch[2];
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "exchange_not_found");
        }
        if (action === "complete") {
          if (ex.status === "COMPLETED") return send({ exchange: exchangeView(ex, viewer.id) });
          if (ex.status !== "ACTIVE") return problem(409, "exchange_not_active");
          if (hasBlockBetween(ex.requesterId, ex.recipientId)) {
            return problem(409, "exchange_blocked");
          }
          const progress = computeExchangeProgress(ex);
          if (!progress.planCompleted) return problem(409, "plan_not_completed");
          ex.completionConfirmedBy.add(viewer.id);
          if (ex.completionConfirmedBy.size >= 2) {
            ex.status = "COMPLETED";
            ex.completedAt = nowIso();
            ex.history.push({
              status: "COMPLETED",
              changedBy: personView(viewer),
              reason: null,
              createdAt: nowIso(),
            });
          }
          return send({ exchange: exchangeView(ex, viewer.id) });
        }
        if (action === "cancel") {
          if (ex.status !== "ACTIVE") return problem(409, "exchange_not_active");
          const reason = String(body.reason ?? "").trim();
          if (!reason || Array.from(reason).length > 500) {
            return problem(422, "validation_failed", { reason: "invalid" });
          }
          const stamp = nowIso();
          ex.status = "CANCELLED";
          ex.cancelledAt = stamp;
          ex.cancellationReason = reason;
          ex.history.push({
            status: "CANCELLED",
            changedBy: personView(viewer),
            reason,
            createdAt: stamp,
          });
          for (const les of exchangeLessonsList(ex.id)) {
            if (les.status !== "COMPLETED") {
              les.status = "CANCELLED";
              les.cancelledAt = stamp;
              les.cancellationReason = reason;
            }
          }
          return send({ exchange: exchangeView(ex, viewer.id) });
        }
      }

      const exDetailMatch = /^exchanges\/([^/]+)$/.exec(path);
      if (exDetailMatch && method === "GET") {
        const ex = exchanges.get(decodeURIComponent(exDetailMatch[1]));
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "exchange_not_found");
        }
        return send({
          exchange: exchangeView(ex, viewer.id),
          history: statusHistoryView(ex.history),
        });
      }

      const sessionProposalsMatch = /^sessions\/([^/]+)\/proposals$/.exec(path);
      if (sessionProposalsMatch && method === "POST") {
        const les = lessons.get(decodeURIComponent(sessionProposalsMatch[1]));
        if (!les) return problem(404, "session_not_found");
        const ex = exchanges.get(les.exchangeId);
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "session_not_found");
        }
        if (ex.status !== "ACTIVE") return problem(409, "exchange_not_active");
        if (hasBlockBetween(ex.requesterId, ex.recipientId))
          return problem(409, "exchange_blocked");
        if (!["DRAFT", "PROPOSED", "SCHEDULED"].includes(les.status)) {
          return problem(409, "session_not_schedulable");
        }
        const startMs = Date.parse(body.startsAt);
        const endMs = Date.parse(body.endsAt);
        if (!Number.isFinite(startMs) || startMs <= nowMs()) {
          return problem(409, "session_in_past");
        }
        const durationMinutes = Math.round((endMs - startMs) / 60000);
        if (!Number.isFinite(endMs) || durationMinutes < 15 || durationMinutes > 240) {
          return problem(422, "validation_failed", { endsAt: "invalid" });
        }
        if (
          hasScheduleConflict([ex.requesterId, ex.recipientId], body.startsAt, body.endsAt, les.id)
        ) {
          return problem(409, "session_conflict");
        }

        if (les.status === "DRAFT" && body.teacherId && body.teacherId !== les.teacherId) {
          const otherId = body.teacherId === ex.requesterId ? ex.recipientId : ex.requesterId;
          const terms =
            body.teacherId === ex.requesterId ? ex.requesterTeaches : ex.recipientTeaches;
          les.teacherId = body.teacherId;
          les.learnerId = otherId;
          les.skill = { ...terms.skill };
          les.plannedDurationMinutes = terms.durationMinutes;
        }

        for (const p of les.proposals) {
          if (p.status === "PENDING" || p.status === "ACCEPTED") p.status = "SUPERSEDED";
        }
        const propId = makeUuid("44100000", nextSeq());
        const prop = {
          id: propId,
          sessionId: les.id,
          status: "PENDING",
          proposedById: viewer.id,
          startsAt: body.startsAt,
          endsAt: body.endsAt,
          format: body.format,
          meetingPlace: body.format === "OFFLINE" ? String(body.meetingPlace ?? "").trim() : null,
          meetingUrl: body.format === "ONLINE" ? String(body.meetingUrl ?? "").trim() : null,
          respondedById: null,
          respondedAt: null,
          createdAt: nowIso(),
        };
        les.proposals.unshift(prop);
        les.activeProposalId = propId;
        les.status = "PROPOSED";
        les.title = String(body.title ?? "").trim();
        les.description = String(body.description ?? "").trim() || null;
        les.format = body.format;
        les.startsAt = body.startsAt;
        les.endsAt = body.endsAt;
        les.durationMinutes = durationMinutes;
        les.meetingPlace = prop.meetingPlace;
        les.meetingUrl = prop.meetingUrl;
        les.scheduleConfirmedBy = new Set([viewer.id]);

        const partnerId = ex.requesterId === viewer.id ? ex.recipientId : ex.requesterId;
        addNotification({
          recipientId: partnerId,
          actorId: viewer.id,
          eventKey: `session-prop:${propId}`,
          eventType: "SESSION_UPDATED",
          title: `${viewer.firstName} ${viewer.lastName} пропонує час заняття «${les.title}»`,
          body: les.skill.name,
          target: {
            kind: "SESSION",
            id: les.id,
            requestId: ex.requestId,
            exchangeId: ex.id,
            conversationId: conversationForExchange(ex).id,
            sessionId: les.id,
          },
          exchangeId: ex.id,
          sessionId: les.id,
        });

        return send({ session: lessonView(les, viewer.id), proposal: proposalView(prop) }, 201);
      }

      const sessionActionMatch = /^sessions\/([^/]+)\/(accept|decline|cancel|confirm)$/.exec(path);
      if (sessionActionMatch && method === "POST") {
        const les = lessons.get(decodeURIComponent(sessionActionMatch[1]));
        const action = sessionActionMatch[2];
        if (!les) return problem(404, "session_not_found");
        const ex = exchanges.get(les.exchangeId);
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "session_not_found");
        }
        const partnerId = ex.requesterId === viewer.id ? ex.recipientId : ex.requesterId;

        if (action === "accept" || action === "decline") {
          const activeProp = les.proposals.find((p) => p.id === les.activeProposalId);
          if (les.status !== "PROPOSED" || !activeProp) return problem(409, "session_not_proposed");
          if (activeProp.proposedById === viewer.id) return problem(403, "action_not_allowed");
          if (
            action === "accept" &&
            hasScheduleConflict([ex.requesterId, ex.recipientId], les.startsAt, les.endsAt, les.id)
          ) {
            return problem(409, "session_conflict");
          }
          activeProp.respondedById = viewer.id;
          activeProp.respondedAt = nowIso();
          if (action === "accept") {
            activeProp.status = "ACCEPTED";
            les.status = "SCHEDULED";
            les.scheduleConfirmedBy.add(viewer.id);
          } else {
            activeProp.status = "DECLINED";
            les.status = "DRAFT";
            les.activeProposalId = null;
            les.startsAt = null;
            les.endsAt = null;
            les.durationMinutes = null;
            les.meetingPlace = null;
            les.meetingUrl = null;
            les.scheduleConfirmedBy = new Set();
          }
          addNotification({
            recipientId: partnerId,
            actorId: viewer.id,
            eventKey: `session-decide:${activeProp.id}:${action}`,
            eventType: "SESSION_UPDATED",
            title:
              action === "accept"
                ? `${viewer.firstName} ${viewer.lastName} погодив(-ла) час заняття «${les.title}»`
                : `${viewer.firstName} ${viewer.lastName} відхилив(-ла) час заняття «${les.title}»`,
            body: les.skill.name,
            target: {
              kind: "SESSION",
              id: les.id,
              requestId: ex.requestId,
              exchangeId: ex.id,
              conversationId: conversationForExchange(ex).id,
              sessionId: les.id,
            },
            exchangeId: ex.id,
            sessionId: les.id,
          });
          return send({ session: lessonView(les, viewer.id) });
        }

        if (action === "cancel") {
          if (!["PROPOSED", "SCHEDULED"].includes(les.status)) {
            return problem(409, "session_not_cancellable");
          }
          const reason = String(body.reason ?? "").trim();
          if (!reason || Array.from(reason).length > 500) {
            return problem(422, "validation_failed", { reason: "invalid" });
          }
          les.status = "CANCELLED";
          les.cancelledAt = nowIso();
          les.cancellationReason = reason;
          return send({ session: lessonView(les, viewer.id) });
        }

        if (action === "confirm") {
          if (ex.status !== "ACTIVE") return problem(409, "exchange_not_active");
          if (hasBlockBetween(ex.requesterId, ex.recipientId)) {
            return problem(409, "exchange_blocked");
          }
          if (les.status === "COMPLETED") {
            return send({
              session: lessonView(les, viewer.id),
              exchange: exchangeView(ex, viewer.id),
            });
          }
          if (les.status !== "SCHEDULED") return problem(409, "session_not_confirmable");
          if (!les.endsAt || Date.parse(les.endsAt) > nowMs()) {
            return problem(409, "session_not_finished");
          }
          les.completionConfirmedBy.add(viewer.id);
          if (les.completionConfirmedBy.size >= 2) {
            les.status = "COMPLETED";
            les.completedAt = nowIso();
          }
          return send({
            session: lessonView(les, viewer.id),
            exchange: exchangeView(ex, viewer.id),
          });
        }
      }

      const sessionDetailMatch = /^sessions\/([^/]+)$/.exec(path);
      if (sessionDetailMatch && method === "GET") {
        const les = lessons.get(decodeURIComponent(sessionDetailMatch[1]));
        if (!les) return problem(404, "session_not_found");
        const ex = exchanges.get(les.exchangeId);
        if (!ex || (ex.requesterId !== viewer.id && ex.recipientId !== viewer.id)) {
          return problem(404, "session_not_found");
        }
        return send({
          session: lessonView(les, viewer.id),
          proposals: les.proposals.map(proposalView),
        });
      }

      if (path === "me/schedule" && method === "GET") {
        const status = url.searchParams.get("status") ?? "SCHEDULED";
        const upcoming = url.searchParams.get("upcoming") !== "false";
        const ownLessons = [...lessons.values()].filter((les) => {
          const ex = exchanges.get(les.exchangeId);
          return ex && (ex.requesterId === viewer.id || ex.recipientId === viewer.id);
        });
        const pendingProposalsCount = ownLessons.filter((les) => {
          const activeProp = les.proposals.find((p) => p.id === les.activeProposalId);
          return les.status === "PROPOSED" && activeProp && activeProp.proposedById !== viewer.id;
        }).length;
        const filtered = ownLessons
          .filter((les) => les.status !== "DRAFT")
          .filter((les) => status === "ALL" || les.status === status)
          .filter((les) => !upcoming || (les.endsAt && Date.parse(les.endsAt) > nowMs()))
          .sort((a, b) => Date.parse(a.startsAt ?? "0") - Date.parse(b.startsAt ?? "0"))
          .map((les) => lessonView(les, viewer.id));
        return send({
          ...paginate(filtered, url, 20),
          timezone: "Europe/Kyiv",
          pendingProposalsCount,
        });
      }

      if (path === "me/conversations" && method === "GET") {
        const items = [...conversations.values()]
          .filter((c) => c.requesterId === viewer.id || c.recipientId === viewer.id)
          .map((c) => conversationView(c, viewer.id))
          .sort(
            (a, b) =>
              Date.parse(b.lastMessageAt ?? b.createdAt) -
              Date.parse(a.lastMessageAt ?? a.createdAt),
          );
        return send(paginate(items, url, 20));
      }

      const convMessagesMatch = /^conversations\/([^/]+)\/messages$/.exec(path);
      if (convMessagesMatch) {
        const conv = conversations.get(decodeURIComponent(convMessagesMatch[1]));
        if (!conv || (conv.requesterId !== viewer.id && conv.recipientId !== viewer.id)) {
          return problem(404, "conversation_not_found");
        }
        const list = messages.get(conv.id) ?? [];

        if (method === "GET") {
          const after = url.searchParams.get("after") ?? "";
          if (after) {
            const idx = list.findIndex((m) => m.id === after);
            const slice = idx >= 0 ? list.slice(idx + 1) : list;
            return send({
              items: slice.slice(0, 20).map(messageView),
              page: 1,
              pageSize: 20,
              total: slice.length,
              pollIntervalSeconds: config.pollIntervalSeconds,
            });
          }
          const pageData = paginate(list.map(messageView), url, 20);
          return send({ ...pageData, pollIntervalSeconds: config.pollIntervalSeconds });
        }

        if (method === "POST") {
          const view = conversationView(conv, viewer.id);
          if (!view.canSend) return problem(409, "conversation_blocked");
          const text = String(body.body ?? "").trim();
          if (!text || Array.from(text).length > 2000) {
            return problem(422, "validation_failed", { body: "invalid" });
          }
          const id = makeUuid("46000000", nextSeq());
          const msg = {
            id,
            conversationId: conv.id,
            authorId: viewer.id,
            type: "TEXT",
            body: text,
            createdAt: nowIso(),
            editedAt: null,
            readAt: null,
          };
          list.push(msg);
          const partnerId = conv.requesterId === viewer.id ? conv.recipientId : conv.requesterId;
          addNotification({
            recipientId: partnerId,
            actorId: viewer.id,
            eventKey: `msg:${id}`,
            eventType: "CHAT_MESSAGE",
            title: `Нове повідомлення від ${viewer.firstName} ${viewer.lastName}`,
            body: text.slice(0, 120),
            target: {
              kind: "CONVERSATION",
              id: conv.id,
              requestId: conv.requestId,
              exchangeId: conv.exchangeId,
              conversationId: conv.id,
              sessionId: null,
            },
            requestId: conv.requestId,
            exchangeId: conv.exchangeId,
            conversationId: conv.id,
            messageId: id,
          });
          return send({ message: messageView(msg) }, 201);
        }
      }

      const convReadMatch = /^conversations\/([^/]+)\/read$/.exec(path);
      if (convReadMatch && method === "POST") {
        const conv = conversations.get(decodeURIComponent(convReadMatch[1]));
        if (!conv || (conv.requesterId !== viewer.id && conv.recipientId !== viewer.id)) {
          return problem(404, "conversation_not_found");
        }
        const list = messages.get(conv.id) ?? [];
        const stamp = nowIso();
        let readCount = 0;
        for (const msg of list) {
          if (msg.authorId && msg.authorId !== viewer.id && !msg.readAt) {
            msg.readAt = stamp;
            readCount++;
          }
        }
        for (const notif of notifications.values()) {
          if (
            notif.recipientId === viewer.id &&
            notif.conversationId === conv.id &&
            notif.eventType === "CHAT_MESSAGE" &&
            !notif.isRead
          ) {
            notif.isRead = true;
            notif.readAt = stamp;
          }
        }
        return send({ conversation: conversationView(conv, viewer.id), readCount });
      }

      const convDetailMatch = /^conversations\/([^/]+)$/.exec(path);
      if (convDetailMatch && method === "GET") {
        const conv = conversations.get(decodeURIComponent(convDetailMatch[1]));
        if (!conv || (conv.requesterId !== viewer.id && conv.recipientId !== viewer.id)) {
          return problem(404, "conversation_not_found");
        }
        return send({ conversation: conversationView(conv, viewer.id) });
      }

      if (path === "me/notifications" && method === "GET") {
        const unreadOnly = url.searchParams.get("unreadOnly") === "true";
        const own = [...notifications.values()]
          .filter((n) => n.recipientId === viewer.id)
          .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
        const unreadCount = own.filter((n) => !n.isRead).length;
        const filtered = unreadOnly ? own.filter((n) => !n.isRead) : own;
        return send({ ...paginate(filtered, url, 20), unreadCount });
      }

      if (path === "me/notifications/read-all" && method === "POST") {
        const stamp = nowIso();
        let markedCount = 0;
        for (const n of notifications.values()) {
          if (n.recipientId === viewer.id && !n.isRead) {
            n.isRead = true;
            n.readAt = stamp;
            markedCount++;
          }
        }
        return send({ markedCount, unreadCount: 0 });
      }

      const notifReadMatch = /^me\/notifications\/([^/]+)\/read$/.exec(path);
      if (notifReadMatch && method === "POST") {
        const notif = notifications.get(decodeURIComponent(notifReadMatch[1]));
        if (!notif || notif.recipientId !== viewer.id) {
          return problem(404, "notification_not_found");
        }
        if (!notif.isRead) {
          notif.isRead = true;
          notif.readAt = nowIso();
        }
        const unreadCount = [...notifications.values()].filter(
          (n) => n.recipientId === viewer.id && !n.isRead,
        ).length;
        return send({ notification: notif, unreadCount });
      }

      if (path === "me/privacy") {
        if (method === "PATCH") {
          viewer.privacy = {
            isDiscoverable: Boolean(body.isDiscoverable),
            acceptsRequests: Boolean(body.acceptsRequests),
            requestScope: body.requestScope === "SAME_UNIVERSITY" ? "SAME_UNIVERSITY" : "ANYONE",
          };
        }
        return send({ privacy: { ...viewer.privacy } });
      }

      if (path === "auth/change-password" && method === "POST") {
        const currentPassword = String(body.currentPassword ?? "");
        const newPassword = String(body.newPassword ?? "");
        if (currentPassword !== viewer.password) {
          return problem(422, "validation_failed", { currentPassword: "invalid" });
        }
        if (newPassword === currentPassword) {
          return problem(422, "validation_failed", { newPassword: "must differ" });
        }
        if (Array.from(newPassword).length < 12 || Buffer.byteLength(newPassword, "utf8") > 72) {
          return problem(422, "validation_failed", { newPassword: "invalid length" });
        }
        viewer.password = newPassword;
        revokeUserSessions(viewer.id, session.id);
        return send({ status: "password_changed" });
      }

      if (path === "me/account" && method === "DELETE") {
        if (String(body.password ?? "") !== viewer.password) {
          return problem(422, "validation_failed", { password: "invalid" });
        }
        viewer.deleted = true;
        viewer.firstName = "Видалений";
        viewer.lastName = "користувач";
        viewer.bio = "";
        viewer.city = "";
        viewer.teachingSkills = [];
        viewer.learningSkills = [];
        viewer.privacy.isDiscoverable = false;
        viewer.privacy.acceptsRequests = false;
        for (const list of messages.values()) {
          for (const msg of list) {
            if (msg.authorId === viewer.id) {
              msg.body = "Повідомлення видалено";
            }
          }
        }
        cancelRelationshipsForUser(viewer.id, "Акаунт учасника видалено");
        revokeUserSessions(viewer.id);
        await context.clearCookies({ name: "skillswap_session" });
        return route.fulfill({
          status: 204,
          headers: {
            "cache-control": "no-store",
            "set-cookie": "skillswap_session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax",
          },
        });
      }

      if (path === "me/blocks" && method === "GET") {
        const items = blocks
          .filter((b) => b.blockerId === viewer.id)
          .map((b) => ({ user: personView(users.get(b.blockedId)), createdAt: b.createdAt }));
        return send(paginate(items, url, 20));
      }

      if (path === "reports" && method === "POST") {
        const target = users.get(body.reportedUserId);
        if (!target || target.id === viewer.id || target.deleted) {
          return problem(404, "student_not_found");
        }
        const alreadyOpen = [...reports.values()].some(
          (r) =>
            r.reporterId === viewer.id && r.reportedUserId === target.id && r.status === "OPEN",
        );
        if (alreadyOpen) return problem(409, "report_already_open");
        const reason = String(body.reason ?? "").trim();
        if (!reason || Array.from(reason).length > 1000) {
          return problem(422, "validation_failed", { reason: "invalid" });
        }
        const id = makeUuid("4a000000", nextSeq());
        const rep = {
          id,
          reporterId: viewer.id,
          reportedUserId: target.id,
          reason,
          status: "OPEN",
          createdAt: nowIso(),
          resolvedAt: null,
          decision: null,
        };
        reports.set(id, rep);
        return send(
          {
            report: {
              id: rep.id,
              reportedUser: personView(target),
              reason: rep.reason,
              status: rep.status,
              createdAt: rep.createdAt,
              resolvedAt: null,
            },
          },
          201,
        );
      }

      if (path === "me/reports" && method === "GET") {
        const items = [...reports.values()]
          .filter((r) => r.reporterId === viewer.id)
          .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
          .map((r) => ({
            id: r.id,
            reportedUser: personView(users.get(r.reportedUserId)),
            reason: r.reason,
            status: r.status,
            createdAt: r.createdAt,
            resolvedAt: r.resolvedAt,
          }));
        return send(paginate(items, url, 20));
      }

      if (path.startsWith("admin/")) {
        if (viewer.role !== "ADMIN") return problem(403, "admin_required");

        if (path === "admin/reports" && method === "GET") {
          const status = url.searchParams.get("status") ?? "";
          const items = [...reports.values()]
            .filter((r) => !status || r.status === status)
            .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
            .map(adminReportView);
          return send(paginate(items, url, 20));
        }

        const adminResolveMatch = /^admin\/reports\/([^/]+)\/resolve$/.exec(path);
        if (adminResolveMatch && method === "POST") {
          const rep = reports.get(decodeURIComponent(adminResolveMatch[1]));
          if (!rep) return problem(404, "report_not_found");
          if (rep.status !== "OPEN") return problem(409, "report_already_resolved");
          const decision = String(body.decision ?? "");
          const reason = String(body.reason ?? "").trim();
          const reported = users.get(rep.reportedUserId);
          if (decision === "SUSPEND" && reported.id === viewer.id) {
            return problem(409, "self_moderation");
          }
          const stamp = nowIso();
          rep.status = decision === "DISMISS" ? "DISMISSED" : "RESOLVED";
          rep.resolvedAt = stamp;
          if (decision === "SUSPEND") {
            reported.accountStatus = "SUSPENDED";
            revokeUserSessions(reported.id);
            cancelRelationshipsForUser(reported.id, "Акаунт учасника призупинено модератором");
          }
          const modAction = {
            id: makeUuid("4b000000", nextSeq()),
            reportId: rep.id,
            moderator: personView(viewer),
            subject: personView(reported),
            skill: null,
            action:
              decision === "DISMISS"
                ? "DISMISS_REPORT"
                : decision === "RESOLVE"
                  ? "RESOLVE_REPORT"
                  : "SUSPEND_ACCOUNT",
            reason,
            createdAt: stamp,
          };
          moderationActions.unshift(modAction);
          rep.decision = modAction;
          return send({ report: adminReportView(rep) });
        }

        const adminReportDetailMatch = /^admin\/reports\/([^/]+)$/.exec(path);
        if (adminReportDetailMatch && method === "GET") {
          const rep = reports.get(decodeURIComponent(adminReportDetailMatch[1]));
          if (!rep) return problem(404, "report_not_found");
          const subjectHistory = moderationActions.filter(
            (a) => a.reportId === rep.id || a.subject?.id === rep.reportedUserId,
          );
          return send({ report: adminReportView(rep), history: subjectHistory });
        }

        const adminUserModMatch = /^admin\/users\/([^/]+)\/(suspend|restore)$/.exec(path);
        if (adminUserModMatch && method === "POST") {
          const subject = users.get(decodeURIComponent(adminUserModMatch[1]));
          const modKind = adminUserModMatch[2];
          if (!subject || subject.deleted) return problem(404, "user_not_found");
          if (subject.id === viewer.id) return problem(409, "self_moderation");
          const reason = String(body.reason ?? "").trim();
          subject.accountStatus = modKind === "suspend" ? "SUSPENDED" : "ACTIVE";
          if (modKind === "suspend") {
            revokeUserSessions(subject.id);
            cancelRelationshipsForUser(subject.id, "Акаунт учасника призупинено модератором");
          }
          const modAction = {
            id: makeUuid("4b000000", nextSeq()),
            reportId: null,
            moderator: personView(viewer),
            subject: personView(subject),
            skill: null,
            action: modKind === "suspend" ? "SUSPEND_ACCOUNT" : "RESTORE_ACCOUNT",
            reason,
            createdAt: nowIso(),
          };
          moderationActions.unshift(modAction);
          return send({
            user: { ...personView(subject), accountStatus: subject.accountStatus },
            action: modAction,
          });
        }

        if (path === "admin/moderation-actions" && method === "GET") {
          const userId = url.searchParams.get("userId") ?? "";
          const items = userId
            ? moderationActions.filter((a) => a.subject?.id === userId)
            : moderationActions;
          return send(paginate(items, url, 20));
        }

        if (path === "admin/skills" && method === "GET") {
          const q = (url.searchParams.get("q") ?? "").trim().toLocaleLowerCase("uk");
          const items = skills.filter(
            (s) =>
              !q ||
              s.name.toLocaleLowerCase("uk").includes(q) ||
              s.slug.toLocaleLowerCase("uk").includes(q),
          );
          return send(paginate(items, url, 50));
        }

        if (path === "admin/skills" && method === "POST") {
          const cat = categories.find((c) => c.id === body.categoryId);
          if (!cat) return problem(422, "validation_failed", { categoryId: "invalid" });
          if (
            skills.some(
              (s) =>
                s.slug === body.slug ||
                (s.categoryId === cat.id &&
                  s.name.toLocaleLowerCase("uk") === String(body.name).toLocaleLowerCase("uk")),
            )
          ) {
            return problem(409, "skill_already_exists");
          }
          const created = {
            id: makeUuid("30000000", 100 + nextSeq()),
            categoryId: cat.id,
            categoryName: cat.name,
            name: String(body.name).trim(),
            slug: String(body.slug).trim(),
            description: String(body.description ?? "").trim(),
            isActive: Boolean(body.isActive),
          };
          skills.push(created);
          moderationActions.unshift({
            id: makeUuid("4b000000", nextSeq()),
            reportId: null,
            moderator: personView(viewer),
            subject: null,
            skill: { id: created.id, name: created.name },
            action: "CREATE_SKILL",
            reason: `Створено навичку «${created.name}»`,
            createdAt: nowIso(),
          });
          return send({ skill: created }, 201);
        }

        const adminSkillPatchMatch = /^admin\/skills\/([^/]+)$/.exec(path);
        if (adminSkillPatchMatch && method === "PATCH") {
          const existing = findSkill(decodeURIComponent(adminSkillPatchMatch[1]));
          if (!existing) return problem(404, "skill_not_found");
          const cat = categories.find((c) => c.id === body.categoryId) ?? categories[0];
          existing.categoryId = cat.id;
          existing.categoryName = cat.name;
          existing.name = String(body.name).trim();
          existing.slug = String(body.slug).trim();
          existing.description = String(body.description ?? "").trim();
          existing.isActive = Boolean(body.isActive);
          moderationActions.unshift({
            id: makeUuid("4b000000", nextSeq()),
            reportId: null,
            moderator: personView(viewer),
            subject: null,
            skill: { id: existing.id, name: existing.name },
            action: "UPDATE_SKILL",
            reason: `Оновлено навичку «${existing.name}»`,
            createdAt: nowIso(),
          });
          return send({ skill: existing });
        }
      }

      return problem(404, "not_found");
    });
  }

  return {
    config,
    universities,
    categories,
    skills,
    users,
    sessions,
    mailbox,
    requests,
    exchanges,
    lessons,
    conversations,
    messages,
    notifications,
    reviews,
    blocks,
    reports,
    moderationActions,
    requestLog,
    nowMs,
    nowIso,
    advanceMinutes(minutes) {
      clockOffsetMs += minutes * 60_000;
    },
    futureLocal(minutesFromNow) {
      return kyivLocalInput(nowMs() + minutesFromNow * 60_000);
    },
    seedUser,
    signIn,
    rotateCsrf,
    addFault,
    attach,
    issueMailToken,
  };
}
