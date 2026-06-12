-- Add appeal_status to moderation_reports
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appeal_status TEXT;
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS admin_notes TEXT;
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS resolved_by UUID REFERENCES users(id);

-- Add admin_notes to moderation_actions
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS admin_notes TEXT;

-- Add appeal_reason to moderation_reports
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appeal_reason TEXT;
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appealed_at TIMESTAMPTZ;
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appeal_resolved_by UUID REFERENCES users(id);
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appeal_resolved_at TIMESTAMPTZ;
ALTER TABLE moderation_reports ADD COLUMN IF NOT EXISTS appeal_decision TEXT; -- 'granted', 'denied'

-- Add restored_by / restored_at to moderation_actions for reversibility
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS reversed_by UUID REFERENCES users(id);
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS reversed_at TIMESTAMPTZ;
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS reversed_reason TEXT;

-- Index for report queue ordering
CREATE INDEX IF NOT EXISTS idx_moderation_reports_queue
    ON moderation_reports(priority DESC, created_at ASC)
    WHERE status = 'open' OR status = 'assigned';

-- Index for appeal lookups
CREATE INDEX IF NOT EXISTS idx_moderation_reports_appeals
    ON moderation_reports(appeal_status)
    WHERE appeal_status IS NOT NULL;

-- Index for audit log by actor
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_id ON audit_log(actor_id);

-- Add more permissions for fine-grained moderation
INSERT INTO permissions (name) VALUES
    ('moderation:reports:list'),
    ('moderation:reports:assign'),
    ('moderation:reports:resolve'),
    ('moderation:actions:hide'),
    ('moderation:actions:unpublish'),
    ('moderation:actions:delete'),
    ('moderation:actions:suspend'),
    ('moderation:actions:ban'),
    ('moderation:actions:restore'),
    ('moderation:audit:view'),
    ('moderation:notes:manage'),
    ('moderation:appeals:review')
ON CONFLICT (name) DO NOTHING;

-- Grant new permissions to admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
  AND p.name IN (
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
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Grant subset to moderator role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'moderator'
  AND p.name IN (
    'moderation:reports:list',
    'moderation:reports:assign',
    'moderation:reports:resolve',
    'moderation:actions:hide',
    'moderation:actions:unpublish',
    'moderation:actions:restore',
    'moderation:audit:view',
    'moderation:notes:manage'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;