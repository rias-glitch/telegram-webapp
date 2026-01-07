package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"telegram_webapp/internal/domain"

	"github.com/gin-gonic/gin"
)

// GetQuests возвращает все активные квесты
func (h *Handler) GetQuests(c *gin.Context) {
	ctx := c.Request.Context()
	quests, err := h.QuestRepo.GetActiveQuests(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get quests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"quests": quests})
}

// QuestWithProgress - квест с прогрессом пользователя
type QuestWithProgress struct {
	Quest         *domain.Quest `json:"quest"`
	CurrentCount  int           `json:"current_count"`
	TargetCount   int           `json:"target_count"`
	Completed     bool          `json:"completed"`
	RewardClaimed bool          `json:"reward_claimed"`
	Progress      int           `json:"progress"`
	UserQuestID   *int64        `json:"user_quest_id,omitempty"`
}

// GetMyQuests возвращает квесты пользователя с прогрессом
func (h *Handler) GetMyQuests(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()

	// Получаем все активные квесты
	allQuests, err := h.QuestRepo.GetActiveQuests(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get quests"})
		return
	}

	// Получаем прогресс пользователя
	userQuests, err := h.QuestRepo.GetUserQuests(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user quests"})
		return
	}

	// Создаём map для быстрого поиска прогресса
	progressMap := make(map[int64]*domain.UserQuestWithDetails)
	for _, uq := range userQuests {
		progressMap[uq.QuestID] = uq
	}

	// Формируем ответ с прогрессом для каждого квеста
	var result []QuestWithProgress
	for _, q := range allQuests {
		qwp := QuestWithProgress{
			Quest:       q,
			TargetCount: q.TargetCount,
		}

		if uq, exists := progressMap[q.ID]; exists {
			qwp.CurrentCount = uq.CurrentCount
			qwp.Completed = uq.Completed
			qwp.RewardClaimed = uq.RewardClaimed
			qwp.UserQuestID = &uq.ID
			qwp.Progress = uq.Progress(q.TargetCount)
		}

		result = append(result, qwp)
	}

	c.JSON(http.StatusOK, gin.H{"quests": result})
}

// ClaimQuestReward забирает награду за выполненный квест
func (h *Handler) ClaimQuestReward(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	userQuestIDStr := c.Param("id")
	userQuestID, err := strconv.ParseInt(userQuestIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quest id"})
		return
	}

	ctx := c.Request.Context()

	// Забираем награду
	rewardGems, err := h.QuestRepo.ClaimReward(ctx, userQuestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot claim reward"})
		return
	}

	// Начисляем гемы пользователю
	var newBalance int64
	err = h.DB.QueryRow(ctx,
		`UPDATE users SET gems = gems + $1 WHERE id = $2 RETURNING gems`,
		rewardGems, userID,
	).Scan(&newBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update balance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reward": rewardGems,
		"gems":   newBalance,
	})
}

// updateQuestsAfterGame вызывается после каждой игры для обновления прогресса квестов
func (h *Handler) updateQuestsAfterGame(userID int64, gameType string, result string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Получаем все активные квесты
	quests, err := h.QuestRepo.GetActiveQuests(ctx)
	if err != nil {
		return
	}

	for _, quest := range quests {
		// Проверяем соответствие типа игры
		if quest.GameType != nil && *quest.GameType != "any" && *quest.GameType != gameType {
			continue
		}

		// Проверяем тип действия
		shouldIncrement := false
		switch quest.ActionType {
		case domain.ActionTypePlay:
			shouldIncrement = true
		case domain.ActionTypeWin:
			shouldIncrement = (result == "win")
		case domain.ActionTypeLose:
			shouldIncrement = (result == "lose")
		}

		if shouldIncrement {
			_ = h.QuestRepo.IncrementProgress(ctx, userID, quest, 1)
		}
	}
}
