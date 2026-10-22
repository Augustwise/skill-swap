import type { Metadata } from "next";
import { Profile } from "../components/Profile";
import { readPage } from "../components/DiscoveryApi";
import "../styles/dashboard.css";
import "../styles/profile.css";

export const metadata: Metadata = {
  title: "Мій профіль — SkillSwap",
  description: "Твої особисті дані, навички та формати навчання.",
};

export default async function ProfilePage({ searchParams }: PageProps<"/profile">) {
  const { tab, reviewsPage } = await searchParams;
  return (
    <Profile
      section={tab === "skills" || tab === "reviews" ? tab : "about"}
      reviewsPage={readPage(typeof reviewsPage === "string" ? reviewsPage : null)}
    />
  );
}
