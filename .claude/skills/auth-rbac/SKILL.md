---
name: auth-rbac
description: Implement secure authentication and role-based access control for students, instructors and admins.
---

# Auth & RBAC

Initial roles:
- student
- instructor
- admin

## Rules
- Login uses email + password.
- Store only secure password hashes.
- Never store plaintext passwords.
- Backend enforces permissions.
- Hiding a frontend button is not authorization.
- Keep authentication logic in `internal/auth`.
- Keep profile/user-domain logic in `internal/users`.

## Protected actions
Examples:
- student: access owned/enrolled learning data
- instructor: manage permitted educational content
- admin: manage platform-wide resources

## Security
- Validate credentials securely.
- Avoid leaking whether sensitive accounts exist when unnecessary.
- Rotate secrets through environment/config management.
- Production traffic uses HTTPS.
