# Sprint 6: private chat and in-app notifications (FR-08, FR-13)

Laboratory 1, section 7.2 assigns the private text chat and notifications about the main
events to sprint 6. The Go endpoints were merged earlier (`feat/sprint6-chat-notifications`);
this branch, `feat/sprint6-frontend-chat-notifications`, adds the web client for them.

## Pages

| Route                         | Purpose                                                                  | API                                                        |
| ----------------------------- | ------------------------------------------------------------------------ | ---------------------------------------------------------- |
| `/messages`                   | Your conversations, newest activity first, with unread counters          | `GET /me/conversations`                                    |
| `/messages/[conversationId]`  | The chat: history, new messages every 5 seconds, read marks, sending     | `GET /conversations/{id}`, `…/messages`, `POST …/messages` |
| `/notifications`              | All or unread notifications, open one, mark one or all as read           | `GET /me/notifications`, `POST …/read`, `POST …/read-all`  |
| Top bar on every private page | Bell with the unread count and a panel with the latest six               | `GET /me/notifications`                                    |
| `/requests/[requestId]`       | "Чат" card: the last message, unread count and a link to the chat        | `GET /exchange-requests/{id}/conversation`                 |
| `/exchanges/[exchangeId]`     | The same card for the exchange; the request chat moves here on accept    | `GET /exchanges/{id}/conversation`                         |
| Sidebar and account menu      | "Повідомлення" opens `/messages` and shows the number of unread messages | `GET /me/conversations`                                    |

All pages need a session with a verified email. `/notifications?filter=unread&page=2` keeps
the filter and page in the URL. On screens up to 800 px wide `/messages` shows only the
list and a chat only the chat, with a "← До розмов" link.

## Architecture

Pages call only the Go HTTP API through the `/api/v1/*` rewrite. CommunicationService rules
(membership in `conversation_participants`, blocks, 1–2000 characters, idempotent
notifications) stay on the server; the client explains them earlier and renders the DTOs.

| Module                   | Responsibility                                                                     |
| ------------------------ | ---------------------------------------------------------------------------------- |
| `ChatApi.ts`             | Chat DTOs, message validation, merging and polling cursor, links, requests, errors |
| `ChatFormat.ts`          | Message times, "Сьогодні" / "Учора" day labels, list times, grouping by day        |
| `NotificationApi.ts`     | Notification DTOs, URL filters, the page each notification opens, requests         |
| `useChat.ts`             | One open chat: newest page, 5-second polling, read marks, sending, earlier pages   |
| `usePolling.ts`          | Repeats a request while the tab is visible and again when it is shown              |
| `Messages.tsx`           | `/messages` layout: conversation list and the open chat                            |
| `ConversationList.tsx`   | Conversations with previews and unread counters, "Показати ще"                     |
| `ChatPanel.tsx`          | Chat header (partner, skills, request or exchange), connection banner, states      |
| `ChatThread.tsx`         | Messages grouped by day, own and partner bubbles, read marks, scrolling            |
| `ChatComposer.tsx`       | Growing text field, Enter to send, character counter, send errors                  |
| `ChatCard.tsx`           | The chat card on request and exchange pages                                        |
| `NotificationBell.tsx`   | Bell, unread count and the panel with the latest notifications                     |
| `NotificationCards.tsx`  | Notification row, skeleton and the shared open / mark-read actions                 |
| `Notifications.tsx`      | `/notifications`: filter tabs, list, "Позначити всі прочитаними", pagination       |
| `AppShell.tsx` (updated) | Counters for requests, messages and notifications; `useCountersChanged`            |
| `AuthApi.ts` (updated)   | `writeRequest`: a POST with CSRF that refreshes a stale token once                 |

### Polling (FR-08, LR2 3.3)

The chat opens with `GET …/messages?page=1` to learn the total, then loads the last page
(and the one before it when the last page has fewer than ten messages). Every 5 seconds
(`pollIntervalSeconds` from the response) it asks for `GET …/messages?after={id}`, so only
new messages travel. When some of your recent messages are still unread, the cursor starts
just before the earliest of them, so their read mark refreshes in the same request. If the
server returns more than one page of new messages, the client continues from the last one.

| Situation                       | Client behaviour                                                                 |
| ------------------------------- | -------------------------------------------------------------------------------- |
| New message from the partner    | Appears within 5 seconds; the chat scrolls down only if you were at the bottom   |
| The chat is open and visible    | `POST …/read` marks the incoming messages and their notifications as read        |
| The tab is hidden               | Polling pauses and resumes immediately when the tab is shown again               |
| No connection, 5xx              | "Немає з’єднання із сервером…" banner; polling continues and hides it on success |
| 401 / 403 / 404                 | Polling stops; login link or "Розмову не знайдено."                              |
| 409 `conversation_blocked`      | The field is replaced by an explanation; history stays readable                  |
| 403 `csrf_invalid` when sending | The token is refreshed from `/auth/me` and the message is sent once more         |

