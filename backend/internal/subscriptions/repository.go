package subscriptions

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/notifications"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrDuplicateSlug    = errors.New("slug already exists")
	ErrPlanInactive     = errors.New("plan is not active")
	ErrDuplicateIdemKey = errors.New("duplicate idempotency key")
	ErrAlreadyProcessed = errors.New("payment already processed")
	ErrForbidden        = errors.New("you may not act on another user's payment")
	ErrWrongProvider    = errors.New("payment does not belong to the mock provider")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// --- Plans -------------------------------------------------------------

const planColumns = "id, name, slug, description, price_amount, currency, duration_days, active, created_at, updated_at"

func scanPlan(row pgx.Row) (*Plan, error) {
	var p Plan
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceAmount, &p.Currency, &p.DurationDays, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListActivePlans is the public catalog — active plans only.
func (r *Repository) ListActivePlans(ctx context.Context) ([]Plan, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+planColumns+` FROM subscription_plans WHERE active = true ORDER BY price_amount ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceAmount, &p.Currency, &p.DurationDays, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// ListAllPlansAdmin returns every plan (active or not) for the admin list.
func (r *Repository) ListAllPlansAdmin(ctx context.Context) ([]Plan, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+planColumns+` FROM subscription_plans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceAmount, &p.Currency, &p.DurationDays, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+planColumns+` FROM subscription_plans WHERE id = $1`, id)
	return scanPlan(row)
}

func (r *Repository) CreatePlan(ctx context.Context, input PlanInput) (*Plan, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO subscription_plans (name, slug, description, price_amount, currency, duration_days, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+planColumns,
		input.Name, input.Slug, input.Description, input.PriceAmount, input.Currency, input.DurationDays, input.Active)
	p, err := scanPlan(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	return p, err
}

func (r *Repository) UpdatePlan(ctx context.Context, id uuid.UUID, input PlanInput) (*Plan, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE subscription_plans
		SET name = $2, slug = $3, description = $4, price_amount = $5, currency = $6, duration_days = $7, active = $8, updated_at = now()
		WHERE id = $1
		RETURNING `+planColumns,
		id, input.Name, input.Slug, input.Description, input.PriceAmount, input.Currency, input.DurationDays, input.Active)
	p, err := scanPlan(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	return p, err
}

// --- Subscriptions -------------------------------------------------------

const subscriptionColumns = "id, user_id, plan_id, status, starts_at, expires_at, canceled_at, created_at, updated_at"

func scanSubscription(row pgx.Row) (*Subscription, error) {
	var s Subscription
	err := row.Scan(&s.ID, &s.UserID, &s.PlanID, &s.Status, &s.StartsAt, &s.ExpiresAt, &s.CanceledAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+subscriptionColumns+` FROM subscriptions WHERE id = $1`, id)
	return scanSubscription(row)
}

// GetActiveSubscription returns the caller's current subscription, treating
// one whose expires_at has already passed as not-active even if its status
// column still says "active" — see internal/access for why this must be
// evaluated live rather than trusted from the stored status. It opportunely
// syncs that stale status to "expired" on read, which is a cheap, safe way
// to keep the data honest without a background worker.
func (r *Repository) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	if _, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET status = 'expired', updated_at = now()
		WHERE user_id = $1 AND status = 'active' AND expires_at <= now()
	`, userID); err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT `+subscriptionColumns+`
		FROM subscriptions
		WHERE user_id = $1 AND status = 'active' AND expires_at > now()
		ORDER BY expires_at DESC
		LIMIT 1
	`, userID)
	return scanSubscription(row)
}

func (r *Repository) ListSubscriptionsAdmin(ctx context.Context, status string, limit, offset int) ([]AdminSubscriptionSummary, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.plan_id, s.status, s.starts_at, s.expires_at, s.canceled_at, s.created_at, s.updated_at,
			u.email, TRIM(u.first_name || ' ' || u.last_name), p.name,
			COUNT(*) OVER() AS total
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN subscription_plans p ON p.id = s.plan_id
		WHERE $1 = '' OR s.status = $1
		ORDER BY s.created_at DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminSubscriptionSummary{}
	total := 0
	for rows.Next() {
		var s AdminSubscriptionSummary
		if err := rows.Scan(&s.ID, &s.UserID, &s.PlanID, &s.Status, &s.StartsAt, &s.ExpiresAt, &s.CanceledAt, &s.CreatedAt, &s.UpdatedAt,
			&s.UserEmail, &s.UserName, &s.PlanName, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}

// --- Payments -------------------------------------------------------

const paymentColumns = "id, user_id, subscription_id, provider, provider_payment_id, amount, currency, status, idempotency_key, created_at, paid_at, failed_at"

func scanPayment(row pgx.Row) (*Payment, error) {
	var p Payment
	err := row.Scan(&p.ID, &p.UserID, &p.SubscriptionID, &p.Provider, &p.ProviderPaymentID, &p.Amount, &p.Currency, &p.Status, &p.IdempotencyKey, &p.CreatedAt, &p.PaidAt, &p.FailedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id)
	return scanPayment(row)
}

func (r *Repository) ListPaymentsAdmin(ctx context.Context, status string, limit, offset int) ([]AdminPaymentSummary, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, p.subscription_id, p.provider, p.provider_payment_id, p.amount, p.currency, p.status, p.idempotency_key, p.created_at, p.paid_at, p.failed_at,
			u.email, TRIM(u.first_name || ' ' || u.last_name)
		FROM payments p
		JOIN users u ON u.id = p.user_id
		WHERE $1 = '' OR p.status = $1
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminPaymentSummary{}
	for rows.Next() {
		var p AdminPaymentSummary
		if err := rows.Scan(&p.ID, &p.UserID, &p.SubscriptionID, &p.Provider, &p.ProviderPaymentID, &p.Amount, &p.Currency, &p.Status, &p.IdempotencyKey, &p.CreatedAt, &p.PaidAt, &p.FailedAt,
			&p.UserEmail, &p.UserName); err != nil {
			return nil, 0, err
		}
		result = append(result, p)
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE $1 = '' OR status = $1`, status).Scan(&total); err != nil {
		return nil, 0, err
	}

	return result, total, rows.Err()
}

