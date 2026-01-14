package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"telegram_webapp/internal/logger"
	"telegram_webapp/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// AdminBot handles admin commands via Telegram
type AdminBot struct {
	bot          *tgbotapi.BotAPI
	adminService *service.AdminService
	adminIDs     []int64 // Telegram user IDs who can use admin commands
	stopCh       chan struct{}
	wg           sync.WaitGroup
	log          *slog.Logger
}

// NewAdminBot creates a new admin bot
func NewAdminBot(token string, adminService *service.AdminService, adminIDs []int64) (*AdminBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	log := logger.With("component", "admin_bot")
	log.Info("admin bot authorized", "username", bot.Self.UserName)

	return &AdminBot{
		bot:          bot,
		adminService: adminService,
		adminIDs:     adminIDs,
		stopCh:       make(chan struct{}),
		log:          log,
	}, nil
}

// Start starts listening for commands
func (b *AdminBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)
	b.log.Info("starting bot update loop")

	for {
		select {
		case <-b.stopCh:
			b.log.Info("stopping bot update loop")
			return
		case update, ok := <-updates:
			if !ok {
				return
			}

			if update.Message == nil {
				continue
			}

			// Check if user is admin
			if !b.isAdmin(update.Message.From.ID) {
				continue
			}

			if !update.Message.IsCommand() {
				continue
			}

			b.wg.Add(1)
			go func(msg *tgbotapi.Message) {
				defer b.wg.Done()
				b.handleCommand(msg)
			}(update.Message)
		}
	}
}

// Stop gracefully stops the bot
func (b *AdminBot) Stop() {
	b.log.Info("stopping admin bot...")
	close(b.stopCh)
	b.bot.StopReceivingUpdates()

	// Wait for pending handlers with timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.log.Info("admin bot stopped gracefully")
	case <-time.After(10 * time.Second):
		b.log.Warn("admin bot shutdown timeout, some handlers may not have completed")
	}
}

