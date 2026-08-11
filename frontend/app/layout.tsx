import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { NavBar } from "@/components/NavBar";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { getUnreadNotificationCount } from "@/lib/api";

const inter = Inter({ subsets: ["latin", "cyrillic"], variable: "--font-sans", display: "swap" });

export const metadata: Metadata = {
  title: "LMS Platform",
  description: "Учебная платформа: специальности, курсы, модули, уроки",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const user = await getCurrentUser();
  const token = await getSessionToken();
  const unreadCount = user && token ? await getUnreadNotificationCount(token) : 0;

  return (
    <html lang="ru" className={inter.variable}>
      <body>
        <div className="page-shell">
          <NavBar user={user} unreadCount={unreadCount} />
          {children}
        </div>
      </body>
    </html>
  );
}