// CreatePendingSubscriptionAndPayment creates a pending subscription and its
// pending payment atomically — a subscription is never left without a
// corresponding payment record, and vice versa.
func (r *Repository) CreatePendingSubscriptionAndPayment(ctx context.Context, userID, planID uuid.UUID, amount int64, currency, provider, providerPaymentID, idempotencyKey string) (*Subscription, *Payment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	subRow := tx.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+subscriptionColumns, userID, planID)
	sub, err := scanSubscription(subRow)
	if err != nil {
		return nil, nil, err
	}

	payRow := tx.QueryRow(ctx, `
		INSERT INTO payments (user_id, subscription_id, provider, provider_payment_id, amount, currency, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING `+paymentColumns,
		userID, sub.ID, provider, providerPaymentID, amount, currency, idempotencyKey)
	pay, err := scanPayment(payRow)
	if isUniqueViolation(err) {
		return nil, nil, ErrDuplicateIdemKey
	}
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return sub, pay, nil
}

// ConfirmPayment marks a pending payment paid and activates its subscription
// in one transaction. It is a no-op returning ErrAlreadyProcessed when the
// payment is already paid, which combined with SELECT ... FOR UPDATE makes
// repeated confirm calls (the idempotency requirement) safe under
// concurrent requests instead of only when called serially.
func (r *Repository) ConfirmPayment(ctx context.Context, paymentID uuid.UUID) (*Payment, *Subscription, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	var subscriptionID *uuid.UUID
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT status, subscription_id, user_id FROM payments WHERE id = $1 FOR UPDATE`, paymentID).Scan(&status, &subscriptionID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if status != "pending" {
		return nil, nil, ErrAlreadyProcessed
	}

	payRow := tx.QueryRow(ctx, `
		UPDATE payments SET status = 'paid', paid_at = now() WHERE id = $1
		RETURNING `+paymentColumns, paymentID)
	payment, err := scanPayment(payRow)
	if err != nil {
		return nil, nil, err
	}

	var duration int
	var planName string
	if err := tx.QueryRow(ctx, `
		SELECT p.duration_days, p.name FROM subscriptions s JOIN subscription_plans p ON p.id = s.plan_id WHERE s.id = $1
	`, subscriptionID).Scan(&duration, &planName); err != nil {
		return nil, nil, err
	}

	starts := time.Now().UTC()
	expires := starts.AddDate(0, 0, duration)

	subRow := tx.QueryRow(ctx, `
		UPDATE subscriptions
		SET status = 'active', starts_at = $2, expires_at = $3, updated_at = now()
		WHERE id = $1
		RETURNING `+subscriptionColumns, subscriptionID, starts, expires)
	subscription, err := scanSubscription(subRow)
	if err != nil {
		return nil, nil, err
	}

	// Both notifications are enqueued here — the only place a payment ever
	// flips to paid (the service layer's own pre-check plus this
	// transaction's FOR UPDATE guard against the same row make this
	// unreachable a second time for the same payment; see ConfirmPayment's
	// idempotent-replay handling in service.go for the retry case).
	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    userID,
		Type:      notifications.TypePaymentPaid,
		Data:      map[string]any{"payment_id": paymentID, "amount": payment.Amount, "currency": payment.Currency},
		DedupeKey: "payment_paid:" + paymentID.String(),
		Channels:  []string{notifications.ChannelInApp},
	}); err != nil {
		return nil, nil, err
	}
	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    userID,
		Type:      notifications.TypeSubscriptionActivated,
		Data:      map[string]any{"subscription_id": subscriptionID, "plan_name": planName, "expires_at": expires},
		DedupeKey: "subscription_activated:" + subscriptionID.String(),
		Channels:  []string{notifications.ChannelInApp, notifications.ChannelEmail},
	}); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return payment, subscription, nil
}

// --- Revenue analytics (admin-only, Stage 19A) ----------------------------

// GetRevenueAnalytics runs three small aggregate queries — payment flow,
// subscription flow, and point-in-time active/MRR — each GROUP BY currency,
// and merges them in Go keyed by currency. Every row stands on its own
// currency; nothing here ever sums across currencies (see CurrencyRevenue's
// doc comment).
func (r *Repository) GetRevenueAnalytics(ctx context.Context, from, to time.Time) ([]CurrencyRevenue, error) {
	byCurrency := map[string]*CurrencyRevenue{}

	get := func(currency string) *CurrencyRevenue {
		cr, ok := byCurrency[currency]
		if !ok {
			cr = &CurrencyRevenue{Currency: currency}
			byCurrency[currency] = cr
		}
		return cr
	}

	// Payment flow within [from, to), bucketed by paid_at. Note: this
	// codebase has no separate refunded_at column and no code path that
	// ever sets status='refunded' today (it is a CHECK-allowed value with
	// no producer yet), so refunded_amount/refunded_count read zero until
	// that exists — bucketing by paid_at is still the correct choice once
	// it does, since a payment must have been paid before it can be
	// refunded.
	payRows, err := r.pool.Query(ctx, `
		SELECT currency,
			COALESCE(SUM(amount) FILTER (WHERE status = 'paid'), 0),
			COUNT(*) FILTER (WHERE status = 'paid'),
			COALESCE(SUM(amount) FILTER (WHERE status = 'refunded'), 0),
			COUNT(*) FILTER (WHERE status = 'refunded')
		FROM payments
		WHERE status IN ('paid', 'refunded') AND paid_at >= $1 AND paid_at < $2
		GROUP BY currency
	`, from, to)
	if err != nil {
		return nil, err
	}
	for payRows.Next() {
		var currency string
		var paidAmount, refundedAmount int64
		var paidCount, refundedCount int
		if err := payRows.Scan(&currency, &paidAmount, &paidCount, &refundedAmount, &refundedCount); err != nil {
			payRows.Close()
			return nil, err
		}
		cr := get(currency)
		cr.PaidAmount, cr.PaidCount = paidAmount, paidCount
		cr.RefundedAmount, cr.RefundedCount = refundedAmount, refundedCount
	}
	if err := payRows.Err(); err != nil {
		return nil, err
	}
	payRows.Close()

	// Subscription flow within [from, to) — plans carry the currency, not
	// subscriptions themselves, hence the join.
	subFlowRows, err := r.pool.Query(ctx, `
		SELECT p.currency,
			COUNT(*) FILTER (WHERE s.created_at >= $1 AND s.created_at < $2),
			COUNT(*) FILTER (WHERE s.canceled_at >= $1 AND s.canceled_at < $2)
		FROM subscriptions s
		JOIN subscription_plans p ON p.id = s.plan_id
		GROUP BY p.currency
	`, from, to)
	if err != nil {
		return nil, err
	}
	for subFlowRows.Next() {
		var currency string
		var newSubs, canceledSubs int
		if err := subFlowRows.Scan(&currency, &newSubs, &canceledSubs); err != nil {
			subFlowRows.Close()
			return nil, err
		}
		cr := get(currency)
		cr.NewSubscriptions, cr.CanceledSubscriptions = newSubs, canceledSubs
	}
	if err := subFlowRows.Err(); err != nil {
		return nil, err
	}
	subFlowRows.Close()

	// Point-in-time (as of now, independent of from/to): active
	// subscriptions and normalized MRR — see CurrencyRevenue's doc comment
	// for the 30-day normalization formula.
	activeRows, err := r.pool.Query(ctx, `
		SELECT p.currency,
			COUNT(*),
			COALESCE(SUM(p.price_amount * 30.0 / p.duration_days), 0)::bigint
		FROM subscriptions s
		JOIN subscription_plans p ON p.id = s.plan_id
		WHERE s.status = 'active' AND s.expires_at > now()
		GROUP BY p.currency
	`)
	if err != nil {
		return nil, err
	}
	for activeRows.Next() {
		var currency string
		var active int
		var mrr int64
		if err := activeRows.Scan(&currency, &active, &mrr); err != nil {
			activeRows.Close()
			return nil, err
		}
		cr := get(currency)
		cr.ActiveSubscriptions, cr.MRR = active, mrr
	}
	if err := activeRows.Err(); err != nil {
		return nil, err
	}
	activeRows.Close()

	currencies := make([]string, 0, len(byCurrency))
	for currency := range byCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)

	result := make([]CurrencyRevenue, 0, len(currencies))
	for _, currency := range currencies {
		result = append(result, *byCurrency[currency])
	}
	return result, nil
}

// GetPlanBreakdown is Stage 19C's per-plan aggregate — one row per
// subscription_plans row, LEFT JOINed through subscriptions to payments so
// plans with zero subscriptions still appear (with all-zero counts) rather
// than being silently omitted. COUNT(DISTINCT s.id) guards the
// subscription-based counts against the payments join's fan-out (a plan
// could in principle have more payment rows than subscriptions if a future
// retry flow ever allows more than one payment per subscription — today
// it's always 1:1, but DISTINCT keeps this correct either way);
// COUNT(pay.id)/SUM(pay.amount) don't need DISTINCT since each payment row
// already appears at most once per subscription it belongs to.
func (r *Repository) GetPlanBreakdown(ctx context.Context, from, to time.Time) ([]PlanRevenue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id, p.name, p.currency,
			COUNT(DISTINCT s.id) FILTER (WHERE s.status = 'active' AND s.expires_at > now()),
			COUNT(DISTINCT s.id) FILTER (WHERE s.created_at >= $1 AND s.created_at < $2),
			COUNT(DISTINCT s.id) FILTER (WHERE s.canceled_at >= $1 AND s.canceled_at < $2),
			COALESCE(SUM(pay.amount) FILTER (WHERE pay.status = 'paid' AND pay.paid_at >= $1 AND pay.paid_at < $2), 0),
			COUNT(pay.id) FILTER (WHERE pay.status = 'paid' AND pay.paid_at >= $1 AND pay.paid_at < $2)
		FROM subscription_plans p
		LEFT JOIN subscriptions s ON s.plan_id = p.id
		LEFT JOIN payments pay ON pay.subscription_id = s.id
		GROUP BY p.id, p.name, p.currency
		ORDER BY p.name
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []PlanRevenue{}
	for rows.Next() {
		var pr PlanRevenue
		if err := rows.Scan(&pr.PlanID, &pr.PlanName, &pr.Currency,
			&pr.ActiveSubscriptions, &pr.NewSubscriptions, &pr.CanceledSubscriptions,
			&pr.PaidAmount, &pr.PaidCount); err != nil {
			return nil, err
		}
		result = append(result, pr)
	}
	return result, rows.Err()
}
