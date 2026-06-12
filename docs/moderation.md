# Moderation

TPT Online Video includes a full moderation workflow system with roles, reports, actions, audit logging, and an admin dashboard UI.

## Architecture

The moderation system is structured in three layers:

- **Repository** (`services/api/internal/moderation/repository.go`) — database access layer
- **Service** (`services/api/internal/moderation/service.go`) — business logic & action application
- **Handler** (`services/api/internal/http/handlers/moderation.go`) — HTTP API endpoints
- **Frontend** (`apps/web/src/admin/ModerationDashboard.tsx`) — React admin dashboard

## Roles & Permissions

The system defines these roles (stored in the `roles` table):

| Role        | Description                              |
|-------------|------------------------------------------|
| `admin`     | Full access to all moderation features   |
| `moderator` | Can manage reports, take actions, view audit logs, but cannot delete content or ban users |
| `user`      | Can create reports on content            |

Fine-grained permissions are stored in the `permissions` table:

| Permission                       | Description                       |
|----------------------------------|-----------------------------------|
| `moderation:reports:list`        | View all reports                  |
| `moderation:reports:assign`      | Assign reports to moderators      |
| `moderation:reports:resolve`     | Resolve or dismiss reports        |
| `moderation:actions:hide`        | Hide content                      |
| `moderation:actions:unpublish`   | Unpublish a video                 |
| `moderation:actions:delete`      | Delete a video                    |
| `moderation:actions:suspend`     | Suspend a user                    |
| `moderation:actions:ban`         | Ban a user                        |
| `moderation:actions:restore`     | Restore previously moderated content |
| `moderation:audit:view`          | View the audit log                |
| `moderation:notes:manage`        | Add admin notes to reports        |
| `moderation:appeals:review`      | Review and resolve appeals        |

## Moderation targets

The system supports reports against these target types:

- `video` — VOD content
- `comment` — Comments on videos
- `user` — User profiles / accounts
- `live_stream` — Live broadcasts
- `live_chat_message` — Individual live chat messages

## Moderation actions

When resolving a report, the system can apply these actions:

| Action               | Effect                                                    |
|----------------------|-----------------------------------------------------------|
| `hide_content`       | Marks video as private or comment as hidden               |
| `unpublish_video`    | Removes video from public listing, sets to unlisted       |
| `delete_video`       | Soft-deletes the video                                    |
| `remove_comment`     | Soft-deletes the comment                                  |
| `terminate_live_stream` | (Hook for live service — ends the broadcast)          |
| `lock_live_chat`     | (Hook for live service — disables chat)                   |
| `suspend_user`       | Sets user status to `suspended` — cannot log in           |
| `ban_user`           | Sets user status to `banned` — permanently blocked        |
| `restore_content`    | Reverses a previous hide/delete/remove action             |

## Workflow

```
1. User creates a report (via API)
       │
2. Report enters queue (status: 'open')
       │
3. Moderator reviews report queue
       │
4. Moderator assigns report to self (status: 'assigned')
       │
5. Moderator investigates and decides action:
   ├─ Take moderation action → action recorded + audit log → report resolved
   ├─ Dismiss report → no action taken → report dismissed
   └─ Add admin notes → optional investigation context
       │
6. (Optional) User appeals the decision
       │
7. Another moderator reviews the appeal:
   ├─ Granted → original content restored
   └─ Denied → original decision stands
```

## API Endpoints

### User-facing (authenticated)

| Method | Path                                  | Description                        |
|--------|---------------------------------------|------------------------------------|
| POST   | `/api/v1/reports`                     | Create a report (general)          |
| POST   | `/api/v1/videos/{videoID}/report`     | Report a video                     |
| POST   | `/api/v1/users/{userID}/report`       | Report a user                      |
| POST   | `/api/v1/comments/{commentID}/report` | Report a comment (legacy)          |
| POST   | `/api/v1/live/streams/{streamID}/report` | Report a live stream           |
| POST   | `/api/v1/live/chat/{messageID}/report` | Report a live chat message        |
| POST   | `/api/v1/reports/{reportID}/appeal`   | Submit an appeal on a resolved report |

### Admin/mod-only

