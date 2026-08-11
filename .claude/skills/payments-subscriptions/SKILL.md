---
name: payments-subscriptions
description: Implement subscriptions, payment records and access activation safely.
---

# Payments & Subscriptions

## Entities
- plans
- subscriptions
- payments

## Rules
- Payment provider state is authoritative for payment success.
- Never activate access only because the frontend says payment succeeded.
- Verify callbacks/webhooks according to provider requirements.
- Use idempotency for repeated payment notifications where possible.
- Use a DB transaction for payment-record + subscription/access updates.

## Keep payment secrets server-side only.
Never expose provider secret keys to browser code.
