-- Migration 075: Create core engagement tables for Sales Engagement Module (v7.0)
-- All tables are tenant-scoped — do NOT add any to masterOnlyTableNames

-- sequences: Sales sequences (org-level, not user-level)
CREATE TABLE IF NOT EXISTS sequences (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    timezone TEXT NOT NULL DEFAULT 'America/New_York',
    business_hours_start TEXT NOT NULL DEFAULT '09:00',
    business_hours_end TEXT NOT NULL DEFAULT '17:00',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sequences_org ON sequences(org_id);

-- sequence_steps: Individual steps within a sequence
CREATE TABLE IF NOT EXISTS sequence_steps (
    id TEXT PRIMARY KEY,
    sequence_id TEXT NOT NULL,
    step_number INTEGER NOT NULL,
    step_type TEXT NOT NULL,
    delay_days INTEGER NOT NULL DEFAULT 0,
    delay_hours INTEGER NOT NULL DEFAULT 0,
    template_id TEXT,
    config_json TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sequence_steps_sequence ON sequence_steps(sequence_id);

-- sequence_enrollments: Contact enrollments in sequences (contacts only, no leads)
CREATE TABLE IF NOT EXISTS sequence_enrollments (
    id TEXT PRIMARY KEY,
    sequence_id TEXT NOT NULL,
    contact_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    enrolled_by TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'enrolled',
    current_step INTEGER NOT NULL DEFAULT 0,
    ab_variant_id TEXT,
    enrolled_at TEXT,
    finished_at TEXT,
    paused_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(sequence_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_seq_enrollments_org_user ON sequence_enrollments(org_id, enrolled_by);
CREATE INDEX IF NOT EXISTS idx_seq_enrollments_seq_status ON sequence_enrollments(sequence_id, status);
CREATE INDEX IF NOT EXISTS idx_seq_enrollments_contact ON sequence_enrollments(contact_id);

-- sequence_step_executions: Per-step execution tracking
CREATE TABLE IF NOT EXISTS sequence_step_executions (
    id TEXT PRIMARY KEY,
    enrollment_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    scheduled_at TEXT,
    executed_at TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_seq_step_exec_scheduler ON sequence_step_executions(org_id, status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_seq_step_exec_enrollment ON sequence_step_executions(enrollment_id);

-- email_templates: Reusable email templates for sequence steps
CREATE TABLE IF NOT EXISTS email_templates (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT,
    body_text TEXT,
    has_compliance_footer INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_templates_org ON email_templates(org_id);

-- email_tracking_events: Immutable event log for opens, clicks, replies, bounces
CREATE TABLE IF NOT EXISTS email_tracking_events (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    enrollment_id TEXT NOT NULL,
    step_execution_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    link_url TEXT,
    metadata_json TEXT,
    occurred_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_tracking_org_enrollment ON email_tracking_events(org_id, enrollment_id);
CREATE INDEX IF NOT EXISTS idx_email_tracking_exec_type ON email_tracking_events(step_execution_id, event_type);

-- call_dispositions: Outcomes logged after call steps
CREATE TABLE IF NOT EXISTS call_dispositions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    step_execution_id TEXT NOT NULL,
    contact_id TEXT NOT NULL,
    enrolled_by TEXT NOT NULL,
    disposition TEXT NOT NULL,
    notes TEXT,
    duration_seconds INTEGER,
    called_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_call_dispositions_org_contact ON call_dispositions(org_id, contact_id);

-- sms_messages: SMS messages sent/received during sequence steps
CREATE TABLE IF NOT EXISTS sms_messages (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    enrollment_id TEXT NOT NULL,
    step_execution_id TEXT NOT NULL,
    contact_id TEXT NOT NULL,
    direction TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    external_id TEXT,
    sent_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sms_messages_org_contact ON sms_messages(org_id, contact_id);

-- opt_out_list: Per-channel opt-out registry (email/sms/all)
CREATE TABLE IF NOT EXISTS opt_out_list (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    contact_id TEXT NOT NULL,
    channel TEXT NOT NULL,
    reason TEXT,
    opted_out_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, contact_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_opt_out_org_contact ON opt_out_list(org_id, contact_id);

-- email_warmup_sessions: Gmail account warmup tracking
CREATE TABLE IF NOT EXISTS email_warmup_sessions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    gmail_account_email TEXT NOT NULL,
    daily_limit INTEGER NOT NULL DEFAULT 5,
    current_daily_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    started_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_warmup_sessions_org_user ON email_warmup_sessions(org_id, user_id);

-- ab_test_variants: A/B test variant definitions for sequence steps
CREATE TABLE IF NOT EXISTS ab_test_variants (
    id TEXT PRIMARY KEY,
    step_id TEXT NOT NULL,
    variant_label TEXT NOT NULL,
    subject_override TEXT,
    body_html_override TEXT,
    traffic_pct INTEGER NOT NULL DEFAULT 0,
    is_winner INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ab_variants_step ON ab_test_variants(step_id);

-- ab_tracking_stats: Aggregated A/B variant performance stats
CREATE TABLE IF NOT EXISTS ab_tracking_stats (
    id TEXT PRIMARY KEY,
    variant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    sends INTEGER NOT NULL DEFAULT 0,
    opens INTEGER NOT NULL DEFAULT 0,
    clicks INTEGER NOT NULL DEFAULT 0,
    replies INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(variant_id)
);

CREATE INDEX IF NOT EXISTS idx_ab_stats_variant ON ab_tracking_stats(variant_id);
