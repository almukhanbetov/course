"use client";

import { useActionState } from "react";
import { loginAction, type FormState } from "@/lib/actions";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

const initialState: FormState = { error: null };

export function LoginForm({ locale }: { locale: Locale }) {
  const [state, formAction, pending] = useActionState(loginAction, initialState);
  const dict = getDictionary(locale).auth;

  return (
    <form action={formAction} className="auth-form">
      <label>
        {dict.emailLabel}
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label>
        {dict.passwordLabel}
        <input type="password" name="password" required autoComplete="current-password" />
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" disabled={pending}>
        {pending ? dict.loginSubmitting : dict.loginSubmit}
      </button>
    </form>
  );
}
