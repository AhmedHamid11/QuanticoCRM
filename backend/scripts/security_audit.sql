-- Security Audit Queries
-- Run these BEFORE implementing Phase 2 security fixes to understand impact

-- ============================================
-- 1. Check PDF Template Settings in Use
-- ============================================
-- Identifies non-standard values that might break with whitelist validation

SELECT
    id,
    org_id,
    name,
    page_size,
    orientation,
    margins,
    CASE
        WHEN page_size NOT IN ('A4', 'Letter', 'Legal', 'A3', '') THEN 'NON-STANDARD PAGE SIZE'
        ELSE 'OK'
    END as page_size_status,
    CASE
        WHEN orientation NOT IN ('portrait', 'landscape', '') THEN 'NON-STANDARD ORIENTATION'
        ELSE 'OK'
    END as orientation_status
FROM pdf_templates
WHERE page_size NOT IN ('A4', 'Letter', 'Legal', 'A3', '')
   OR orientation NOT IN ('portrait', 'landscape', '');

-- Count of templates by page size
SELECT page_size, COUNT(*) as count
FROM pdf_templates
GROUP BY page_size;

-- Count of templates by orientation
SELECT orientation, COUNT(*) as count
FROM pdf_templates
GROUP BY orientation;


-- ============================================
-- 2. Check Webhook URLs for Internal Addresses
-- ============================================
-- Identifies webhooks pointing to internal/private IPs that would be blocked

SELECT
    id,
    org_id,
    name,
    webhook_url,
    'POTENTIAL SSRF RISK' as risk_level
FROM tripwires
WHERE webhook_url LIKE '%localhost%'
   OR webhook_url LIKE '%127.0.0.1%'
   OR webhook_url LIKE '%192.168.%'
   OR webhook_url LIKE '%10.%'
   OR webhook_url LIKE '%172.16.%'
   OR webhook_url LIKE '%172.17.%'
   OR webhook_url LIKE '%172.18.%'
   OR webhook_url LIKE '%172.19.%'
   OR webhook_url LIKE '%172.2_.%'
   OR webhook_url LIKE '%172.30.%'
   OR webhook_url LIKE '%172.31.%'
   OR webhook_url LIKE '%169.254.%'
   OR webhook_url LIKE '%[::1]%'
   OR webhook_url LIKE '%0.0.0.0%';

-- All webhook URLs (for manual review)
SELECT
    org_id,
    name,
    webhook_url,
    enabled
FROM tripwires
WHERE webhook_url IS NOT NULL AND webhook_url != ''
ORDER BY org_id;


-- ============================================
-- 3. Check Rollup Queries for Security Issues
-- ============================================
-- Review all rollup queries for potential SQL injection or data leakage

SELECT
    fd.org_id,
    fd.entity_name,
    fd.name as field_name,
    fd.rollup_query,
    fd.rollup_result_type,
    CASE
        WHEN fd.rollup_query LIKE '%password%' THEN 'SENSITIVE DATA ACCESS'
        WHEN fd.rollup_query LIKE '%users%' THEN 'USER TABLE ACCESS'
        WHEN fd.rollup_query LIKE '%sessions%' THEN 'SESSION TABLE ACCESS'
        WHEN fd.rollup_query LIKE '%api_tokens%' THEN 'API TOKEN ACCESS'
        WHEN fd.rollup_query LIKE '%sqlite_master%' THEN 'SCHEMA ACCESS'
        WHEN fd.rollup_query NOT LIKE '%{{id}}%' THEN 'NO RECORD FILTER'
        ELSE 'REVIEW NEEDED'
    END as risk_assessment
FROM field_defs fd
WHERE fd.type = 'rollup'
ORDER BY fd.org_id, fd.entity_name;


-- ============================================
-- 4. Check for Non-Standard Entity Names
-- ============================================
-- Identifies entities with names that might cause issues with SQL

SELECT
    org_id,
    name,
    label,
    'POTENTIALLY UNSAFE NAME' as warning
FROM entity_defs
WHERE name GLOB '*[^a-zA-Z0-9_]*'
   OR name LIKE '%-%'
   OR name LIKE '% %'
   OR LENGTH(name) > 64;


