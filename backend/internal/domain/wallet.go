package domain

import "time"

// Wallet represents a linked TON wallet
type Wallet struct {
	ID                 int64     `db:"id" json:"id"`
	UserID             int64     `db:"user_id" json:"user_id"`
	Address            string    `db:"address" json:"address"`
	RawAddress         string    `db:"raw_address" json:"raw_address,omitempty"`
	LinkedAt           time.Time `db:"linked_at" json:"linked_at"`
	IsVerified         bool      `db:"is_verified" json:"is_verified"`
	LastProofTimestamp int64     `db:"last_proof_timestamp" json:"last_proof_timestamp,omitempty"`
}

// Deposit represents an incoming TON deposit
type Deposit struct {
	ID            int64         `db:"id" json:"id"`
	UserID        int64         `db:"user_id" json:"user_id"`
	WalletAddress string        `db:"wallet_address" json:"wallet_address"`
	AmountNano    int64         `db:"amount_nano" json:"amount_nano"`
	GemsCredited  int64         `db:"gems_credited" json:"gems_credited"` // Legacy
	CoinsCredited int64         `db:"coins_credited" json:"coins_credited"` // New: coins credited (1 TON = 10 coins)
	ExchangeRate  int           `db:"exchange_rate" json:"exchange_rate"`
	TxHash        string        `db:"tx_hash" json:"tx_hash"`
	TxLt          int64         `db:"tx_lt" json:"tx_lt,omitempty"`
	Status        DepositStatus `db:"status" json:"status"`
	Memo          string        `db:"memo" json:"memo,omitempty"`
	CreatedAt     time.Time     `db:"created_at" json:"created_at"`
	ConfirmedAt   *time.Time    `db:"confirmed_at" json:"confirmed_at,omitempty"`
	Processed     bool          `db:"processed" json:"processed"`
}

// DepositStatus represents deposit processing status
type DepositStatus string

const (
	DepositStatusPending   DepositStatus = "pending"
	DepositStatusConfirmed DepositStatus = "confirmed"
	DepositStatusFailed    DepositStatus = "failed"
	DepositStatusExpired   DepositStatus = "expired"
)

// Withdrawal represents an outgoing TON withdrawal (coins only)
type Withdrawal struct {
	ID            int64            `db:"id" json:"id"`
	UserID        int64            `db:"user_id" json:"user_id"`
	WalletAddress string           `db:"wallet_address" json:"wallet_address"`
	CoinsAmount   int64            `db:"coins_amount" json:"coins_amount"` // Amount in coins
	TonAmountNano int64            `db:"ton_amount_nano" json:"ton_amount_nano"`
	FeeCoins      int64            `db:"fee_coins" json:"fee_coins"` // Fixed fee in coins (0.1 TON = 1 coin)
	ExchangeRate  int              `db:"exchange_rate" json:"exchange_rate"` // CoinsPerTON
	Status        WithdrawalStatus `db:"status" json:"status"`
	TxHash        string           `db:"tx_hash" json:"tx_hash,omitempty"`
	TxLt          int64            `db:"tx_lt" json:"tx_lt,omitempty"`
	AdminNotes    string           `db:"admin_notes" json:"admin_notes,omitempty"`
	CreatedAt     time.Time        `db:"created_at" json:"created_at"`
	ProcessedAt   *time.Time       `db:"processed_at" json:"processed_at,omitempty"`
	CompletedAt   *time.Time       `db:"completed_at" json:"completed_at,omitempty"`
	// Legacy fields (kept for backward compatibility)
	GemsAmount int64 `db:"gems_amount" json:"gems_amount,omitempty"`
	FeeGems    int64 `db:"fee_gems" json:"fee_gems,omitempty"`
}

// WithdrawalStatus represents withdrawal processing status
type WithdrawalStatus string

const (
	WithdrawalStatusPending    WithdrawalStatus = "pending"
	WithdrawalStatusProcessing WithdrawalStatus = "processing"
	WithdrawalStatusSent       WithdrawalStatus = "sent"
	WithdrawalStatusCompleted  WithdrawalStatus = "completed"
	WithdrawalStatusFailed     WithdrawalStatus = "failed"
	WithdrawalStatusCancelled  WithdrawalStatus = "cancelled"
)

// DepositInfo is returned to user when they want to deposit
type DepositInfo struct {
	PlatformAddress string `json:"platform_address"`
	Memo            string `json:"memo"`
	MinAmountTON    string `json:"min_amount_ton"`
	ExchangeRate    int    `json:"exchange_rate"` // coins per TON (10)
}

// WithdrawRequest represents a withdrawal request from user
type WithdrawRequest struct {
	CoinsAmount int64 `json:"coins_amount" binding:"required,min=10"` // Minimum 10 coins (1 TON)
}

// WithdrawEstimate shows user what they'll receive
type WithdrawEstimate struct {
	CoinsAmount   int64   `json:"coins_amount"`
	FeeCoins      int64   `json:"fee_coins"`       // Fixed fee in coins
	NetCoins      int64   `json:"net_coins"`       // After fee
	TonAmount     string  `json:"ton_amount"`      // human readable
	TonAmountNano int64   `json:"ton_amount_nano"` // in nanoTON
	ExchangeRate  int     `json:"exchange_rate"`   // 10 coins per TON
	FeePercent    float64 `json:"fee_percent"`     // Legacy field (0 for fixed fee)
	FeeTON        float64 `json:"fee_ton"`         // Fixed fee in TON (0.1)
}
