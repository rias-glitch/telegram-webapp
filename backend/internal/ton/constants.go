package ton

import "time"

const (
	// CoinsPerTON is the exchange rate: how many coins for 1 TON
	// 1 TON = 10 coins (premium currency)
	CoinsPerTON = 10

	// GemsPerTON is kept for backward compatibility (free currency, not withdrawable)
	// 1 TON = 10000 gems (for reference only, gems are not purchasable with TON)
	GemsPerTON = 10000

	// NanoTON is the smallest TON unit (1 TON = 10^9 nanoTON)
	NanoTON = 1_000_000_000

	// MinDepositNano is the minimum deposit amount in nanoTON (1 TON = 10 coins)
	MinDepositNano = 1_000_000_000

	// MinWithdrawCoins is the minimum withdrawal amount in coins (10 coins = 1 TON)
	MinWithdrawCoins = 10

	// WithdrawFeeCoinsFixed is the fixed platform fee on withdrawals (1 coin = 0.1 TON)
	WithdrawFeeCoinsFixed = 1

	// WithdrawFeePercent is kept for backward compatibility but not used (replaced by fixed fee)
	WithdrawFeePercent = 5

	// MaxWithdrawCoinsPerDay is the maximum withdrawal per day in coins (1000 coins = 100 TON)
	MaxWithdrawCoinsPerDay = 1000

	// DepositConfirmations is how many confirmations needed
	DepositConfirmations = 1

	// ProofTTL is how long a TON Connect proof is valid
	ProofTTL = 15 * time.Minute

	// DepositCheckInterval is how often to check for new deposits
	DepositCheckInterval = 30 * time.Second

	// WithdrawProcessInterval is how often to process pending withdrawals
	WithdrawProcessInterval = 1 * time.Minute
)

// Network represents TON network type
type Network string

const (
	NetworkMainnet Network = "mainnet"
	NetworkTestnet Network = "testnet"
)

// TON API endpoints
const (
	TonAPIMainnet = "https://tonapi.io/v2"
	TonAPITestnet = "https://testnet.tonapi.io/v2"

	TonCenterMainnet = "https://toncenter.com/api/v2"
	TonCenterTestnet = "https://testnet.toncenter.com/api/v2"
)

// TONToNano converts TON to nanoTON
func TONToNano(ton float64) int64 {
	return int64(ton * NanoTON)
}

// NanoToTON converts nanoTON to TON
func NanoToTON(nano int64) float64 {
	return float64(nano) / NanoTON
}

// NanoToGems converts nanoTON to gems using the exchange rate (legacy)
func NanoToGems(nano int64) int64 {
	ton := NanoToTON(nano)
	return int64(ton * GemsPerTON)
}

// GemsToNano converts gems to nanoTON using the exchange rate (legacy)
func GemsToNano(gems int64) int64 {
	ton := float64(gems) / GemsPerTON
	return TONToNano(ton)
}

// NanoToCoins converts nanoTON to coins (1 TON = 10 coins)
func NanoToCoins(nano int64) int64 {
	ton := NanoToTON(nano)
	return int64(ton * CoinsPerTON)
}

// CoinsToNano converts coins to nanoTON (10 coins = 1 TON)
func CoinsToNano(coins int64) int64 {
	ton := float64(coins) / CoinsPerTON
	return TONToNano(ton)
}

// CoinsToTON converts coins to TON
func CoinsToTON(coins int64) float64 {
	return float64(coins) / CoinsPerTON
}

// TONToCoins converts TON to coins
func TONToCoins(ton float64) int64 {
	return int64(ton * CoinsPerTON)
}

// CalculateWithdrawFeeCoins calculates the fee for a withdrawal in coins (fixed 0.1 TON = 1 coin)
func CalculateWithdrawFeeCoins(coinsAmount int64) int64 {
	return WithdrawFeeCoinsFixed
}

// CalculateWithdrawNetCoins calculates the net coins after fee
func CalculateWithdrawNetCoins(coinsAmount int64) int64 {
	return coinsAmount - WithdrawFeeCoinsFixed
}

// Legacy functions for backward compatibility
func CalculateWithdrawFee(gemsAmount int64) int64 {
	return gemsAmount * WithdrawFeePercent / 100
}

func CalculateWithdrawNet(gemsAmount int64) int64 {
	return gemsAmount - CalculateWithdrawFee(gemsAmount)
}
