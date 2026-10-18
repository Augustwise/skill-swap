"use client";

import { Fragment, useEffect, useLayoutEffect, useRef } from "react";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { messageParts, type ChatMessage } from "./ChatApi";
import { formatTime, groupByDay } from "./ChatFormat";
import { fullName } from "./DiscoveryFormat";

const nearBottom = 80;

function MessageBody({ body }: { body: string }) {
  return (
    <p className="chat-message__body">
      {messageParts(body).map((part, index) =>
        part.href ? (
          <a href={part.href} target="_blank" rel="noopener noreferrer nofollow" key={index}>
            {part.text}
          </a>
        ) : (
          <Fragment key={index}>{part.text}</Fragment>
        ),
      )}
    </p>
  );
}

function MessageItem({ message, own }: { message: ChatMessage; own: boolean }) {
  if (!message.author) {
    return (
      <li className="chat-message chat-message--system">
        <MessageBody body={message.body} />
      </li>
    );
  }
  return (
    <li className={`chat-message ${own ? "chat-message--own" : ""}`}>
      <span className="sr-only">{own ? "Ти" : fullName(message.author)}: </span>
      <MessageBody body={message.body} />
      <span className="chat-message__meta">
        <time dateTime={message.createdAt}>{formatTime(message.createdAt)}</time>
        {message.editedAt && <span>· змінено</span>}
        {own && (
          <span className="chat-message__status" title={message.readAt ? "Прочитано" : "Надіслано"}>
            <Icon name={message.readAt ? "read" : "sent"} />
            <span className="sr-only">{message.readAt ? "Прочитано" : "Надіслано"}</span>
          </span>
        )}
      </span>
    </li>
  );
}

export function ChatThread({
  messages,
  viewerId,
  partnerName,
  hasEarlier,
  earlier,
  onLoadEarlier,
}: {
  messages: ChatMessage[];
  viewerId: string;
  partnerName: string;
  hasEarlier: boolean;
  earlier: "idle" | "loading" | "error";
  onLoadEarlier: () => void;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const atBottom = useRef(true);
  const firstId = messages[0]?.id;
  const last = messages.at(-1);
  const previous = useRef({ firstId: "", lastId: "", height: 0 });

  useLayoutEffect(() => {
    const element = scroller.current;
    if (!element) return;
    const before = previous.current;
    if (firstId && firstId !== before.firstId && last?.id === before.lastId) {
      element.scrollTop += element.scrollHeight - before.height;
    } else if (
      last &&
      last.id !== before.lastId &&
      (atBottom.current || last.author?.id === viewerId)
    ) {
      element.scrollTop = element.scrollHeight;
    }
    previous.current = {
      firstId: firstId ?? "",
      lastId: last?.id ?? "",
      height: element.scrollHeight,
    };
  }, [firstId, last, viewerId]);

  useEffect(() => {
    const element = scroller.current;
    if (!element) return;
    const observer = new ResizeObserver(() => {
      if (atBottom.current) element.scrollTop = element.scrollHeight;
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      className="chat-thread"
      ref={scroller}
      role="log"
      aria-label={`Чат: ${partnerName}`}
      tabIndex={0}
      onScroll={(event) => {
        const element = event.currentTarget;
        atBottom.current =
          element.scrollHeight - element.scrollTop - element.clientHeight < nearBottom;
      }}
    >
      {hasEarlier && (
        <div className="chat-thread__earlier">
          <button
            className="dashboard-button"
            type="button"
            disabled={earlier === "loading"}
            onClick={onLoadEarlier}
          >
            {earlier === "loading" ? "Завантажуємо…" : "Показати попередні повідомлення"}
          </button>
          {earlier === "error" && (
            <span className="exchange-error" role="alert">
              Не вдалося завантажити попередні повідомлення. Спробуй ще раз.
            </span>
          )}
        </div>
      )}
      {messages.length === 0 ? (
        <div className="chat-thread__empty">
          <span className="dashboard-empty__icon" aria-hidden="true">
            <Icon name="nav-chat" />
          </span>
          <p>
            Повідомлень ще немає. Привітайся й обговоріть деталі обміну: зручний час, місце чи
            посилання на зустріч.
          </p>
        </div>
      ) : (
        groupByDay(messages).map((group) => (
          <section className="chat-day" aria-label={group.label} key={group.messages[0].id}>
            <h3 className="chat-day__label">{group.label}</h3>
            <ol className="chat-messages">
              {group.messages.map((message) => (
                <MessageItem
                  message={message}
                  own={message.author?.id === viewerId}
                  key={message.id}
                />
              ))}
            </ol>
          </section>
        ))
      )}
    </div>
  );
}
