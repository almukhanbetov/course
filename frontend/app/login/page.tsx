import Link from "next/link";
import type { Metadata } from "next";
import { LoginForm } from "@/components/LoginForm";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

export const metadata: Metadata = {
  title: "Вход — LMS Platform",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ registered?: string }>;
}) {
  const { registered } = await searchParams;
  const locale = await getLocale();
  const dict = getDictionary(locale).auth;

  return (
    <main className="auth-page">
      <div className="auth-card">
        <h1>{dict.loginTitle}</h1>
        <p className="subtitle" style={{ marginBottom: 0 }}>
          {dict.loginSubtitle}
        </p>
        {registered && <p className="success">{dict.registeredSuccess}</p>}
        <LoginForm locale={locale} />
        <p className="subtitle" style={{ marginTop: "1.5rem", marginBottom: 0 }}>
          {dict.noAccountPrefix}
          <Link href="/register">{dict.registerLink}</Link>
        </p>
      </div>
    </main>
  );
}
