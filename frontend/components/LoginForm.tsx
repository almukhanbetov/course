"use client";

import { useActionState } from "react";
import { loginAction, type FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function LoginForm() {
  const [state, formAction, pending] = useActionState(loginAction, initialState);

  return (
    <form action={formAction} className="auth-form">
      <label>
        Email
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label>
        Пароль
        <input type="password" name="password" required autoComplete="current-password" />
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" disabled={pending}>
        {pending ? "Вход..." : "Войти"}
      </button>
    </form>
  );
}
