-- Migration: Rename 'member' role to 'user'
-- This standardizes the role naming convention:
-- - owner: Full org control
-- - admin: Access to Setup and admin features
-- - user: Access to CRM objects only

-- Update existing memberships
UPDATE user_org_memberships SET role = 'user' WHERE role = 'member';

-- Update existing invitations
UPDATE org_invitations SET role = 'user' WHERE role = 'member';
