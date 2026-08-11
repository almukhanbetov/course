import Link from "next/link";
import type { Metadata } from "next";
import { RegisterForm } from "@/components/RegisterForm";

export const metadata: Metadata = {
  title: "Регистрация — LMS Platform",
};

export default function RegisterPage() {
  return (
    <main className="auth-page">
      <div className="auth-card">
        <h1>Регистрация</h1>
        <p className="subtitle" style={{ marginBottom: 0 }}>
          Создайте аккаунт и начните обучение
        </p>
        <RegisterForm />
        <p className="subtitle" style={{ marginTop: "1.5rem", marginBottom: 0 }}>
          Уже есть аккаунт? <Link href="/login">Войти</Link>
        </p>
      </div>
    </main>
  );
}
