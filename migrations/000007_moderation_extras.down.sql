-- Drop new permissions
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN (
        'moderation:reports:list',
        'moderation:reports:assign',
        'moderation:reports:resolve',
        'moderation:actions:hide',
        'moderation:actions:unpublish',
        'moderation:actions:delete',
        'moderation:actions:suspend',
        'moderation:actions:ban',
        'moderation:actions:restore',
        'moderation:audit:view',
        'moderation:notes:manage',
        'moderation:appeals:review'
    )
);

DELETE FROM permissions WHERE name IN (
    'moderation:reports:list',
    'moderation:reports:assign',
    'moderation:reports:resolve',
    'moderation:actions:hide',
    'moderation:actions:unpublish',
    'moderation:actions:delete',
    'moderation:actions:suspend',
    'moderation:actions:ban',
    'moderation:actions:restore',
    'moderation:audit:view',
    'moderation:notes:manage',
    'moderation:appeals:review'
);

-- Drop indexes
DROP INDEX IF EXISTS idx_moderation_reports_queue;
DROP INDEX IF EXISTS idx_moderation_reports_appeals;
DROP INDEX IF EXISTS idx_audit_log_actor_id;

-- Drop columns from moderation_actions
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS reversed_by;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS reversed_at;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS reversed_reason;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS admin_notes;

-- Drop columns from moderation_reports
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appeal_status;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS admin_notes;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS resolved_by;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appeal_reason;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appealed_at;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appeal_resolved_by;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appeal_resolved_at;
ALTER TABLE moderation_reports DROP COLUMN IF EXISTS appeal_decision;