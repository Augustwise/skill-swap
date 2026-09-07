import type { Metadata } from "next";
import { Caveat, Geist } from "next/font/google";
import "./globals.css";

const geist = Geist({ variable: "--font-geist-sans", subsets: ["latin", "cyrillic"] });
const caveat = Caveat({
  variable: "--font-caveat",
  subsets: ["latin", "cyrillic", "cyrillic-ext"],
  weight: ["500", "600", "700"],
});

export const metadata: Metadata = {
  title: "SkillSwap — обмін навичками між студентами",
  description: "Знаходь студентів для взаємного обміну знаннями та навичками.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="uk" className={`${geist.variable} ${caveat.variable}`}>
      <body>{children}</body>
    </html>
  );
}
