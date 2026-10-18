"use client";

import { AppDialog, closeDialog } from "./AppDialog";

export type DashboardModalContent = { title: string; description?: string };
export type OpenDashboardModal = (content: DashboardModalContent) => void;

export function DashboardModal({
  title,
  description = "Ця можливість ще в розробці й буде доступна згодом.",
  onClose,
}: DashboardModalContent & { onClose: () => void }) {
  return (
    <AppDialog title={title} description={description} badge="Незабаром" onClose={onClose}>
      <div className="dashboard-modal__actions">
        <button
          className="dashboard-button dashboard-button--primary"
          type="button"
          data-autofocus
          onClick={(event) => closeDialog(event.currentTarget)}
        >
          Зрозуміло
        </button>
      </div>
    </AppDialog>
  );
}
