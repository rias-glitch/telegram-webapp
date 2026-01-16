package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/ton"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// TonHandler handles TON-related endpoints
type TonHandler struct {
	DB             *repository.WalletRepository
	DepositRepo    *repository.DepositRepository
	WithdrawalRepo *repository.WithdrawalRepository
	ReferralRepo   *repository.ReferralRepository
	UserRepo       *repository.UserRepository
	TonClient      *ton.Client
	PlatformWallet string
	AllowedDomain  string
	MainDB         *Handler
}

// NewTonHandler creates a new TON handler
func NewTonHandler(h *Handler) *TonHandler {
	network := ton.NetworkMainnet
	if os.Getenv("TON_NETWORK") == "testnet" {
		network = ton.NetworkTestnet
	}

	return &TonHandler{
		DB:             repository.NewWalletRepository(h.DB),
		DepositRepo:    repository.NewDepositRepository(h.DB),
		WithdrawalRepo: repository.NewWithdrawalRepository(h.DB),
		ReferralRepo:   repository.NewReferralRepository(h.DB),
		UserRepo:       repository.NewUserRepository(h.DB),
		TonClient:      ton.NewClient(network, os.Getenv("TON_API_KEY")),
		PlatformWallet: os.Getenv("TON_PLATFORM_WALLET"),
		AllowedDomain:  os.Getenv("TON_ALLOWED_DOMAIN"),
		MainDB:         h,
	}
}

// ConnectWalletRequest represents wallet connection request
type ConnectWalletRequest struct {
	Account ton.WalletAccount `json:"account"`
	Proof   ton.ConnectProof  `json:"proof"`
}

// ConnectWallet links a TON wallet to user account
func (h *TonHandler) ConnectWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ConnectWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	// Check if user already has a wallet
	existing, err := h.DB.GetByUserID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wallet already linked"})
		return
	}

	// Validate address format
	if !ton.ValidateAddress(req.Account.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wallet address"})
		return
	}

	// Check if address is already linked to another user
	addressExists, err := h.DB.AddressExists(ctx, req.Account.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if addressExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wallet already linked to another account"})
		return
	}

	// Verify TON Connect proof (skip in dev mode)
	isVerified := false
	if os.Getenv("DEV_MODE") != "true" && h.AllowedDomain != "" {
		if err := ton.VerifyProof(req.Account, req.Proof, h.AllowedDomain); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "proof verification failed: " + err.Error()})
			return
		}
		isVerified = true
	} else {
		// In dev mode, trust the connection
		isVerified = true
	}

	// Normalize address
	rawAddress, _ := ton.NormalizeAddress(req.Account.Address)

	// Create wallet record
	wallet := &domain.Wallet{
		UserID:             userID,
		Address:            req.Account.Address,
		RawAddress:         rawAddress,
		IsVerified:         isVerified,
		LastProofTimestamp: req.Proof.Timestamp,
	}

	if err := h.DB.Create(ctx, wallet); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"wallet": wallet,
	})
}

// GetWallet returns user's linked wallet
func (h *TonHandler) GetWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()
	wallet, err := h.DB.GetByUserID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if wallet == nil {
		c.JSON(http.StatusOK, gin.H{"wallet": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"wallet": wallet})
}

// DisconnectWallet removes wallet link
func (h *TonHandler) DisconnectWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()

	// Check for pending withdrawals
	hasPending, err := h.WithdrawalRepo.HasPendingWithdrawal(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if hasPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disconnect wallet with pending withdrawals"})
		return
	}

	if err := h.DB.Delete(ctx, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetDepositInfo returns info for making a deposit
func (h *TonHandler) GetDepositInfo(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Check if platform wallet is configured
	if h.PlatformWallet == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "platform wallet not configured"})
		return
	}

	// Generate unique memo for this user
	memo := fmt.Sprintf("deposit_%d", userID)

	c.JSON(http.StatusOK, domain.DepositInfo{
		PlatformAddress: h.PlatformWallet,
		Memo:            memo,
		MinAmountTON:    fmt.Sprintf("%.2f", ton.NanoToTON(ton.MinDepositNano)),
		ExchangeRate:    ton.CoinsPerTON, // 10 coins per TON
	})
}