The navigation counters (incoming requests, unread messages, unread notifications) are
refreshed every 30 seconds while the tab is visible and right after an action that changes
them: a request decision, a sent or read message, a read notification.

## Design

The Figma file `Fer6NQRC3RZCPHPhqNF8QM` has only the overview and component pages. The
overview map lists "06 · Повідомлення — чат у контексті обміну + пропозиція заняття", but
the screen itself is not in the file, and there is no notifications screen ("08.3 ·
Профіль — Сповіщення" is the settings page of FR-22). The screens were designed in code
from the library components and the existing app style:

| Figma component           | Use                                                                     |
| ------------------------- | ----------------------------------------------------------------------- |
| `AppTopbar`, `Icon/bell`  | The bell button; a red counter like the topbar's notification dot       |
| `AppSidebar`, `Icon/chat` | "Повідомлення" becomes a link with the unread counter                   |
| `Icon/send` (`4:31`)      | The "Надіслати" button, recoloured white for the primary button         |
| `Icon/check` (`3:58`)     | "Позначити прочитаним", "Надіслано"; a double check for "Прочитано"     |
| `Avatar`, `SkillTag`      | Conversation rows and the chat header (what you learn ⇄ what you teach) |
| `StatusBadge`, `Tab`      | Request or exchange status in the chat header; "Усі" / "Непрочитані"    |
| `Button`, `Input`         | Primary send button, secondary context link, the message field          |

Own messages are purple bubbles on the right, the partner's are white on the left, with
day separators between them. The new icons are in `public/figma/chat/`.

## Decisions

- **Scope.** The chat belongs to a request and moves to the exchange when the request is
  accepted, with all messages. There is no "new chat" button: a conversation starts with a
  request. Lesson proposals from the chat are sprint 7 (FR-09); session notifications open
  the exchange page until the schedule exists.
- **Text and links only.** The body is rendered as plain text with line breaks; only
  `http(s)://` and `www.` addresses become links (`target="_blank"`, `rel="noopener
noreferrer nofollow"`). No HTML from a message is ever inserted.
- **Limits like the server.** The trimmed text must have 1–2000 characters, counted as
  Unicode characters. An empty text cannot be sent; a counter appears from 1800.
- **Enter sends.** Shift + Enter adds a line; the button and the field are locked while a
  message is being sent, so a double click does not send it twice.
- **Read means seen.** Incoming messages are marked read only while the chat is open in a
  visible tab. Opening a notification marks it read first, then opens its page;
  Ctrl/⌘-click opens it in a new tab without marking.
- **Unread messages counter** is the sum over the 20 most recently active conversations
  (the first page of `/me/conversations`); unread chats move to the top, so this covers
  real use without a separate endpoint.
- **No invented data.** Empty states explain where chats and notifications come from.

## Checks

After `go run ./cmd/db seed`, sign in as `demo.olha` and `demo.andrii` (password in
`backend/README.md`) in two browser profiles. The seed also gives Olha Taras's chat message
and two unread notifications. On a shared database, send Olha → Andrii a new request and
check the chat there instead of answering Taras's demo request.

| Case                                                         | Expected                                                                  |
| ------------------------------------------------------------ | ------------------------------------------------------------------------- |
| Bell on any page                                             | Unread count; the panel lists the latest notifications                    |
| Click "Новий запит на обмін" in the panel                    | The notification is read, the counter drops, the request page opens       |
| `/messages`                                                  | Conversations with previews, "Ти: …" for your last message, unread badges |
| Open a chat with 50+ messages                                | Newest messages, "Показати попередні повідомлення" keeps the scroll place |
| Send " Привіт www.example.com "                              | Trimmed message on the right, clickable link, field cleared               |
| The partner writes                                           | The message appears within 5 s; the chat and its notification become read |
| The partner reads                                            | Your check marks turn into "Прочитано" on the next poll                   |
| Stop the API, then start it                                  | The connection banner appears, then disappears with the missed messages   |
| Spaces only / 2001 characters                                | The button stays disabled; "Повідомлення — до 2000 символів." and counter |
| A blocked partner (409)                                      | The field is replaced by the explanation; history is still visible        |
| `/messages/<someone else's id>`, `/messages/nope`            | "Розмову не знайдено." with a link to all conversations                   |
| `/notifications?filter=unread` → "Позначити всі прочитаними" | "Непрочитаних сповіщень немає.", the bell shows no counter                |
| Widths 375, 768, 1440 px                                     | No horizontal scrolling; on phones the list and the chat take turns       |

Before every push, from `frontend/`: `npm test`, `npm run lint`, `npm run format:check`
and `npm run build`. `npm test` covers message limits, merging and the polling cursor, the
first page to open, link detection, conversation previews, list refresh, the API paths for
chats and notifications, the CSRF retry, Ukrainian errors, chat dates, notification filters
and targets, and the new return paths.
