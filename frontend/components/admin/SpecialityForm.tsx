"use client";

import { useActionState } from "react";
import { createSpecialityAction, updateSpecialityAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Speciality } from "@/lib/api";

const initialState: FormState = { error: null };

export function SpecialityForm({ speciality }: { speciality?: Speciality }) {
  const action = speciality ? updateSpecialityAction.bind(null, speciality.id) : createSpecialityAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Title
        <input type="text" name="title" defaultValue={speciality?.title} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={speciality?.slug} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={4} defaultValue={speciality?.description} />
      </label>
      <label>
        Image URL
        <input type="text" name="image_url" defaultValue={speciality?.image_url} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={speciality?.published} />
        Published
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
