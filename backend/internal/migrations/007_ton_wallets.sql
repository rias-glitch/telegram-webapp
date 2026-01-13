-- TON Wallet integration
-- Links user accounts to TON wallet addresses

CREATE TABLE IF NOT EXISTS wallets (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address VARCHAR(100) NOT NULL,
    -- TON addresses can be in different formats (raw, bounceable, non-bounceable)
    -- We store the raw format for consistency
    raw_address VARCHAR(100),
    linked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_verified BOOLEAN DEFAULT FALSE,
    -- For TON Connect proof verification
    last_proof_timestamp BIGINT,
    UNIQUE(user_id),
    UNIQUE(address)
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_address ON wallets(address);
