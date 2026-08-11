"use client";

import { useActionState } from "react";
import { createPlanAction, updatePlanAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Plan } from "@/lib/api";

const initialState: FormState = { error: null };

export function PlanForm({ plan }: { plan?: Plan }) {
  const action = plan ? updatePlanAction.bind(null, plan.id) : createPlanAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Name
        <input type="text" name="name" defaultValue={plan?.name} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={plan?.slug} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={3} defaultValue={plan?.description} />
      </label>
      <label>
        Price ({plan?.currency ?? "KZT"}, major units, e.g. 9900.00)
        <input
          type="number"
          name="price"
          step="0.01"
          min="0"
          defaultValue={plan ? (plan.price_amount / 100).toFixed(2) : undefined}
          required
        />
      </label>
      <label>
        Currency
        <input type="text" name="currency" maxLength={3} defaultValue={plan?.currency ?? "KZT"} required />
      </label>
      <label>
        Duration (days)
        <input type="number" name="duration_days" min="1" defaultValue={plan?.duration_days ?? 30} required />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="active" defaultChecked={plan?.active ?? true} />
        Active
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