// GetDeposits returns user's deposit history
func (h *TonHandler) GetDeposits(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()
	deposits, err := h.DepositRepo.GetByUserID(ctx, userID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deposits": deposits})
}

// WithdrawRequest represents withdrawal request (coins only - premium currency)
type WithdrawRequestBody struct {
	CoinsAmount int64 `json:"coins_amount" binding:"required,min=10"` // Minimum 10 coins (1 TON)
}

// RequestWithdrawal creates a new withdrawal request (coins only)
func (h *TonHandler) RequestWithdrawal(c *gin.Context, db *pgx.Conn) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req WithdrawRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Check user has linked wallet
	wallet, err := h.DB.GetByUserID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if wallet == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no wallet linked"})
		return
	}

	// Check for existing pending withdrawal
	hasPending, err := h.WithdrawalRepo.HasPendingWithdrawal(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if hasPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you already have a pending withdrawal"})
		return
	}

	// Check daily limit (in coins)
	todayTotal, err := h.WithdrawalRepo.GetTotalCoinsWithdrawnToday(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if todayTotal+req.CoinsAmount > ton.MaxWithdrawCoinsPerDay {
		remaining := ton.MaxWithdrawCoinsPerDay - todayTotal
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "daily withdrawal limit exceeded",
			"remaining_today": remaining,
		})
		return
	}

	// Calculate amounts (5% fee)
	feeCoins := ton.CalculateWithdrawFeeCoins(req.CoinsAmount)
	netCoins := ton.CalculateWithdrawNetCoins(req.CoinsAmount)
	tonAmountNano := ton.CoinsToNano(netCoins)

	// Create withdrawal (balance check and deduction should be done in a service/transaction)
	withdrawal := &domain.Withdrawal{
		UserID:        userID,
		WalletAddress: wallet.Address,
		CoinsAmount:   req.CoinsAmount,
		TonAmountNano: tonAmountNano,
		FeeCoins:      feeCoins,
		ExchangeRate:  ton.CoinsPerTON,
		Status:        domain.WithdrawalStatusPending,
	}

	if err := h.WithdrawalRepo.Create(ctx, withdrawal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create withdrawal"})
		return
	}

	// Give 50% of fee to referrer (if user was referred)
	referrerID, err := h.ReferralRepo.GetReferrerID(ctx, userID)
	if err == nil && referrerID > 0 {
		// 50% of fee goes to referrer
		referrerCommission := feeCoins / 2
		if referrerCommission > 0 {
			// Add coins to referrer
			_, _ = h.UserRepo.UpdateCoins(ctx, referrerID, referrerCommission)
			// Track referral earnings
			_ = h.UserRepo.AddReferralEarnings(ctx, referrerID, referrerCommission)

			// Record transaction for referrer
			meta := map[string]interface{}{
				"type":           "referral_commission",
				"from_user_id":   userID,
				"withdrawal_id":  withdrawal.ID,
				"total_fee":      feeCoins,
				"commission_pct": 50,
			}
			metaB, _ := json.Marshal(meta)
			_, _ = h.MainDB.DB.Exec(ctx,
				`INSERT INTO transactions (user_id, type, amount, meta) VALUES ($1, $2, $3, $4)`,
				referrerID, "referral_commission", referrerCommission, metaB)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"withdrawal": withdrawal,
		"estimate": domain.WithdrawEstimate{
			CoinsAmount:   req.CoinsAmount,
			FeeCoins:      feeCoins,
			NetCoins:      netCoins,
			TonAmount:     fmt.Sprintf("%.4f", ton.NanoToTON(tonAmountNano)),
			TonAmountNano: tonAmountNano,
			ExchangeRate:  ton.CoinsPerTON,
			FeePercent:    0, // No longer percentage-based, using fixed fee
			FeeTON:        ton.CoinsToTON(feeCoins),
		},
	})
}

