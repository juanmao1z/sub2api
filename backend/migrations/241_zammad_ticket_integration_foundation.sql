-- Stage 2 foundation for Zammad-backed tickets.
-- Sub2API keeps permissions, order references, approval state and retry metadata;
-- Zammad remains the source of truth for ticket articles and conversation content.

CREATE TABLE IF NOT EXISTS support_ticket_zammad_users (
    id BIGSERIAL PRIMARY KEY,
    sub2api_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zammad_user_id BIGINT NULL,
    email TEXT NOT NULL,
    sync_state VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    conflict_reason TEXT NULL,
    last_synced_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT support_ticket_zammad_users_sub2api_user_uq UNIQUE (sub2api_user_id),
    CONSTRAINT support_ticket_zammad_users_zammad_user_uq UNIQUE (zammad_user_id)
);

CREATE INDEX IF NOT EXISTS support_ticket_zammad_users_sync_state_idx
    ON support_ticket_zammad_users (sync_state);

CREATE TABLE IF NOT EXISTS support_ticket_links (
    id BIGSERIAL PRIMARY KEY,
    local_ticket_id BIGINT NULL UNIQUE REFERENCES support_tickets(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zammad_ticket_id BIGINT NOT NULL UNIQUE,
    ticket_type VARCHAR(32) NOT NULL,
    order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    contact TEXT NULL,
    subject_cache VARCHAR(200) NOT NULL DEFAULT '',
    status_cache VARCHAR(32) NOT NULL DEFAULT 'OPEN',
    sync_state VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_synced_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS support_ticket_links_user_created_idx
    ON support_ticket_links (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS support_ticket_links_local_ticket_idx
    ON support_ticket_links (local_ticket_id);
CREATE INDEX IF NOT EXISTS support_ticket_links_sync_state_idx
    ON support_ticket_links (sync_state, updated_at);

CREATE TABLE IF NOT EXISTS support_ticket_approvals (
    id BIGSERIAL PRIMARY KEY,
    ticket_link_id BIGINT NOT NULL REFERENCES support_ticket_links(id) ON DELETE CASCADE,
    approval_state VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    approved_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ NULL,
    decision_reason TEXT NULL,
    execution_result JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT support_ticket_approvals_ticket_uq UNIQUE (ticket_link_id)
);

CREATE INDEX IF NOT EXISTS support_ticket_approvals_state_idx
    ON support_ticket_approvals (approval_state, updated_at);

CREATE TABLE IF NOT EXISTS support_ticket_sync_outbox (
    id BIGSERIAL PRIMARY KEY,
    ticket_link_id BIGINT NULL REFERENCES support_ticket_links(id) ON DELETE CASCADE,
    event_type VARCHAR(32) NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_at TIMESTAMPTZ NULL,
    locked_by TEXT NULL,
    processed_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS support_ticket_sync_outbox_claim_idx
    ON support_ticket_sync_outbox (processed_at, next_attempt_at, locked_at, locked_by, id);
