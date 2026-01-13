-- TON Deposits
-- Tracks incoming TON payments and gems credited

CREATE TABLE IF NOT EXISTS deposits (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_address VARCHAR(100) NOT NULL,

    -- Amount in nanoTON (1 TON = 10^9 nanoTON)
    amount_nano BIGINT NOT NULL,

    -- Gems credited to user account
    gems_credited INT NOT NULL,

    -- Exchange rate at time of deposit (gems per TON)
    exchange_rate INT NOT NULL,

    -- TON blockchain transaction details
    tx_hash VARCHAR(100) UNIQUE NOT NULL,
    tx_lt BIGINT,  -- Logical time

    -- Deposit status
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'failed', 'expired')),

    -- Optional memo for identifying deposits
    memo VARCHAR(100),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    confirmed_at TIMESTAMP WITH TIME ZONE,

    -- For idempotency - prevent double crediting
    processed BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_deposits_user_id ON deposits(user_id);
CREATE INDEX IF NOT EXISTS idx_deposits_tx_hash ON deposits(tx_hash);
CREATE INDEX IF NOT EXISTS idx_deposits_status ON deposits(status);
CREATE INDEX IF NOT EXISTS idx_deposits_wallet ON deposits(wallet_address);
