package domain

import "time"

type User struct {
	ID              int64     `db:"id" json:"id"`
	TgID            int64     `db:"tg_id" json:"tg_id"`
	Username        string    `db:"username" json:"username"`
	FirstName       string    `db:"first_name" json:"first_name"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	Gems            int64     `db:"gems" json:"gems"`                         // Free currency (earned in PvE, bonuses)
	Coins           int64     `db:"coins" json:"coins"`                       // Premium currency (purchased with TON, 10 coins = 1 TON)
	GK              int64     `db:"gk" json:"gk"`                             // Upgrade currency (earned from referrals)
	CharacterLevel  int       `db:"character_level" json:"character_level"`   // Character upgrade level (1-10)
	ReferralEarnings int64    `db:"referral_earnings" json:"referral_earnings"` // Total earned from referral commissions
}

// Currency types for games
type Currency string

const (
	CurrencyGems  Currency = "gems"
	CurrencyCoins Currency = "coins"
)

// Exchange rates
const (
	CoinsPerTON       = 10    // 10 coins = 1 TON
	WithdrawalFeePct  = 5     // 5% fee on withdrawals
	MinWithdrawCoins  = 10    // Minimum 10 coins (1 TON) to withdraw
)
