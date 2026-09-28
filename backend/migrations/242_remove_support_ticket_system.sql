-- Remove the retired built-in ticket system and its Zammad integration metadata.
-- Historical migrations remain immutable; this migration is the single cleanup step
-- for existing databases and leaves no ticket tables behind after deployment.
DROP TABLE IF EXISTS support_ticket_sync_outbox CASCADE;
DROP TABLE IF EXISTS support_ticket_approvals CASCADE;
DROP TABLE IF EXISTS support_ticket_links CASCADE;
DROP TABLE IF EXISTS support_ticket_zammad_users CASCADE;
DROP TABLE IF EXISTS support_ticket_messages CASCADE;
DROP TABLE IF EXISTS support_tickets CASCADE;
