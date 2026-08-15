import Link from "next/link";
import { getPlans, formatPrice } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { createSubscriptionAction } from "@/lib/actions";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

export default async function PricingPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const { error } = await searchParams;
  const locale = await getLocale();
  const dict = getDictionary(locale).pricing;
  const [plans, token] = await Promise.all([getPlans(), getSessionToken()]);

  return (
    <main>
      <h1>{dict.title}</h1>
      <p className="subtitle">{dict.subtitle}</p>

      {error && <p role="alert">{error}</p>}

      <div className="plans-grid">
        {plans.map((plan) => (
          <div key={plan.id} className="plan-card">
            <h2>{plan.name}</h2>
            <p className="plan-price">{formatPrice(plan)}</p>
            <p className="my-course-meta">{dict.days(plan.duration_days)}</p>
            <p>{plan.description}</p>
            {token ? (
              <form action={createSubscriptionAction.bind(null, plan.id)}>
                <button type="submit" className="btn-primary">
                  {dict.subscribeCta}
                </button>
              </form>
            ) : (
              <Link href="/login" className="btn-primary">
                {dict.loginToSubscribe}
              </Link>
            )}
          </div>
        ))}
      </div>

      {plans.length === 0 && <p>{dict.noPlans}</p>}
    </main>
  );
}