// isAdmin checks if user is an admin
func (b *AdminBot) isAdmin(userID int64) bool {
	for _, id := range b.adminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// handleCommand processes admin commands
func (b *AdminBot) handleCommand(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var response string

	switch msg.Command() {
	case "start", "help":
		response = b.helpMessage()

	case "stats":
		response = b.handleStats(ctx)

	case "user":
		response = b.handleUser(ctx, msg.CommandArguments())

	case "addgems":
		response = b.handleAddGems(ctx, msg.CommandArguments())

	case "setgems":
		response = b.handleSetGems(ctx, msg.CommandArguments())

	case "ban":
		response = b.handleBan(ctx, msg.CommandArguments())

	case "unban":
		response = b.handleUnban(ctx, msg.CommandArguments())

	case "top":
		response = b.handleTop(ctx, msg.CommandArguments())

	case "games":
		response = b.handleRecentGames(ctx)

	case "withdrawals":
		response = b.handleWithdrawals(ctx)

	case "approve":
		response = b.handleApproveWithdrawal(ctx, msg.CommandArguments())

	case "reject":
		response = b.handleRejectWithdrawal(ctx, msg.CommandArguments())

	case "broadcast":
		response = b.handleBroadcast(ctx, msg.CommandArguments())

	default:
		response = "Unknown command. Use /help for available commands."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, response)
	reply.ParseMode = "HTML"
	reply.ReplyToMessageID = msg.MessageID

	if _, err := b.bot.Send(reply); err != nil {
		b.log.Error("error sending message", "error", err)
	}
}

func (b *AdminBot) helpMessage() string {
	return `<b>Admin Commands</b>

<b>Statistics:</b>
/stats - Platform statistics
/top [limit] - Top users by gems
/games - Recent games

<b>User Management:</b>
/user &lt;id|tg_id|username&gt; - User info
/addgems &lt;user_id&gt; &lt;amount&gt; - Add gems
/setgems &lt;user_id&gt; &lt;amount&gt; - Set gems
/ban &lt;user_id&gt; - Ban user
/unban &lt;user_id&gt; - Unban user

<b>Withdrawals:</b>
/withdrawals - Pending withdrawals
/approve &lt;id&gt; &lt;tx_hash&gt; - Approve withdrawal
/reject &lt;id&gt; &lt;reason&gt; - Reject withdrawal

<b>Broadcast:</b>
/broadcast &lt;message&gt; - Send to all users`
}

func (b *AdminBot) handleStats(ctx context.Context) string {
	stats, err := b.adminService.GetStats(ctx)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf(`<b>Platform Statistics</b>

<b>Users:</b>
• Total: %d
• Active today: %d
• Active this week: %d

<b>Games:</b>
• Total played: %d
• Today: %d

<b>Economy:</b>
• Total gems: %d
• Total wagered: %d
• Wagered today: %d
• House profit: %d
• Profit today: %d

<b>Payments:</b>
• Total deposited: %d gems
• Total withdrawn: %d gems
• Pending withdrawals: %d`,
		stats.TotalUsers,
		stats.ActiveUsersToday,
		stats.ActiveUsersWeek,
		stats.TotalGamesPlayed,
		stats.GamesToday,
		stats.TotalGems,
		stats.TotalWagered,
		stats.WageredToday,
		stats.HouseProfit,
		stats.ProfitToday,
		stats.TotalDeposited,
		stats.TotalWithdrawn,
		stats.PendingWithdraws,
	)
}

func (b *AdminBot) handleUser(ctx context.Context, args string) string {
	if args == "" {
		return "Usage: /user <id|tg_id|username>"
	}

	user, err := b.adminService.GetUser(ctx, args)
	if err != nil {
		return fmt.Sprintf("User not found: %v", err)
	}

	return fmt.Sprintf(`<b>User Info</b>

• ID: %d
• Telegram ID: %d
• Username: @%s
• Name: %s
• Gems: %d
• Games played: %d
• Total won: %d
• Total lost: %d
• Registered: %s`,
		user.ID,
		user.TgID,
		user.Username,
		user.FirstName,
		user.Gems,
		user.GamesPlayed,
		user.TotalWon,
		user.TotalLost,
		user.CreatedAt.Format("2006-01-02 15:04"),
	)
}

func (b *AdminBot) handleAddGems(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "Usage: /addgems <user_id> <amount>"
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Invalid user ID"
	}

	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "Invalid amount"
	}

	newBalance, err := b.adminService.AddUserGems(ctx, userID, amount)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Added %d gems to user %d. New balance: %d", amount, userID, newBalance)
}

func (b *AdminBot) handleSetGems(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "Usage: /setgems <user_id> <amount>"
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Invalid user ID"
	}

	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "Invalid amount"
	}

	if err := b.adminService.SetUserGems(ctx, userID, amount); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Set user %d gems to %d", userID, amount)
}

func (b *AdminBot) handleBan(ctx context.Context, args string) string {
	if args == "" {
		return "Usage: /ban <user_id>"
	}

	userID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Invalid user ID"
	}

	if err := b.adminService.BanUser(ctx, userID); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("User %d has been banned", userID)
}

func (b *AdminBot) handleUnban(ctx context.Context, args string) string {
	if args == "" {
		return "Usage: /unban <user_id>"
	}

	userID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Invalid user ID"
	}

	if err := b.adminService.UnbanUser(ctx, userID); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("User %d has been unbanned", userID)
}

func (b *AdminBot) handleTop(ctx context.Context, args string) string {
	limit := 10
	if args != "" {
		if n, err := strconv.Atoi(args); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	users, err := b.adminService.GetTopUsers(ctx, limit)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(users) == 0 {
		return "No users found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Top %d Users by Gems</b>\n\n", limit))

	for i, u := range users {
		username := u.Username
		if username == "" {
			username = u.FirstName
		}
		sb.WriteString(fmt.Sprintf("%d. @%s - %d gems\n", i+1, username, u.Gems))
	}

	return sb.String()
}

