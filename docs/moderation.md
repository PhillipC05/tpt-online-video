# Moderation

TPT Online Video includes a full moderation workflow design.

## Moderation targets

The system is designed to moderate:

- Videos
- Comments
- Users
- Live streams
- Live chat messages

## Moderation actions

Planned actions:

- Hide content
- Unpublish video
- Delete video
- Remove comment
- Terminate live stream
- Lock live chat
- Suspend user
- Ban user
- Restore content

## Auditability

Every moderation action should be recorded in `audit_log` with:

- Actor
- Target type
- Target ID
- Action
- Reason
- Metadata
- IP address where available
- Timestamp

## Workflow

1. User or system creates a report.
2. Moderator reviews the report.
3. Moderator takes an action.
4. System records audit log.
5. Affected content/user state is updated.
6. Optional appeal status can be tracked later.

## Policy note

This software provides workflow and audit tooling. It does not define legal or community policy for deployers. Each deployment owner is responsible for their own moderation policy.