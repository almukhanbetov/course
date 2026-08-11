import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListPlans } from "@/lib/admin-api";
import { formatPrice } from "@/lib/api";
import { PlanForm } from "@/components/admin/PlanForm";

export const metadata: Metadata = {
  title: "Plans — Admin",
};

export default async function AdminPlansPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const plans = await adminListPlans(token);

  return (
    <div>
      <div className="admin-header">
        <h1>Plans</h1>
      </div>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Slug</th>
              <th>Price</th>
              <th>Duration</th>
              <th>Active</th>
            </tr>
          </thead>
          <tbody>
            {plans.map((plan) => (
              <tr key={plan.id}>
                <td>{plan.name}</td>
                <td>{plan.slug}</td>
                <td>{formatPrice(plan)}</td>
                <td>{plan.duration_days} days</td>
                <td>{plan.active ? "yes" : "no"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {plans.map((plan) => (
        <details key={plan.id} className="admin-card">
          <summary>Edit «{plan.name}»</summary>
          <PlanForm plan={plan} />
        </details>
      ))}

      <div className="admin-card">
        <h3>New plan</h3>
        <PlanForm />
      </div>
    </div>
  );
}
