"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AuthError } from "./AuthApi";
import {
  isIncomingUnread,
  latestMessages,
  listMessages,
  markConversationRead,
  mergeMessages,
  newMessages,
  POLL_INTERVAL_SECONDS,
  sendMessage,
  type ChatMessage,
} from "./ChatApi";

function stopsPolling(error: unknown) {
  return error instanceof AuthError && [401, 403, 404].includes(error.status);
}

// Messages of one open chat: the newest page first, then new ones every few seconds.
// The parent renders it with key={conversationId}, so another chat starts from scratch.
export function useChat({
  conversationId,
  viewerId,
  csrfToken,
  onActivity,
}: {
  conversationId: string;
  viewerId: string;
  csrfToken: string;
  onActivity: () => void;
}) {
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [firstPage, setFirstPage] = useState(1);
  const [online, setOnline] = useState(true);
  const [earlier, setEarlier] = useState<"idle" | "loading" | "error">("idle");
  const [attempt, setAttempt] = useState(0);
  const list = useRef<ChatMessage[]>([]);
  const activity = useRef(onActivity);

  useEffect(() => {
    activity.current = onActivity;
  });

  const update = useCallback((next: ChatMessage[]) => {
    list.current = next;
    setMessages(next);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    let timer = 0;
    let interval = POLL_INTERVAL_SECONDS * 1000;
    let polling = false;
    let marking = false;
    let stopped = false;

    // Incoming messages count as read only while the student can see the chat.
    async function markRead() {
      if (marking || document.hidden) return;
      if (!list.current.some((message) => isIncomingUnread(message, viewerId))) return;
      marking = true;
      try {
        await markConversationRead(conversationId, csrfToken);
        if (stopped) return;
        const now = new Date().toISOString();
        update(
          list.current.map((message) =>
            isIncomingUnread(message, viewerId) ? { ...message, readAt: now } : message,
          ),
        );
        activity.current();
      } catch {
        // The next poll tries again.
      } finally {
        marking = false;
      }
    }

    function schedule() {
      window.clearTimeout(timer);
      if (!stopped) timer = window.setTimeout(poll, interval);
    }

    async function poll() {
      if (polling || stopped) return;
      if (document.hidden) return schedule();
      polling = true;
      try {
        const result = await newMessages(conversationId, list.current, viewerId, controller.signal);
        if (stopped) return;
        interval = result.pollIntervalSeconds * 1000;
        setOnline(true);
        const before = list.current.length;
        update(mergeMessages(list.current, result.items));
        if (list.current.length > before) activity.current();
        void markRead();
      } catch (problem) {
        if (stopped) return;
        if (stopsPolling(problem)) {
          stopped = true;
          setError(problem);
          setStatus("error");
          return;
        }
        setOnline(false);
      } finally {
        polling = false;
      }
      schedule();
    }

    function onVisibility() {
      if (!document.hidden) void poll();
    }

    latestMessages(conversationId, controller.signal).then(
      (result) => {
        if (stopped) return;
        interval = result.pollIntervalSeconds * 1000;
        update(result.items);
        setFirstPage(result.firstPage);
        setStatus("ready");
        document.addEventListener("visibilitychange", onVisibility);
        void markRead();
        schedule();
      },
      (problem: unknown) => {
        if (stopped) return;
        setError(problem);
        setStatus("error");
      },
    );

    return () => {
      stopped = true;
      controller.abort();
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [conversationId, viewerId, csrfToken, attempt, update]);

  const send = useCallback(
    async (body: string) => {
      const message = await sendMessage(conversationId, body, csrfToken);
      update(mergeMessages(list.current, [message]));
      setOnline(true);
      activity.current();
    },
    [conversationId, csrfToken, update],
  );

  const loadEarlier = useCallback(async () => {
    if (firstPage <= 1) return;
    setEarlier("loading");
    try {
      const page = await listMessages(conversationId, firstPage - 1);
      update(mergeMessages(list.current, page.items));
      setFirstPage(firstPage - 1);
      setEarlier("idle");
    } catch {
      setEarlier("error");
    }
  }, [conversationId, firstPage, update]);

  return {
    status,
    error,
    messages,
    online,
    hasEarlier: firstPage > 1,
    earlier,
    loadEarlier,
    send,
    retry: () => {
      setStatus("loading");
      setError(null);
      setAttempt((value) => value + 1);
    },
  };
}
