"use client";

import { useActionState } from "react";
import { registerAction, type FormState } from "@/lib/actions";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

const initialState: FormState = { error: null };

export function RegisterForm({ locale }: { locale: Locale }) {
  const [state, formAction, pending] = useActionState(registerAction, initialState);
  const dict = getDictionary(locale).auth;

  return (
    <form action={formAction} className="auth-form">
      <label>
        {dict.firstNameLabel}
        <input type="text" name="first_name" required autoComplete="given-name" />
      </label>
      <label>
        {dict.lastNameLabel}
        <input type="text" name="last_name" required autoComplete="family-name" />
      </label>
      <label>
        {dict.emailLabel}
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label>
        {dict.passwordLabel}
        <input type="password" name="password" required minLength={8} autoComplete="new-password" />
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" disabled={pending}>
        {pending ? dict.registerSubmitting : dict.registerSubmit}
      </button>
    </form>
  );
}
