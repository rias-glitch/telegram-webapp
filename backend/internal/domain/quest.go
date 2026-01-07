package domain

import "time"

// QuestType - тип квеста
type QuestType string

const (
	QuestTypeDaily   QuestType = "daily"
	QuestTypeWeekly  QuestType = "weekly"
	QuestTypeOneTime QuestType = "one_time"
)

// ActionType - тип действия для квеста
type ActionType string

const (
	ActionTypePlay      ActionType = "play"
	ActionTypeWin       ActionType = "win"
	ActionTypeLose      ActionType = "lose"
	ActionTypeSpendGems ActionType = "spend_gems"
	ActionTypeEarnGems  ActionType = "earn_gems"
)

// Quest - шаблон задания
type Quest struct {
	ID          int64      `db:"id" json:"id"`
	QuestType   QuestType  `db:"quest_type" json:"quest_type"`
	Title       string     `db:"title" json:"title"`
	Description string     `db:"description" json:"description"`
	GameType    *string    `db:"game_type" json:"game_type,omitempty"` // 'rps', 'mines', 'any', NULL
	ActionType  ActionType `db:"action_type" json:"action_type"`
	TargetCount int        `db:"target_count" json:"target_count"`
	RewardGems  int64      `db:"reward_gems" json:"reward_gems"`
	IsActive    bool       `db:"is_active" json:"is_active"`
	SortOrder   int        `db:"sort_order" json:"sort_order"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// UserQuest - прогресс пользователя по заданию
type UserQuest struct {
	ID              int64      `db:"id" json:"id"`
	UserID          int64      `db:"user_id" json:"user_id"`
	QuestID         int64      `db:"quest_id" json:"quest_id"`
	CurrentCount    int        `db:"current_count" json:"current_count"`
	Completed       bool       `db:"completed" json:"completed"`
	RewardClaimed   bool       `db:"reward_claimed" json:"reward_claimed"`
	StartedAt       time.Time  `db:"started_at" json:"started_at"`
	CompletedAt     *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	RewardClaimedAt *time.Time `db:"reward_claimed_at" json:"reward_claimed_at,omitempty"`
	PeriodStart     time.Time  `db:"period_start" json:"period_start"`
}

// UserQuestWithDetails - прогресс с деталями квеста (для API ответов)
type UserQuestWithDetails struct {
	UserQuest
	Quest Quest `json:"quest"`
}

// IsExpired проверяет, истёк ли период квеста
func (uq *UserQuest) IsExpired(quest *Quest) bool {
	now := time.Now()
	switch quest.QuestType {
	case QuestTypeDaily:
		// Daily квесты сбрасываются в полночь
		return uq.PeriodStart.Day() != now.Day() ||
			uq.PeriodStart.Month() != now.Month() ||
			uq.PeriodStart.Year() != now.Year()
	case QuestTypeWeekly:
		// Weekly квесты сбрасываются через 7 дней
		return now.Sub(uq.PeriodStart) >= 7*24*time.Hour
	case QuestTypeOneTime:
		// One-time квесты никогда не истекают
		return false
	}
	return false
}

// CanClaim проверяет, можно ли забрать награду
func (uq *UserQuest) CanClaim() bool {
	return uq.Completed && !uq.RewardClaimed
}

// Progress возвращает прогресс в процентах (0-100)
func (uq *UserQuest) Progress(targetCount int) int {
	if targetCount <= 0 {
		return 100
	}
	progress := (uq.CurrentCount * 100) / targetCount
	if progress > 100 {
		return 100
	}
	return progress
}
