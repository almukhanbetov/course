-- +goose Up
-- Production Launch Fix 1: 00008_seed_admin_user.sql shipped a hardcoded,
-- publicly-documented admin credential (admin@example.com / a password
-- named in that migration's own comment) that runs unconditionally in
-- every environment goose migrates, including production — there is no
-- environment branching anywhere in this project's migration or deploy
-- path. Since this repository is public, that credential is visible to
-- anyone, on any deployment that has run 00008.
--
-- This migration is intentionally NOT an edit to 00008 itself — migrations
-- are treated as immutable history in this project once shipped; editing
-- 00008 would do nothing for any database that already ran it (this
-- project's own dev stack, and possibly real production) and would only
-- silently change behavior for brand-new environments, which is a worse,
-- less auditable fix than an explicit follow-up migration.
--
-- Matches on the EXACT bcrypt hash 00008 inserted, not just the email —
-- bcrypt embeds a fresh random salt on every hash, so this can only ever
-- match the untouched original seeded row. If an operator already rotated
-- that row's password by hand, the hash no longer matches and this is a
-- safe no-op. Deactivates the row (rather than deleting it, avoiding any
-- FK-reference complication with rows that may already point at this
-- user's id) and replaces its password with a freshly-generated,
-- cryptographically random value that is never returned to the client,
-- logged, or stored anywhere outside this row — bcrypt-hashed server-side
-- via pgcrypto (already enabled by 00001_init_extensions.sql), so no
-- plaintext password ever exists at all, not even transiently.
--
-- After this runs, zero active admin accounts exist anywhere it applies
-- (fresh installs included, since 00008 -> 00041 always run back-to-back
-- in sequence) — see cmd/bootstrap-admin, which is the only supported way
-- to create the first real admin from here on, from runtime configuration
-- only, never from source code.
UPDATE users
SET password_hash = crypt(encode(gen_random_bytes(32), 'hex'), gen_salt('bf', 10)),
    active = false,
    updated_at = now()
WHERE email = 'admin@example.com'
  AND password_hash = '$2a$10$r8bZWjVNrOGGz4.dLlfm2eKb3KNa2vy45HXJMkqH5sxDkmcJg04FW';

-- +goose Down
-- Deliberately not reversible: restoring the original hardcoded credential
-- would reintroduce the exact vulnerability this migration exists to
-- close. A `down` migration that undoes a security fix is worse than no
-- `down` at all, so this is a documented no-op.
SELECT 1;
