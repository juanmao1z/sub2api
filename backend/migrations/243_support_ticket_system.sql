-- Local first-party ticketing workflow.
-- This migration intentionally uses the existing users and payment_orders tables,
-- keeps conversation data local, and leaves room for assignment, tags, SLA and audit.

CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject VARCHAR(160) NOT NULL,
    description TEXT NOT NULL,
    contact VARCHAR(200) NOT NULL DEFAULT '',
    category VARCHAR(32) NOT NULL DEFAULT 'api',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    assigned_to BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    client_id VARCHAR(100) NULL,
    first_response_due_at TIMESTAMPTZ NULL,
    resolution_due_at TIMESTAMPTZ NULL,
    first_response_at TIMESTAMPTZ NULL,
    resolved_at TIMESTAMPTZ NULL,
    closed_at TIMESTAMPTZ NULL,
    last_message_id BIGINT NULL,
    last_message_preview TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT support_tickets_category_ck CHECK (category IN ('billing', 'api', 'account', 'other')),
    CONSTRAINT support_tickets_priority_ck CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT support_tickets_status_ck CHECK (status IN ('open', 'in_progress', 'waiting_user', 'resolved', 'closed'))
);

CREATE INDEX IF NOT EXISTS support_tickets_user_updated_idx ON support_tickets (user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS support_tickets_queue_idx ON support_tickets (status, priority, updated_at DESC);
CREATE INDEX IF NOT EXISTS support_tickets_assignment_idx ON support_tickets (assigned_to, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS support_tickets_order_idx ON support_tickets (order_id);
CREATE UNIQUE INDEX IF NOT EXISTS support_tickets_user_client_idx ON support_tickets (user_id, client_id) WHERE client_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    sender_type VARCHAR(16) NOT NULL DEFAULT 'user',
    kind VARCHAR(16) NOT NULL DEFAULT 'message',
    body TEXT NOT NULL,
    client_id VARCHAR(100) NULL,
    event_type VARCHAR(64) NOT NULL DEFAULT '',
    event_data JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT support_ticket_messages_sender_type_ck CHECK (sender_type IN ('user', 'admin', 'system')),
    CONSTRAINT support_ticket_messages_kind_ck CHECK (kind IN ('message', 'event'))
);

CREATE INDEX IF NOT EXISTS support_ticket_messages_ticket_id_idx ON support_ticket_messages (ticket_id, id);
CREATE INDEX IF NOT EXISTS support_ticket_messages_created_at_idx ON support_ticket_messages (created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS support_ticket_messages_client_idx ON support_ticket_messages (ticket_id, sender_id, client_id) WHERE client_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS support_ticket_reads (
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_message_id BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (ticket_id, user_id)
);

CREATE TABLE IF NOT EXISTS support_ticket_tags (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    slug VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS support_ticket_tag_links (
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES support_ticket_tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (ticket_id, tag_id)
);

CREATE INDEX IF NOT EXISTS support_ticket_tag_links_tag_idx ON support_ticket_tag_links (tag_id, ticket_id);

INSERT INTO support_ticket_tags (name, slug)
VALUES
    ('Billing', 'billing'),
    ('API', 'api'),
    ('Account', 'account'),
    ('Bug', 'bug'),
    ('Feature request', 'feature-request'),
    ('Refund', 'refund')
ON CONFLICT (slug) DO NOTHING;
