"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";

// A native modal dialog: focus stays inside, Escape and a click outside close it.
export function AppDialog({
  title,
  description,
  badge,
  className = "",
  canClose = true,
  onClose,
  children,
}: {
  title: string;
  description?: ReactNode;
  badge?: string;
  className?: string;
  canClose?: boolean;
  onClose: () => void;
  children?: ReactNode;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const backdropPressed = useRef(false);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    const previousOverflow = document.body.style.overflow;
    dialog.showModal();
    dialog.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    document.body.style.overflow = "hidden";
    return () => {
      dialog.close();
      document.body.style.overflow = previousOverflow;
    };
  }, []);

  function isOutside(event: React.PointerEvent | React.MouseEvent) {
    const bounds = event.currentTarget.getBoundingClientRect();
    return (
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom
    );
  }

  return (
    <dialog
      ref={dialogRef}
      className={`dashboard-modal ${className}`.trim()}
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      onCancel={(event) => {
        if (!canClose) event.preventDefault();
      }}
      onClose={() => {
        // Strict Mode reopens the dialog after a test cleanup; that queued close is not real.
        if (!dialogRef.current?.open) onClose();
      }}
      onPointerDown={(event) => {
        backdropPressed.current = isOutside(event);
      }}
      onClick={(event) => {
        if (canClose && backdropPressed.current && isOutside(event)) dialogRef.current?.close();
        backdropPressed.current = false;
      }}
    >
      <div className="dashboard-modal__content">
        <button
          className="dashboard-modal__close"
          type="button"
          aria-label="Закрити вікно"
          disabled={!canClose}
          onClick={() => dialogRef.current?.close()}
        >
          <span aria-hidden="true">×</span>
        </button>
        {badge && <span className="dashboard-modal__badge">{badge}</span>}
        <h2 id={titleId}>{title}</h2>
        {description && (
          <p className="dashboard-modal__description" id={descriptionId}>
            {description}
          </p>
        )}
        {children}
      </div>
    </dialog>
  );
}

export function closeDialog(element: Element | null) {
  element?.closest("dialog")?.close();
}
