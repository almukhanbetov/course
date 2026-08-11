import type { Metadata } from "next";
import "./globals.css";
import { NavBar } from "@/components/NavBar";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { getUnreadNotificationCount } from "@/lib/api";

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
    <html lang="ru">
      <body>
        <NavBar user={user} unreadCount={unreadCount} />
        {children}
      </body>
    </html>
  );
}
