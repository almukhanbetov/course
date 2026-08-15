// Command bootstrap-admin is Production Launch Fix 1's safe replacement for
// the hardcoded admin credential 00008_seed_admin_user.sql used to ship
// (now neutralized by 00041_neutralize_seeded_admin_credential.sql).
//
// It is deliberately NOT a goose migration: migrations are static SQL
// checked into git, and a production admin credential must never live in
// source code (a migration file, committed to a public repository, is
// exactly that). This command reads ADMIN_BOOTSTRAP_EMAIL/
// ADMIN_BOOTSTRAP_PASSWORD from its own process environment only — the
// same "VPS secrets stay on the VPS" convention every other production
// secret in this project already follows (BACKUP_ENCRYPTION_PASSPHRASE,
// JWT_SECRET, etc.) — never from a flag, never from a config file.
//
// Safe to run on every single deploy, forever: it first checks whether an
// active admin already exists and, if so, does nothing at all (exit 0).
// It only ever attempts to create/reactivate an admin when zero active
// admins exist — which happens exactly once in a real deployment's
// lifetime (right after 00041 neutralizes the old seeded row, or on a
// genuinely fresh database) — and in that case it fails closed (exit 1,
// no database write) if the required environment variables aren't set,
// rather than silently skipping or falling back to any built-in default.
//
// Run manually against a dev database:
//
//	go run ./cmd/bootstrap-admin
package main

import (
	"context"
	"log"
	"strings"

	"lms-backend/internal/auth"
	"lms-backend/internal/config"
	"lms-backend/internal/db"
)

// adminRoleID matches roles.id for 'admin', seeded by
// 00006_create_roles.sql with this fixed UUID — reused here rather than
// looked up by name to avoid an extra round trip; every other migration
// and seed script in this project already hardcodes this same value.
const adminRoleID = "44444444-4444-4444-4444-444444444443"

// minBootstrapPasswordLen guards against ADMIN_BOOTSTRAP_PASSWORD being set
// to something trivially weak (e.g. left as a short placeholder someone
// forgot to replace) — this command's whole purpose is "no weak/default
// admin password can provide production access," so it enforces a floor
// here too, not just "not the literal old hardcoded one."
const minBootstrapPasswordLen = 12

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("bootstrap-admin: database connection failed: %v", err)
	}
	defer pool.Close()

	var activeAdminCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE r.name = 'admin' AND u.active = true
	`).Scan(&activeAdminCount)
	if err != nil {
		log.Fatalf("bootstrap-admin: checking for an existing active admin failed: %v", err)
	}

	if activeAdminCount > 0 {
		log.Printf("bootstrap-admin: %d active admin account(s) already exist, nothing to do", activeAdminCount)
		return
	}

	// No active admin exists — this is the only branch that ever touches
	// ADMIN_BOOTSTRAP_EMAIL/ADMIN_BOOTSTRAP_PASSWORD, and it fails closed
	// (no database write at all) if either is missing. Values themselves
	// are never logged below, in either branch — only the resulting email
	// (not a secret) on success, or the name of what's missing on failure.
	email := strings.ToLower(strings.TrimSpace(cfg.AdminBootstrapEmail))
	password := cfg.AdminBootstrapPassword

	var missing []string
	if email == "" {
		missing = append(missing, "ADMIN_BOOTSTRAP_EMAIL")
	}
	if password == "" {
		missing = append(missing, "ADMIN_BOOTSTRAP_PASSWORD")
	}
	if len(missing) > 0 {
		log.Fatalf("bootstrap-admin: no active admin exists and %s (not set) - refusing to create an admin account without explicit runtime configuration", strings.Join(missing, ", "))
	}
	if len(password) < minBootstrapPasswordLen {
		log.Fatalf("bootstrap-admin: ADMIN_BOOTSTRAP_PASSWORD is shorter than the required minimum (%d characters) - refusing to create a weak admin account", minBootstrapPasswordLen)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("bootstrap-admin: hashing the bootstrap password failed: %v", err)
	}

	// ON CONFLICT (email) DO UPDATE, not DO NOTHING: the email this
	// bootstrap is asked to use might already exist as an inactive row
	// (e.g. exactly the row 00041 just deactivated, if an operator reuses
	// the same address) — DO NOTHING would silently leave zero active
	// admins in that case while still exiting 0, which is the opposite of
	// fail-safe. Forcing role/active/password_hash on conflict makes this
	// command's contract exactly "ensure this email is an active admin
	// with this password," regardless of that row's prior state — and
	// this branch is only ever reached when the pre-check above already
	// confirmed zero active admins exist, so this can never overwrite a
	// different, legitimately-active admin's credentials.
	tag, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, role_id, active)
		VALUES ($1, $2, 'Admin', 'User', $3, true)
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    role_id = EXCLUDED.role_id,
		    active = true,
		    updated_at = now()
	`, email, passwordHash, adminRoleID)
	if err != nil {
		log.Fatalf("bootstrap-admin: creating the admin account failed: %v", err)
	}

	log.Printf("bootstrap-admin: admin account ready for %s (%d row affected)", email, tag.RowsAffected())
}
