# Moderator Guide

This guide covers the moderation workflow for users with the `moderator` role.

---

## Moderator vs. admin

| Capability | Moderator | Admin |
|------------|-----------|-------|
| View and manage reports | Yes | Yes |
| Take moderation actions (hide, unpublish, suspend) | Yes | Yes |
| View audit log | Yes | Yes |
| Add notes to reports | Yes | Yes |
| Review appeals | Yes | Yes |
| Delete videos permanently | No | Yes |
| Ban users platform-wide | No | Yes |
| Manage user roles | No | Yes |
| Access system settings | No | Yes |

---

## Accessing the moderation panel

Sign in and navigate to `/admin`. The **Moderation** tab is your primary workspace. You will see:

- **Stats** — open reports, reports assigned to you, pending appeals
- **Report Queue** — all incoming reports
- **Actions** — history of moderation actions taken
- **Audit Log** — immutable record of all admin/mod actions

---

## Report workflow

### 1 — Review the queue

**Admin → Moderation → Reports**

The queue lists all reports sorted by creation time. Filter by:
- **Status** — `open`, `assigned`, `resolved`, `dismissed`
- **Type** — video, comment, user, live stream, chat message

### 2 — Assign a report

Click **Assign to me** to claim a report. Once assigned the report moves to your personal queue, signalling to other moderators that it is being handled.

If you cannot handle a report, click **Unassign** to return it to the open queue.

### 3 — Review the content

Open the report detail view. You will see:
- The reported content (with a direct link)
- The reporter's reason and optional details
- Previous moderation history on that content or user
- Any existing admin notes

### 4 — Take action or dismiss

**Taking action:**

1. Click **Resolve with action**.
2. Select the appropriate action:

   | Action | Effect |
   |--------|--------|
   | `hide_content` | Marks content as hidden; no longer visible publicly |
   | `unpublish_video` | Changes video visibility to `unlisted` |
   | `remove_comment` | Soft-deletes the comment |
   | `suspend_user` | Prevents the user from logging in |
   | `terminate_live_stream` | Ends the broadcast immediately |
   | `lock_live_chat` | Disables new messages in the chat |

3. Add a **note** (optional but recommended — visible in the audit log).
4. Click **Confirm**.

**Dismissing a report:**

If the content does not violate platform rules:

1. Click **Dismiss**.
2. Add a brief note explaining why.
3. Click **Confirm**.

### 5 — Reversing an action

If a moderation action was applied incorrectly:

1. Go to **Admin → Moderation → Actions**.
2. Find the action.
3. Click **Reverse**.

Reversed actions are recorded in the audit log alongside the original action.

---

## Handling appeals

When a user appeals a moderation decision, the report moves to an `appeal` state.

1. Go to **Admin → Moderation → Reports** and filter by status `appeal`.
2. Open the report and read the user's appeal statement.
3. Review the original content and the action taken.
4. Click **Grant appeal** or **Deny appeal**.
   - **Granted** — the moderation action is reversed and the content is restored.
   - **Denied** — the action stands and the user is notified.
5. Add a note explaining the decision.

---

## Adding admin notes

You can add internal notes to any report at any stage. Notes are visible to all moderators and admins but not to the reporter or the content owner.

Open the report → **Notes** tab → type your note → **Save**.

---

## Audit log

**Admin → Audit Log**

Every action you take is recorded automatically. Use the audit log to:

- Review the history of actions taken on a specific user or piece of content
- Check what another moderator did on a report before you picked it up
- Prepare a summary if a dispute is escalated

Filter by actor (your own user ID to see your history), action type, or date range.

---

## Live chat moderation

Moderators can moderate live chat across all streams (not just their own).

From the stream watch page:
- Hover a message → **Delete**
- Click a username → **Timeout** (choose duration) or **Ban**
- Chat settings → **Lock Chat** / **Unlock Chat**

All chat moderation actions are logged in the audit log.

---

## Reporting a report (escalation)

If a report involves content that requires permanent deletion or a permanent ban — actions reserved for admins — add a note to the report and reach out to an admin directly. You can assign the report to an admin's name in the notes.

---

## Best practices

- **Assign before acting.** Always claim a report before taking action so other moderators do not duplicate effort.
- **Leave notes.** Brief notes in every resolved or dismissed report make appeals and audits much easier.
- **Don't dismiss without reading.** Even low-quality reports occasionally contain valid concerns buried in the details field.
- **Err toward lighter actions.** Prefer `hide_content` or `unpublish_video` over suspending a user. Suspensions are more disruptive and harder for the user to appeal.
- **When in doubt, escalate.** Add a note and ask an admin if you are unsure whether content crosses the line.
