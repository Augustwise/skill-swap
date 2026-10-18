import type { Metadata } from "next";
import { Suspense } from "react";
import { AppSkeleton } from "../components/Skeleton";
import { StudentSearch } from "../components/StudentSearch";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";

export const metadata: Metadata = {
  title: "Пошук навичок — SkillSwap",
  description: "Пошук студентів за навичкою або ім’ям з фільтрами категорії, рівня та формату.",
};

export default function SearchPage() {
  return (
    <Suspense fallback={<AppSkeleton page="search" />}>
      <StudentSearch />
    </Suspense>
  );
}
