import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";

const geist = Geist({ variable: "--font-geist-sans", subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  title: "SkillSwap — обмін навичками між студентами",
  description: "Знаходь студентів для взаємного обміну знаннями та навичками.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="uk" className={geist.variable}>
      <body>{children}</body>
    </html>
  );
}
