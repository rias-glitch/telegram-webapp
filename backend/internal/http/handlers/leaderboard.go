package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetLeaderboard returns the monthly top 100 users
func (h *Handler) GetLeaderboard(c *gin.Context) {
	top, err := h.UserRepo.GetMonthlyTop(c.Request.Context(), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get leaderboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": top,
		"period":      "monthly",
	})
}

// GetMyRank returns the current user's rank in the monthly leaderboard
func (h *Handler) GetMyRank(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rank, wonAmount, err := h.UserRepo.GetUserRank(c.Request.Context(), userID)
	if err != nil {
		// If no games played, rank is 0
		c.JSON(http.StatusOK, gin.H{
			"rank":       0,
			"won_amount": 0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rank":       rank,
		"won_amount": wonAmount,
	})
}
