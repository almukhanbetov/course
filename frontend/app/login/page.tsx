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
    <main>
      <h1>Вход</h1>
      {registered && <p className="success">Регистрация прошла успешно. Теперь войдите.</p>}
      <LoginForm />
      <p className="subtitle">
        Нет аккаунта? <Link href="/register">Зарегистрироваться</Link>
      </p>
    </main>
  );
}
