import Link from "next/link";
import type { Metadata } from "next";
import { LoginForm } from "@/components/LoginForm";

export const metadata: Metadata = {
  title: "Вход — LMS Platform",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ registered?: string }>;
}) {
  const { registered } = await searchParams;

  return (
    <main className="auth-page">
      <div className="auth-card">
        <h1>Вход</h1>
        <p className="subtitle" style={{ marginBottom: 0 }}>
          Рады видеть вас снова
        </p>
        {registered && <p className="success">Регистрация прошла успешно. Теперь войдите.</p>}
        <LoginForm />
        <p className="subtitle" style={{ marginTop: "1.5rem", marginBottom: 0 }}>
          Нет аккаунта? <Link href="/register">Зарегистрироваться</Link>
        </p>
      </div>
    </main>
  );
}
