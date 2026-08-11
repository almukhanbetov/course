"use client";

import { useActionState } from "react";
import { createCategoryAction, updateCategoryAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Category } from "@/lib/api";

const initialState: FormState = { error: null };

export function CategoryForm({ category }: { category?: Category }) {
  const action = category ? updateCategoryAction.bind(null, category.id) : createCategoryAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Name
        <input type="text" name="name" defaultValue={category?.name} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={category?.slug} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={3} defaultValue={category?.description} />
      </label>
      <label>
        Position
        <input type="number" name="position" defaultValue={category?.position ?? 0} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="active" defaultChecked={category?.active ?? true} />
        Active
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
