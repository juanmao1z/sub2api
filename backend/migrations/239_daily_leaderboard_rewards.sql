-- @brief Permanent daily reward receipts; balance and receipt commit in the same transaction.
CREATE TABLE IF NOT EXISTS daily_leaderboard_rewards (
    reward_date date PRIMARY KEY,
    actor_id bigint NOT NULL,
    receipt jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
