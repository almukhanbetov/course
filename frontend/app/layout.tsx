import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { NavBar } from "@/components/NavBar";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { getUnreadNotificationCount } from "@/lib/api";
import { getLocale } from "@/lib/i18n/getLocale";

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
  const locale = await getLocale();

  return (
    <html lang={locale} className={inter.variable} suppressHydrationWarning>
      <body>
        {/* Sets data-theme on <html> before the rest of the body paints, so
            a light-theme visitor never sees a flash of the dark theme.
            Kept as a tiny inline script (not a "use client" component)
            specifically so it runs synchronously, ahead of hydration —
            see components/ThemeToggle.tsx for the interactive half. */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("theme");if(t!=="light"&&t!=="dark"){t=window.matchMedia("(prefers-color-scheme: light)").matches?"light":"dark"}document.documentElement.setAttribute("data-theme",t)}catch(e){}})();`,
          }}
        />
        <div className="page-shell">
          <NavBar user={user} unreadCount={unreadCount} locale={locale} />
          {children}
        </div>
      </body>
    </html>
  );
}