// GetWithdrawEstimate returns estimate for a withdrawal (coins only)
func (h *TonHandler) GetWithdrawEstimate(c *gin.Context) {
	var req WithdrawRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.CoinsAmount < ton.MinWithdrawCoins {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "minimum withdrawal is 10 coins (1 TON)",
			"min_coins":  ton.MinWithdrawCoins,
			"min_ton":    fmt.Sprintf("%.2f", ton.NanoToTON(ton.CoinsToNano(ton.MinWithdrawCoins))),
		})
		return
	}

	feeCoins := ton.CalculateWithdrawFeeCoins(req.CoinsAmount)
	netCoins := ton.CalculateWithdrawNetCoins(req.CoinsAmount)
	tonAmountNano := ton.CoinsToNano(netCoins)

	c.JSON(http.StatusOK, domain.WithdrawEstimate{
		CoinsAmount:   req.CoinsAmount,
		FeeCoins:      feeCoins,
		NetCoins:      netCoins,
		TonAmount:     fmt.Sprintf("%.4f", ton.NanoToTON(tonAmountNano)),
		TonAmountNano: tonAmountNano,
		ExchangeRate:  ton.CoinsPerTON,
		FeePercent:    0, // No longer percentage-based, using fixed fee
		FeeTON:        ton.CoinsToTON(feeCoins),
	})
}

// GetWithdrawals returns user's withdrawal history
func (h *TonHandler) GetWithdrawals(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()
	withdrawals, err := h.WithdrawalRepo.GetByUserID(ctx, userID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"withdrawals": withdrawals})
}

// CancelWithdrawal cancels a pending withdrawal
func (h *TonHandler) CancelWithdrawal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		WithdrawalID int64 `json:"withdrawal_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	if err := h.WithdrawalRepo.Cancel(ctx, req.WithdrawalID, userID); err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "withdrawal not found or not cancelable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel withdrawal"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetTonConfig returns TON configuration for frontend
func (h *TonHandler) GetTonConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"platform_wallet":            h.PlatformWallet,
		"coins_per_ton":              ton.CoinsPerTON, // 10 coins = 1 TON
		"min_deposit_ton":            fmt.Sprintf("%.2f", ton.NanoToTON(ton.MinDepositNano)),
		"min_withdraw_coins":         ton.MinWithdrawCoins,
		"withdraw_fee_coins":         ton.WithdrawFeeCoinsFixed, // 1 coin = 0.1 TON
		"withdraw_fee_ton":           ton.CoinsToTON(ton.WithdrawFeeCoinsFixed), // 0.1 TON
		"withdraw_fee_percent":       0, // No longer percentage-based
		"max_withdraw_coins_per_day": ton.MaxWithdrawCoinsPerDay,
		"network":                    os.Getenv("TON_NETWORK"),
	})
}

// RecordManualDeposit records a deposit manually (for testing or admin)
func (h *TonHandler) RecordManualDeposit(c *gin.Context, handler *Handler) {
	// This should be admin-only in production
	if os.Getenv("DEV_MODE") != "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		AmountTON float64 `json:"amount_ton" binding:"required,min=0.1"`
		TxHash    string  `json:"tx_hash" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	// Check if tx already processed
	exists, err := h.DepositRepo.TxHashExists(ctx, req.TxHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction already processed"})
		return
	}

	// Get user's wallet
	wallet, err := h.DB.GetByUserID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	walletAddr := ""
	if wallet != nil {
		walletAddr = wallet.Address
	}

	amountNano := ton.TONToNano(req.AmountTON)
	coinsCredited := ton.NanoToCoins(amountNano)

	deposit := &domain.Deposit{
		UserID:        userID,
		WalletAddress: walletAddr,
		AmountNano:    amountNano,
		CoinsCredited: coinsCredited,
		ExchangeRate:  ton.CoinsPerTON,
		TxHash:        req.TxHash,
		Status:        domain.DepositStatusConfirmed,
		Processed:     true,
	}

	if err := h.DepositRepo.Create(ctx, deposit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create deposit"})
		return
	}

	// Credit coins to user
	_, err = handler.DB.Exec(ctx, `UPDATE users SET coins = coins + $1 WHERE id = $2`, coinsCredited, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to credit coins"})
		return
	}

	// Record transaction
	meta := map[string]interface{}{
		"deposit_id":     deposit.ID,
		"tx_hash":        req.TxHash,
		"ton_amount":     req.AmountTON,
		"coins_credited": coinsCredited,
	}
	metaB, _ := json.Marshal(meta)
	_, _ = handler.DB.Exec(ctx, `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`,
		userID, "ton_deposit", coinsCredited, metaB)

	c.JSON(http.StatusOK, gin.H{
		"deposit":        deposit,
		"coins_credited": coinsCredited,
	})
}
