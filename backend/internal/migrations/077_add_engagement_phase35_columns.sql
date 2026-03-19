-- Migration 077: Add Phase 35 engagement columns for email tracking, A/B testing, and compliance

-- Add gmail_thread_id to sequence_step_executions for reply thread tracking
ALTER TABLE sequence_step_executions ADD COLUMN gmail_thread_id TEXT;

-- Add variant_id to sequence_step_executions for A/B test attribution
ALTER TABLE sequence_step_executions ADD COLUMN variant_id TEXT;

-- Add soft_bounce_count to sequence_enrollments for bounce escalation logic
ALTER TABLE sequence_enrollments ADD COLUMN soft_bounce_count INTEGER NOT NULL DEFAULT 0;

-- Add do_not_email to contacts for compliance suppression
ALTER TABLE contacts ADD COLUMN do_not_email INTEGER NOT NULL DEFAULT 0;

-- Add physical_address to org_settings for CAN-SPAM compliance footer
ALTER TABLE org_settings ADD COLUMN physical_address TEXT DEFAULT '';

-- Indexes for new columns
CREATE INDEX IF NOT EXISTS idx_step_exec_thread ON sequence_step_executions(gmail_thread_id) WHERE gmail_thread_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_step_exec_variant ON sequence_step_executions(variant_id) WHERE variant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_do_not_email ON contacts(do_not_email) WHERE do_not_email = 1;