-- ============================================
-- 5. Check for Non-Standard Field Names
-- ============================================

SELECT
    org_id,
    entity_name,
    name as field_name,
    type,
    'POTENTIALLY UNSAFE NAME' as warning
FROM field_defs
WHERE name GLOB '*[^a-zA-Z0-9_]*'
   OR name LIKE '%-%'
   OR name LIKE '% %'
   OR LENGTH(name) > 64;


-- ============================================
-- 6. Platform Admin Inventory
-- ============================================
-- Know who has platform admin access

SELECT
    id,
    email,
    first_name,
    last_name,
    is_active,
    last_login_at,
    created_at
FROM users
WHERE is_platform_admin = 1;


-- ============================================
-- 7. Organization Owner Inventory
-- ============================================
-- Check for orgs with single owner (risk of lockout)

SELECT
    o.id as org_id,
    o.name as org_name,
    o.slug,
    COUNT(m.id) as owner_count
FROM organizations o
LEFT JOIN user_org_memberships m ON m.org_id = o.id AND m.role = 'owner'
GROUP BY o.id, o.name, o.slug
HAVING owner_count <= 1;


-- ============================================
-- 8. API Token Inventory
-- ============================================
-- Review all active API tokens

SELECT
    t.id,
    t.org_id,
    o.name as org_name,
    t.name as token_name,
    t.token_prefix,
    t.scopes,
    t.is_active,
    t.last_used_at,
    t.expires_at,
    t.created_at,
    CASE
        WHEN t.expires_at IS NULL THEN 'NEVER EXPIRES'
        WHEN t.expires_at < datetime('now') THEN 'EXPIRED'
        ELSE 'ACTIVE'
    END as status
FROM api_tokens t
LEFT JOIN organizations o ON o.id = t.org_id
WHERE t.is_active = 1
ORDER BY t.last_used_at DESC;


-- ============================================
-- 9. Pending Invitations (potential token exposure)
-- ============================================

SELECT
    i.id,
    i.org_id,
    i.email,
    i.role,
    i.expires_at,
    i.created_at,
    CASE
        WHEN i.expires_at < datetime('now') THEN 'EXPIRED'
        WHEN i.accepted_at IS NOT NULL THEN 'ACCEPTED'
        ELSE 'PENDING'
    END as status
FROM org_invitations i
WHERE i.accepted_at IS NULL
ORDER BY i.created_at DESC;


-- ============================================
-- 10. Session Activity Summary
-- ============================================
-- Check for suspicious session patterns

SELECT
    s.user_id,
    u.email,
    COUNT(*) as session_count,
    COUNT(DISTINCT s.ip_address) as unique_ips,
    MAX(s.created_at) as last_session,
    SUM(CASE WHEN s.is_impersonation = 1 THEN 1 ELSE 0 END) as impersonation_sessions
FROM sessions s
JOIN users u ON u.id = s.user_id
GROUP BY s.user_id, u.email
HAVING session_count > 10 OR unique_ips > 5
ORDER BY session_count DESC;


-- ============================================
-- SUMMARY COUNTS
-- ============================================

SELECT 'Total Organizations' as metric, COUNT(*) as count FROM organizations
UNION ALL
SELECT 'Active Organizations', COUNT(*) FROM organizations WHERE is_active = 1
UNION ALL
SELECT 'Total Users', COUNT(*) FROM users
UNION ALL
SELECT 'Platform Admins', COUNT(*) FROM users WHERE is_platform_admin = 1
UNION ALL
SELECT 'Active API Tokens', COUNT(*) FROM api_tokens WHERE is_active = 1
UNION ALL
SELECT 'Pending Invitations', COUNT(*) FROM org_invitations WHERE accepted_at IS NULL
UNION ALL
SELECT 'Active Tripwires', COUNT(*) FROM tripwires WHERE enabled = 1
UNION ALL
SELECT 'Rollup Fields', COUNT(*) FROM field_defs WHERE type = 'rollup'
UNION ALL
SELECT 'PDF Templates', COUNT(*) FROM pdf_templates;
