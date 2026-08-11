"use client";

import { useActionState } from "react";
import { updateUserAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { PublicUser } from "@/lib/api";

const initialState: FormState = { error: null };

export function UserEditForm({ user }: { user: PublicUser }) {
  const action = updateUserAction.bind(null, user.id);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Email
        <input type="text" value={user.email} disabled />
      </label>
      <label>
        First name
        <input type="text" name="first_name" defaultValue={user.first_name} required />
      </label>
      <label>
        Last name
        <input type="text" name="last_name" defaultValue={user.last_name} required />
      </label>
      <label>
        Role
        <select name="role" defaultValue={user.role}>
          <option value="student">student</option>
          <option value="instructor">instructor</option>
          <option value="admin">admin</option>
        </select>
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="active" defaultChecked={user.active} />
        Active
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
