import { redirect } from "next/navigation";

export default async function VerifyEmailPage({ searchParams }: PageProps<"/verify-email">) {
  const { token } = await searchParams;
  const destination =
    typeof token === "string" && token
      ? `/onboarding?token=${encodeURIComponent(token)}`
      : "/onboarding";
  redirect(destination);
}
