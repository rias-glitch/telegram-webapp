package handlers

import (
	"net/http"

	"telegram_webapp/internal/repository"

	"github.com/gin-gonic/gin"
)

// UpgradeHandler handles character upgrade requests
type UpgradeHandler struct {
	userRepo     *repository.UserRepository
	referralRepo *repository.ReferralRepository
}

// NewUpgradeHandler creates a new upgrade handler
func NewUpgradeHandler(userRepo *repository.UserRepository, referralRepo *repository.ReferralRepository) *UpgradeHandler {
	return &UpgradeHandler{
		userRepo:     userRepo,
		referralRepo: referralRepo,
	}
}

// UpgradeCosts defines GK cost for each level upgrade
var UpgradeCosts = map[int]int64{
	2:  100,   // Level 1 -> 2
	3:  250,   // Level 2 -> 3
	4:  500,   // Level 3 -> 4
	5:  1000,  // Level 4 -> 5
	6:  2000,  // Level 5 -> 6
	7:  4000,  // Level 6 -> 7
	8:  7500,  // Level 7 -> 8
	9:  12000, // Level 8 -> 9
	10: 20000, // Level 9 -> 10
}

// ReferralGKRewards defines GK rewards for number of referrals
var ReferralGKRewards = map[int]int64{
	1:  50,   // 1 referral
	2:  100,  // 2 referrals
	3:  200,  // 3 referrals
	4:  350,  // 4 referrals
	5:  500,  // 5 referrals
	10: 1500, // 10 referrals
	25: 5000, // 25 referrals
	50: 12000, // 50 referrals
	100: 30000, // 100 referrals
}

// GetUpgradeInfo returns upgrade system information
func (h *UpgradeHandler) GetUpgradeInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"costs":            UpgradeCosts,
		"referral_rewards": ReferralGKRewards,
		"max_level":        10,
	})
}

// GetMyUpgradeStatus returns user's current upgrade status
func (h *UpgradeHandler) GetMyUpgradeStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get user data
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// Get referral stats
	stats, err := h.referralRepo.GetReferralStats(c.Request.Context(), userID)
	if err != nil {
		stats = &repository.ReferralStats{}
	}

	// Calculate next level cost
	var nextLevelCost int64
	if user.CharacterLevel < 10 {
		nextLevelCost = UpgradeCosts[user.CharacterLevel+1]
	}

	// Calculate claimed and unclaimed GK rewards
	claimedRewards, err := h.referralRepo.GetClaimedGKRewards(c.Request.Context(), userID)
	if err != nil {
		claimedRewards = []int{}
	}

	// Find available rewards to claim
	availableRewards := []struct {
		Threshold int   `json:"threshold"`
		Reward    int64 `json:"reward"`
	}{}

	for threshold, reward := range ReferralGKRewards {
		if stats.TotalReferrals >= threshold {
			claimed := false
			for _, c := range claimedRewards {
				if c == threshold {
					claimed = true
					break
				}
			}
			if !claimed {
				availableRewards = append(availableRewards, struct {
					Threshold int   `json:"threshold"`
					Reward    int64 `json:"reward"`
				}{Threshold: threshold, Reward: reward})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"gk":                user.GK,
		"character_level":   user.CharacterLevel,
		"next_level_cost":   nextLevelCost,
		"total_referrals":   stats.TotalReferrals,
		"referral_earnings": user.ReferralEarnings,
		"available_rewards": availableRewards,
		"costs":             UpgradeCosts,
		"referral_rewards":  ReferralGKRewards,
	})
}

type UpgradeRequest struct {
	TargetLevel int `json:"target_level" binding:"required,min=2,max=10"`
}

// UpgradeCharacter upgrades character to next level
func (h *UpgradeHandler) UpgradeCharacter(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Get current level
	currentLevel, err := h.userRepo.GetCharacterLevel(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get level"})
		return
	}

	// Check if upgrade is valid
	if req.TargetLevel != currentLevel+1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can only upgrade to next level"})
		return
	}

	// Get cost
	cost, ok := UpgradeCosts[req.TargetLevel]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target level"})
		return
	}

	// Perform upgrade
	err = h.userRepo.UpgradeCharacter(c.Request.Context(), userID, req.TargetLevel, cost)
	if err != nil {
		if err == repository.ErrInsufficientFunds {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient GK"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade"})
		return
	}

	// Get new balance
	newGK, _ := h.userRepo.GetGK(c.Request.Context(), userID)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"new_level":       req.TargetLevel,
		"gk":              newGK,
		"next_level_cost": UpgradeCosts[req.TargetLevel+1],
	})
}

type ClaimRewardRequest struct {
	Threshold int `json:"threshold" binding:"required"`
}

// ClaimReferralReward claims GK reward for reaching referral threshold
func (h *UpgradeHandler) ClaimReferralReward(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ClaimRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Check if threshold has reward
	reward, ok := ReferralGKRewards[req.Threshold]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threshold"})
		return
	}

	// Get referral stats
	stats, err := h.referralRepo.GetReferralStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	// Check if user has enough referrals
	if stats.TotalReferrals < req.Threshold {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not enough referrals"})
		return
	}

	// Check if already claimed
	claimed, err := h.referralRepo.IsGKRewardClaimed(c.Request.Context(), userID, req.Threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check claim status"})
		return
	}
	if claimed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reward already claimed"})
		return
	}

	// Claim reward
	err = h.referralRepo.ClaimGKReward(c.Request.Context(), userID, req.Threshold, reward)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to claim reward"})
		return
	}

	// Get new GK balance
	newGK, _ := h.userRepo.GetGK(c.Request.Context(), userID)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"reward":    reward,
		"gk":        newGK,
		"threshold": req.Threshold,
	})
}
