"use client";

import Link from "next/link";
import { useId, useLayoutEffect, useRef, useState } from "react";
import { loginHref } from "./AppRoutes";
import { AuthError } from "./AuthApi";
import { chatErrorMessage, MAX_CHAT_MESSAGE_LENGTH, messageError, messageLength } from "./ChatApi";
import { DashboardIcon as Icon } from "./DashboardIcon";

const counterFrom = MAX_CHAT_MESSAGE_LENGTH - 200;

export function ChatComposer({
  partnerName,
  path,
  onSend,
}: {
  partnerName: string;
  path: string;
  onSend: (body: string) => Promise<void>;
}) {
  const [text, setText] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const field = useRef<HTMLTextAreaElement>(null);
  const busy = useRef(false);
  const id = useId();
  const length = messageLength(text);
  const tooLong = length > MAX_CHAT_MESSAGE_LENGTH;

  useLayoutEffect(() => {
    const element = field.current;
    if (!element) return;
    element.style.height = "auto";
    element.style.height = `${element.scrollHeight + 2}px`;
  }, [text]);

  async function submit() {
    if (busy.current || messageError(text)) return;
    busy.current = true;
    setPending(true);
    setError(null);
    try {
      await onSend(text);
      setText("");
    } catch (problem) {
      setError(problem);
    } finally {
      busy.current = false;
      setPending(false);
      field.current?.focus();
    }
  }

  return (
    <form
      className="chat-composer"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <label className="sr-only" htmlFor={`${id}-message`}>
        Нове повідомлення, отримувач: {partnerName}
      </label>
      <div className="chat-composer__row">
        <textarea
          id={`${id}-message`}
          ref={field}
          rows={1}
          value={text}
          readOnly={pending}
          placeholder="Напиши повідомлення…"
          aria-invalid={tooLong || undefined}
          aria-describedby={`${id}-hint`}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              void submit();
            }
          }}
        />
        <button
          className="dashboard-button dashboard-button--primary chat-composer__send"
          type="submit"
          disabled={pending || length === 0 || tooLong}
          aria-label={pending ? "Надсилаємо…" : "Надіслати"}
        >
          <Icon name="send" />
          <span>{pending ? "Надсилаємо…" : "Надіслати"}</span>
        </button>
      </div>
      <div className="chat-composer__hint" id={`${id}-hint`}>
        {error ? (
          <p className="exchange-error" role="alert">
            {chatErrorMessage(error)}
            {error instanceof AuthError && error.status === 401 && (
              <>
                {" "}
                <Link href={loginHref(path)}>Увійти знову</Link>
              </>
            )}
          </p>
        ) : tooLong ? (
          <p className="exchange-error">{messageError(text)}</p>
        ) : (
          <p>Enter — надіслати, Shift + Enter — новий рядок. Посилання стануть клікабельними.</p>
        )}
        {length >= counterFrom && (
          <span className={`chat-composer__count ${tooLong ? "chat-composer__count--over" : ""}`}>
            {length} / {MAX_CHAT_MESSAGE_LENGTH}
          </span>
        )}
      </div>
    </form>
  );
}
