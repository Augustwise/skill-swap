"use client";

import { useEffect, useId, useRef } from "react";

export type DashboardModalContent = { title: string; description?: string };
export type OpenDashboardModal = (content: DashboardModalContent) => void;

export function DashboardModal({
  title,
  description = "Ця можливість ще в розробці й буде доступна згодом.",
  onClose,
}: DashboardModalContent & { onClose: () => void }) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const backdropPressed = useRef(false);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    const previousOverflow = document.body.style.overflow;
    dialog.showModal();
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
      className="dashboard-modal"
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onClose={onClose}
      onPointerDown={(event) => {
        backdropPressed.current = isOutside(event);
      }}
      onClick={(event) => {
        if (backdropPressed.current && isOutside(event)) dialogRef.current?.close();
        backdropPressed.current = false;
      }}
    >
      <div className="dashboard-modal__content">
        <form method="dialog">
          <button className="dashboard-modal__close" type="submit" aria-label="Закрити вікно">
            <span aria-hidden="true">×</span>
          </button>
        </form>
        <span className="dashboard-modal__badge">Незабаром</span>
        <h2 id={titleId}>{title}</h2>
        <p id={descriptionId}>{description}</p>
        <form method="dialog" className="dashboard-modal__actions">
          <button className="dashboard-button dashboard-button--primary" type="submit" autoFocus>
            Зрозуміло
          </button>
        </form>
      </div>
    </dialog>
  );
}
