# Admin User Guide

This guide covers the admin dashboard and all tasks available to users with the `admin` role.

---

## Accessing the admin panel

Sign in with an admin account and navigate to `/admin` in the web interface. The admin panel opens as a tabbed dashboard with sections for users, videos, comments, moderation, audit log, and system settings.

> The seed admin account is created automatically on first startup using the `ADMIN_EMAIL` and `ADMIN_PASSWORD` environment variables. Change the password immediately after first login.

---

## User management

**Location:** Admin → Users

### Viewing users

The user list shows all registered accounts with their email, display name, role, status, and registration date. Use the search box to filter by name or email.

### Changing a user's role

| Role | Capabilities |
|------|-------------|
| `user` | Create content, comment, report |
| `moderator` | All of the above + manage reports, take most moderation actions, view audit log |
| `admin` | Full access including user/role management, system settings, and irreversible actions |

1. Find the user in the list.
2. Click **Edit**.
3. Change the **Role** dropdown.
4. Click **Save**.

### Suspending or banning a user

| Status | Effect |
|--------|--------|
| `active` | Normal access |
| `suspended` | Cannot log in; existing sessions are invalidated |
| `banned` | Permanent block; cannot register with the same email |

1. Find the user in the list.
2. Click **Edit**.
3. Change **Status** to `suspended` or `banned`.
4. Click **Save**.

> You cannot modify your own account through the admin endpoint (to prevent accidental self-lockout). Use the regular profile settings page for your own account.

---

## Video management

**Location:** Admin → Videos

The video list shows all videos across all users. Filter by status (`uploading`, `processing`, `ready`, `error`) or visibility (`public`, `unlisted`, `private`).

### Actions available

| Action | Description |
|--------|-------------|
| **Edit** | Change title, description, or visibility |
| **Delete** | Soft-delete the video (it remains in the database but is not visible) |

Deleted videos can be restored through a moderation action if needed.

---

## Comment management

**Location:** Admin → Comments

View all comments across the platform. Filter by video or user. Edit or remove individual comments directly.

---

## Moderation dashboard

**Location:** Admin → Moderation

See [Moderator Guide](moderator.md) for the full moderation workflow. Admins have all moderator capabilities plus:

- Ban users permanently
- Delete videos (not just unpublish or hide)
- Reverse any moderation action taken by any moderator
- Grant or deny appeals

---

## Audit log

**Location:** Admin → Audit Log

Every admin and moderation action is recorded with:
- **Actor** — who performed the action
- **Action** — what was done
- **Target** — which resource was affected
- **IP address** and **user agent**
- **Timestamp**

The audit log is append-only and cannot be edited or deleted through the UI.

Use the filters to search by actor, action type, or date range.

---

## System settings

**Location:** Admin → Settings

Current configurable settings (subject to change):

- Upload size limits
- Default video visibility
- Registration open/closed
- Comment moderation mode

> Most infrastructure settings (JWT secret, CORS, database) are managed through environment variables, not the UI. See [docs/security.md](../security.md) for secret rotation procedures.

---

## System status

**Location:** Admin → System Status

Displays:
- API, worker, and live service health
- Database connection status
- Redis connection status
- Storage provider status
- Active transcoding jobs
- Current live stream count

For Prometheus-format metrics, use `GET /api/v1/admin/system/metrics`.

---

## First-login checklist

- [ ] Change the seed admin password (Profile → Change Password)
- [ ] Set `CORS_ALLOWED_ORIGINS` to your actual domain
- [ ] Verify `JWT_SECRET` and `LIVE_HOOK_SECRET` are non-default random values
- [ ] Confirm uploads and transcoding work by uploading a short test video
- [ ] Confirm live streaming works with OBS
- [ ] Review the security checklist in [docs/security.md](../security.md)

---

## Common tasks

### Promote a user to moderator

Admin → Users → find user → Edit → Role → `moderator` → Save.

### Remove a bad video immediately

Admin → Videos → find video → Delete.  
Or: Moderation → Create Action → `delete_video` (records in the audit log).

### Unlock a suspended account

Admin → Users → find user → Edit → Status → `active` → Save.

### Investigate an incident

Admin → Audit Log → filter by date range and/or target user → export or note the relevant entries.

### Rotate the JWT secret

See [docs/security.md — Secret rotation](../security.md#secret-rotation-procedures). All users will be logged out and must sign in again.
