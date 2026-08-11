import Link from "next/link";
import type { Metadata } from "next";
import { RegisterForm } from "@/components/RegisterForm";

export const metadata: Metadata = {
  title: "Регистрация — LMS Platform",
};

export default function RegisterPage() {
  return (
    <main>
      <h1>Регистрация</h1>
      <RegisterForm />
      <p className="subtitle">
        Уже есть аккаунт? <Link href="/login">Войти</Link>
      </p>
    </main>
  );
}
