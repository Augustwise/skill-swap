import { authRequest } from "./AuthApi";
import { readPage } from "./DiscoveryApi";
import type { SchedulePage } from "./SessionApi";

export const scheduleStatuses = ["SCHEDULED", "PROPOSED", "COMPLETED", "CANCELLED", "ALL"] as const;
export type ScheduleFilters = {
  status: (typeof scheduleStatuses)[number];
  upcoming: boolean;
  page: number;
};
export const scheduleLabels: Record<ScheduleFilters["status"], string> = {
  SCHEDULED: "Погоджені",
  PROPOSED: "Очікують погодження",
  COMPLETED: "Проведені",
  CANCELLED: "Скасовані",
  ALL: "Усі",
};

export function readScheduleFilters(params: { get(name: string): string | null }): ScheduleFilters {
  const status = scheduleStatuses.find((value) => value === params.get("status")) ?? "SCHEDULED";
  return {
    status,
    upcoming:
      params.get("upcoming") === null
        ? ["SCHEDULED", "PROPOSED"].includes(status)
        : params.get("upcoming") !== "false",
    page: readPage(params.get("page")),
  };
}

export function scheduleQuery(filters: ScheduleFilters) {
  const query = new URLSearchParams({ status: filters.status, upcoming: String(filters.upcoming) });
  if (filters.page > 1) query.set("page", String(filters.page));
  return query.toString();
}

export function scheduleHref(filters: ScheduleFilters) {
  return `/schedule?${scheduleQuery(filters)}`;
}

export function listSchedule(filters: ScheduleFilters, signal?: AbortSignal) {
  return authRequest<SchedulePage>(`me/schedule?${scheduleQuery(filters)}`, { signal });
}
