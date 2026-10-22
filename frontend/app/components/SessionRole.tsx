"use client";

import { createContext, useContext } from "react";

export const SessionRole = createContext<"STUDENT" | "ADMIN">("STUDENT");
export function useIsAdmin() {
  return useContext(SessionRole) === "ADMIN";
}