func (b *AdminBot) handleRecentGames(ctx context.Context) string {
	games, err := b.adminService.GetRecentGames(ctx, 10)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(games) == 0 {
		return "No recent games"
	}

	var sb strings.Builder
	sb.WriteString("<b>Recent Games</b>\n\n")

	for _, g := range games {
		result := g["result"].(string)
		emoji := "🎮"
		if result == "win" {
			emoji = "✅"
		} else if result == "lose" {
			emoji = "❌"
		}

		sb.WriteString(fmt.Sprintf("%s @%s | %s | bet: %d | %+d\n",
			emoji,
			g["username"],
			g["game_type"],
			g["bet_amount"],
			g["win_amount"],
		))
	}

	return sb.String()
}

func (b *AdminBot) handleWithdrawals(ctx context.Context) string {
	withdrawals, err := b.adminService.GetPendingWithdrawals(ctx)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(withdrawals) == 0 {
		return "No pending withdrawals"
	}

	var sb strings.Builder
	sb.WriteString("<b>Pending Withdrawals</b>\n\n")

	for _, w := range withdrawals {
		sb.WriteString(fmt.Sprintf("ID: %d | @%s\n", w.ID, w.Username))
		sb.WriteString(fmt.Sprintf("Amount: %d gems (%s)\n", w.GemsAmount, w.TonAmount))
		sb.WriteString(fmt.Sprintf("Wallet: <code>%s</code>\n", w.WalletAddress))
		sb.WriteString(fmt.Sprintf("Status: %s | %s\n\n", w.Status, w.CreatedAt.Format("01-02 15:04")))
	}

	sb.WriteString("\nUse /approve <id> <tx_hash> or /reject <id> <reason>")

	return sb.String()
}

func (b *AdminBot) handleApproveWithdrawal(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "Usage: /approve <withdrawal_id> <tx_hash>"
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Invalid withdrawal ID"
	}

	txHash := parts[1]

	if err := b.adminService.ApproveWithdrawal(ctx, id, txHash); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Withdrawal #%d approved with tx: %s", id, txHash)
}

func (b *AdminBot) handleRejectWithdrawal(ctx context.Context, args string) string {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return "Usage: /reject <withdrawal_id> <reason>"
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Invalid withdrawal ID"
	}

	reason := parts[1]

	if err := b.adminService.RejectWithdrawal(ctx, id, reason); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Withdrawal #%d rejected. Gems refunded.", id)
}

func (b *AdminBot) handleBroadcast(ctx context.Context, message string) string {
	if message == "" {
		return "Usage: /broadcast <message>"
	}

	b.log.Info("starting broadcast", "message", message)

	userIDs, err := b.adminService.GetAllUserTgIDs(ctx)
	if err != nil {
		b.log.Error("failed to get user IDs", "error", err)
		return fmt.Sprintf("Error getting users: %v", err)
	}

	b.log.Info("found users for broadcast", "count", len(userIDs))

	if len(userIDs) == 0 {
		return "No users found to broadcast to"
	}

	sent := 0
	failed := 0

	for _, tgID := range userIDs {
		msg := tgbotapi.NewMessage(tgID, message)
		msg.ParseMode = "HTML"

		if _, err := b.bot.Send(msg); err != nil {
			b.log.Error("failed to send broadcast message", "tg_id", tgID, "error", err)
			failed++
		} else {
			sent++
		}

		// Rate limiting
		time.Sleep(50 * time.Millisecond)
	}

	b.log.Info("broadcast complete", "sent", sent, "failed", failed)
	return fmt.Sprintf("Broadcast complete. Sent: %d, Failed: %d", sent, failed)
}

// SendNotification sends a notification to a specific user
func (b *AdminBot) SendNotification(tgID int64, message string) error {
	msg := tgbotapi.NewMessage(tgID, message)
	msg.ParseMode = "HTML"
	_, err := b.bot.Send(msg)
	return err
}
