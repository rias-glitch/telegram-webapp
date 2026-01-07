package handlers

import (
	"net/http"
	"strconv"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Profile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	repo := repository.NewUserRepository(h.DB)
	ctx := c.Request.Context()
	user, err := repo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Get user stats for the last month
	since := time.Now().AddDate(0, -1, 0)
	stats, _ := h.GameHistoryRepo.GetUserStats(ctx, id, since)

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"tg_id":      user.TgID,
		"username":   user.Username,
		"first_name": user.FirstName,
		"created_at": user.CreatedAt,
		"gems":       user.Gems,
		"stats":      stats,
	})
}

func (h *Handler) MyGames(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()

	// Get game history
	games, err := h.GameHistoryRepo.GetByUser(ctx, userID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get games"})
		return
	}

	// Get stats for the last month
	since := time.Now().AddDate(0, -1, 0)
	stats, _ := h.GameHistoryRepo.GetUserStats(ctx, userID, since)

	c.JSON(http.StatusOK, gin.H{"games": games, "stats": stats})
}

func (h *Handler) TopUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Use new GameHistoryRepo for monthly stats
	top, err := h.GameHistoryRepo.GetTopUsers(ctx, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get top users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"top": top})
}

func (h *Handler) ListTasks(c *gin.Context) {
	repo := repository.NewTaskRepository(h.DB)
	ctx := c.Request.Context()
	tasks, err := repo.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handler) CreateTask(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	task := &domain.Task{UserID: userID, Title: req.Title, Description: req.Description}
	if err := repository.NewTaskRepository(h.DB).Create(c.Request.Context(), task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": task})
}

// ClaimBonus gives 10000 gems to users with 0 balance (one-time)
func (h *Handler) ClaimBonus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()
	_, err := h.DB.Exec(ctx, `UPDATE users SET gems = gems + 10000 WHERE id = $1 AND gems < 100`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to claim bonus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Bonus claimed!"})
}

func (h *Handler) CompleteTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := repository.NewTaskRepository(h.DB).SetCompleted(c.Request.Context(), id, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update task"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
