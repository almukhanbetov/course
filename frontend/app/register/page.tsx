import Link from "next/link";
import type { Metadata } from "next";
import { RegisterForm } from "@/components/RegisterForm";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

export const metadata: Metadata = {
  title: "Регистрация — LMS Platform",
};

export default async function RegisterPage() {
  const locale = await getLocale();
  const dict = getDictionary(locale).auth;

  return (
    <main className="auth-page">
      <div className="auth-card">
        <h1>{dict.registerTitle}</h1>
        <p className="subtitle" style={{ marginBottom: 0 }}>
          {dict.registerSubtitle}
        </p>
        <RegisterForm locale={locale} />
        <p className="subtitle" style={{ marginTop: "1.5rem", marginBottom: 0 }}>
          {dict.haveAccountPrefix}
          <Link href="/login">{dict.loginLink}</Link>
        </p>
      </div>
    </main>
  );
}
