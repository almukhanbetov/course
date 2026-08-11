"use client";

import { useActionState } from "react";
import { registerAction, type FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function RegisterForm() {
  const [state, formAction, pending] = useActionState(registerAction, initialState);

  return (
    <form action={formAction} className="auth-form">
      <label>
        Имя
        <input type="text" name="first_name" required autoComplete="given-name" />
      </label>
      <label>
        Фамилия
        <input type="text" name="last_name" required autoComplete="family-name" />
      </label>
      <label>
        Email
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label>
        Пароль
        <input
          type="password"
          name="password"
          required
          minLength={8}
          autoComplete="new-password"
        />
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" disabled={pending}>
        {pending ? "Регистрация..." : "Зарегистрироваться"}
      </button>
    </form>
  );
}
