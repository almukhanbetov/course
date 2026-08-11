import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getPlan, formatPrice } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { mockConfirmPaymentAction } from "@/lib/actions";

export default async function CheckoutPage({
  params,
  searchParams,
}: {
  params: Promise<{ planId: string }>;
  searchParams: Promise<{ payment?: string; error?: string }>;
}) {
  const { planId } = await params;
  const { payment, error } = await searchParams;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const plan = await getPlan(planId);
  if (!plan) {
    notFound();
  }

  if (!payment) {
    redirect(`/pricing?error=${encodeURIComponent("Сначала выберите тариф на странице тарифов")}`);
  }

  return (
    <main>
      <Link href="/pricing">← Тарифы</Link>
      <h1>Оформление подписки «{plan.name}»</h1>

      <div className="payment-simulator-banner" role="alert">
        ⚠ Development payment simulator — это тестовая имитация оплаты, деньги не списываются.
        Реальный платёжный провайдер (Kaspi/Stripe и т.п.) в этой версии не подключён.
      </div>

      <div className="admin-card">
        <p>
          Тариф: <strong>{plan.name}</strong>
        </p>
        <p>
          Сумма: <strong>{formatPrice(plan)}</strong>
        </p>
        <p>Срок: {plan.duration_days} дней</p>
        <p className="my-course-meta">Payment ID: {payment}</p>

        {error && <p role="alert">{error}</p>}

        <form action={mockConfirmPaymentAction.bind(null, planId, payment)}>
          <button type="submit" className="btn-primary">
            Подтвердить тестовый платёж
          </button>
        </form>
      </div>
    </main>
  );
}