| Method | Path                                                | Description                    |
|--------|------------------------------------------------------|--------------------------------|
| GET    | `/api/v1/admin/moderation/stats`                    | Dashboard statistics           |
| GET    | `/api/v1/admin/reports`                             | List reports (with filters)    |
| GET    | `/api/v1/admin/reports/{reportID}`                  | Get single report details      |
| POST   | `/api/v1/admin/reports/{reportID}/assign`           | Assign report to moderator     |
| POST   | `/api/v1/admin/reports/{reportID}/unassign`         | Unassign a report              |
| POST   | `/api/v1/admin/reports/{reportID}/resolve`          | Resolve with optional action   |
| POST   | `/api/v1/admin/reports/{reportID}/dismiss`          | Dismiss report without action  |
| PUT    | `/api/v1/admin/reports/{reportID}/notes`            | Add admin notes to report      |
| POST   | `/api/v1/admin/reports/{reportID}/appeal`           | Resolve an appeal              |
| POST   | `/api/v1/admin/moderation/actions`                  | Execute a moderation action    |
| GET    | `/api/v1/admin/moderation/actions`                  | List moderation actions        |
| GET    | `/api/v1/admin/moderation/actions/{actionID}`       | Get single action details      |
| POST   | `/api/v1/admin/moderation/actions/{actionID}/reverse` | Reverse an action           |
| GET    | `/api/v1/admin/audit-log`                           | List audit log entries         |

## Database Schema

The moderation system uses these tables (defined in `000001_init.up.sql` and extended in `000007_moderation_extras.up.sql`):

### `moderation_reports`
- Core table for all report types
- Tracks: reporter, assignee, target type/ID, reason, status, priority
- Supports appeals with `appeal_status`, `appeal_reason`, `appeal_decision`
- Supports admin notes for internal comments

### `moderation_actions`
- Records every moderation action taken
- Linked to a report (optional via `report_id`)
- Supports reversal tracking (`reversed_by`, `reversed_at`, `reversed_reason`)
- Stores metadata as JSONB for extensibility

### `audit_log`
- Immutable log of all admin/moderation actions
- Stores: actor, action, target, IP address, user agent
- Queryable by actor, action, target type, target ID

## Appeal Status Field

When a report is resolved, the affected user can file an appeal:

- `appeal_status`: `pending` | `granted` | `denied`
- `appeal_reason`: User's explanation for the appeal
- `appealed_at`: When the appeal was filed
- `appeal_decision`: Moderator's decision on the appeal
- `appeal_resolved_by`: Which moderator handled it
- `appeal_resolved_at`: When it was resolved

## Action Reversal

Moderation actions can be reversed (e.g., if taken in error):

- `reversed_by`: ID of the moderator who reversed it
- `reversed_at`: When it was reversed
- `reversed_reason`: Explanation for the reversal
- When an action is reversed, the system automatically restores the content's original state where applicable

## Auditability

Every moderation action is recorded in `audit_log` with:

- Actor (moderator/admin who performed the action)
- Target type and ID
- Action identifier (e.g., `moderation:hide_content`, `moderation:reverse:delete_video`)
- Metadata JSON with contextual information
- IP address (from `X-Forwarded-For` or `RemoteAddr`)
- User agent string
- Timestamp

## Policy template

This software provides workflow and audit tooling. It does not define legal or community policy for deployers. Each deployment owner is responsible for their own moderation policy.

A sample policy template can be found below — adapt this to your community's needs:

```markdown
# [Platform Name] Moderation Policy

## Scope
This policy applies to all content types: videos, comments, live streams, live chat, and user profiles.

## Prohibited Content
1. Illegal content (as defined by applicable law)
2. Harassment, hate speech, or bullying
3. Spam or misleading metadata
4. Copyright-infringing material
5. Sexually explicit or obscene material (where applicable)
6. Threats of violence

## Enforcement
- First violation: Content hidden, warning issued
- Repeated violations: Account suspension (1-30 days)
- Severe violations: Permanent ban

## Appeals
Users may appeal moderation actions within 14 days by filing an appeal
through the report interface.

## Moderation Team
- Admins have full authority to enforce all policies
- Moderators can hide content and issue warnings/suspensions
- Bans require admin approval
```

## Testing

The moderation system includes integration tests in `services/api/internal/moderation/repository_test.go` that verify:

- Report creation and lifecycle (open → assigned → resolved)
- Appeal flow (submit → granted/denied)
- Moderation action recording and listing
- Audit log creation and querying
- Report filtering by target type and status
- Action reversal tracking

Run tests with:

```bash
cd services/api
go test ./internal/moderation/... -v -count=1
```

Note: Tests require a running PostgreSQL instance with the schema applied.
