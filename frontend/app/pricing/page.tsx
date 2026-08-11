import Link from "next/link";
import { getPlans, formatPrice } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { createSubscriptionAction } from "@/lib/actions";

export default async function PricingPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const { error } = await searchParams;
  const [plans, token] = await Promise.all([getPlans(), getSessionToken()]);

  return (
    <main>
      <h1>Тарифы</h1>
      <p className="subtitle">Подписка открывает доступ ко всем курсам с пометкой «По подписке».</p>

      {error && <p role="alert">{error}</p>}

      <div className="plans-grid">
        {plans.map((plan) => (
          <div key={plan.id} className="plan-card">
            <h2>{plan.name}</h2>
            <p className="plan-price">{formatPrice(plan)}</p>
            <p className="my-course-meta">{plan.duration_days} дней</p>
            <p>{plan.description}</p>
            {token ? (
              <form action={createSubscriptionAction.bind(null, plan.id)}>
                <button type="submit" className="btn-primary">
                  Оформить подписку
                </button>
              </form>
            ) : (
              <Link href="/login" className="btn-primary">
                Войдите, чтобы оформить
              </Link>
            )}
          </div>
        ))}
      </div>

      {plans.length === 0 && <p>Тарифы пока не добавлены.</p>}
    </main>
  );
}
