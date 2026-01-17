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
	bot              *tgbotapi.BotAPI
	adminService     *service.AdminService
	adminIDs         []int64 // Telegram user IDs who can use admin commands
	stopCh           chan struct{}
	wg               sync.WaitGroup
	log              *slog.Logger
	broadcastPending map[int64]bool // Track admins waiting to enter broadcast message
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
		bot:              bot,
		adminService:     adminService,
		adminIDs:         adminIDs,
		stopCh:           make(chan struct{}),
		log:              log,
		broadcastPending: make(map[int64]bool),
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

			// Check if admin is in broadcast mode (waiting for message)
			if b.broadcastPending[update.Message.From.ID] && !update.Message.IsCommand() {
				b.wg.Add(1)
				go func(msg *tgbotapi.Message) {
					defer b.wg.Done()
					b.executeBroadcast(msg)
				}(update.Message)
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
		response = b.handleBroadcastStart(msg.Chat.ID, msg.From.ID)

	case "users":
		response = b.handleUsers(ctx, msg.CommandArguments())

	case "usergames":
		response = b.handleUserGames(ctx, msg.CommandArguments())

	case "topusergames":
		response = b.handleTopUserGames(ctx, msg.CommandArguments())

	case "addcoins":
		response = b.handleAddCoins(ctx, msg.CommandArguments())

	case "addadmin":
		response = b.handleAddAdmin(msg.CommandArguments())

	case "referrals":
		response = b.handleReferralStats(ctx, msg.CommandArguments())

	default:
		response = "Неизвестная команда. Используйте /help для списка команд."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, response)
	reply.ParseMode = "HTML"
	reply.ReplyToMessageID = msg.MessageID

	if _, err := b.bot.Send(reply); err != nil {
		b.log.Error("error sending message", "error", err)
	}
}

func (b *AdminBot) helpMessage() string {
	return `<b>Команды администратора</b>

<b>Статистика:</b>
/stats - Статистика платформы
/top [лимит] - Топ пользователей по гемам
/games - Последние игры
/usergames &lt;@username|tg_id&gt; - Последние 10 игр пользователя
/topusergames [лимит] - Топ по победам в играх
/referrals [лимит] - Топ по рефералам

<b>Управление пользователями:</b>
/user &lt;@username|tg_id&gt; - Информация о пользователе
/users [страница] - Все пользователи
/addgems &lt;@username|tg_id&gt; &lt;сумма&gt; - Добавить гемы
/addcoins &lt;@username|tg_id&gt; &lt;сумма&gt; - Добавить коины
/setgems &lt;@username|tg_id&gt; &lt;сумма&gt; - Установить гемы
/ban &lt;@username|tg_id&gt; - Заблокировать
/unban &lt;@username|tg_id&gt; - Разблокировать

<b>Управление админами:</b>
/addadmin &lt;tg_id&gt; - Добавить админа

<b>Выводы:</b>
/withdrawals - Ожидающие выводы
/approve &lt;id&gt; [tx_hash] - Одобрить вывод
/reject &lt;id&gt; &lt;причина&gt; - Отклонить вывод

<b>Рассылка:</b>
/broadcast - Отправить сообщение всем (фото, кнопки)`
}

func (b *AdminBot) handleStats(ctx context.Context) string {
	stats, err := b.adminService.GetStats(ctx)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf(`<b>Статистика платформы</b>

<b>Пользователи:</b>
- Всего: %d
- Активных сегодня: %d
- Активных за неделю: %d

<b>Игры:</b>
- Всего сыграно: %d
- Сегодня: %d

<b>Экономика:</b>
- Всего гемов: %d
- Всего коинов: %d
- Всего поставлено (coins): %d
- Поставлено сегодня (coins): %d

<b>Куплено коинов:</b>
- Сегодня: %d
- За неделю: %d
- За месяц: %d
- Всего: %d

<b>Платежи:</b>
- Всего депозитов: %d
- Всего выведено: %d
- Ожидает вывода: %d`,
		stats.TotalUsers,
		stats.ActiveUsersToday,
		stats.ActiveUsersWeek,
		stats.TotalGamesPlayed,
		stats.GamesToday,
		stats.TotalGems,
		stats.TotalCoins,
		stats.TotalWagered,
		stats.WageredToday,
		stats.CoinsPurchasedToday,
		stats.CoinsPurchasedWeek,
		stats.CoinsPurchasedMonth,
		stats.CoinsPurchasedTotal,
		stats.TotalDeposited,
		stats.TotalWithdrawn,
		stats.PendingWithdraws,
	)
}

func (b *AdminBot) handleUser(ctx context.Context, args string) string {
	if args == "" {
		return "Использование: /user <@username|tg_id>"
	}

	user, err := b.adminService.GetUser(ctx, args)
	if err != nil {
		return fmt.Sprintf("Пользователь не найден: %v", err)
	}

	return fmt.Sprintf(`<b>Информация о пользователе</b>

- ID: %d
- Telegram ID: %d
- Username: @%s
- Имя: %s
- Гемы: %d
- Коины: %d
- Игр сыграно: %d
- Выиграно: %d
- Проиграно: %d
- Регистрация: %s`,
		user.ID,
		user.TgID,
		user.Username,
		user.FirstName,
		user.Gems,
		user.Coins,
		user.GamesPlayed,
		user.TotalWon,
		user.TotalLost,
		user.CreatedAt.Format("02.01.2006 15:04"),
	)
}

func (b *AdminBot) handleAddGems(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "Использование: /addgems <@username|tg_id> <сумма>"
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Неверный ID пользователя"
	}

	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "Неверная сумма"
	}

	newBalance, err := b.adminService.AddUserGems(ctx, userID, amount)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Добавлено %d гемов пользователю %d. Новый баланс: %d", amount, userID, newBalance)
}

func (b *AdminBot) handleSetGems(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "Использование: /setgems <@username|tg_id> <сумма>"
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Неверный ID пользователя"
	}

	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "Неверная сумма"
	}

	if err := b.adminService.SetUserGems(ctx, userID, amount); err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Установлено %d гемов пользователю %d", amount, userID)
}

func (b *AdminBot) handleBan(ctx context.Context, args string) string {
	if args == "" {
		return "Использование: /ban <@username|tg_id>"
	}

	userID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Неверный ID пользователя"
	}

	if err := b.adminService.BanUser(ctx, userID); err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Пользователь %d заблокирован", userID)
}

func (b *AdminBot) handleUnban(ctx context.Context, args string) string {
	if args == "" {
		return "Использование: /unban <@username|tg_id>"
	}

	userID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Неверный ID пользователя"
	}

	if err := b.adminService.UnbanUser(ctx, userID); err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Пользователь %d разблокирован", userID)
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
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(users) == 0 {
		return "Пользователи не найдены"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Топ %d по гемам</b>\n\n", limit))

	for i, u := range users {
		username := u.Username
		if username == "" {
			username = u.FirstName
		}
		sb.WriteString(fmt.Sprintf("%d. @%s — %d gems\n", i+1, username, u.Gems))
	}

	return sb.String()
}

func (b *AdminBot) handleRecentGames(ctx context.Context) string {
	games, err := b.adminService.GetRecentGames(ctx, 10)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(games) == 0 {
		return "Нет недавних игр"
	}

	var sb strings.Builder
	sb.WriteString("<b>Последние игры</b>\n\n")

	for _, g := range games {
		result := g["result"].(string)
		status := "[GAME]"
		if result == "win" {
			status = "[WIN]"
		} else if result == "lose" {
			status = "[LOSE]"
		}

		sb.WriteString(fmt.Sprintf("%s @%s | %s | ставка: %d | %+d\n",
			status,
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
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(withdrawals) == 0 {
		return "Нет ожидающих выводов"
	}

	var sb strings.Builder
	sb.WriteString("<b>Ожидающие выводы</b>\n\n")

	for _, w := range withdrawals {
		sb.WriteString(fmt.Sprintf("#%d | @%s\n", w.ID, w.Username))
		sb.WriteString(fmt.Sprintf("Сумма: %d coins (%s)\n", w.GemsAmount, w.TonAmount))
		sb.WriteString(fmt.Sprintf("Кошелёк: <code>%s</code>\n", w.WalletAddress))
		sb.WriteString(fmt.Sprintf("%s\n\n", w.CreatedAt.Format("02.01.2006 15:04")))
	}

	sb.WriteString("\n/approve <id> — одобрить\n/reject <id> <причина> — отклонить")

	return sb.String()
}

func (b *AdminBot) handleApproveWithdrawal(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		return "Использование: /approve <id> [tx_hash]"
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Неверный ID вывода"
	}

	txHash := ""
	if len(parts) >= 2 {
		txHash = parts[1]
	} else {
		txHash = fmt.Sprintf("manual_%d_%d", id, time.Now().Unix())
	}

	if err := b.adminService.ApproveWithdrawal(ctx, id, txHash); err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(parts) >= 2 {
		return fmt.Sprintf("Вывод #%d одобрен\nТранзакция: %s", id, txHash)
	}
	return fmt.Sprintf("Вывод #%d одобрен (ручное подтверждение)", id)
}

func (b *AdminBot) handleRejectWithdrawal(ctx context.Context, args string) string {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return "Использование: /reject <id> <причина>"
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Неверный ID вывода"
	}

	reason := parts[1]

	if err := b.adminService.RejectWithdrawal(ctx, id, reason); err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Вывод #%d отклонён. Средства возвращены.", id)
}

func (b *AdminBot) handleBroadcastStart(chatID int64, adminID int64) string {
	b.broadcastPending[adminID] = true

	return `<b>Broadcast Mode</b>

Введите сообщение для рассылки ниже.

<b>Поддерживается:</b>
- Текст с HTML разметкой
- Фото с подписью
- Кнопки (формат: [текст](url))

Отправьте /cancel для отмены.`
}

func (b *AdminBot) executeBroadcast(msg *tgbotapi.Message) {
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	// Cancel if user sends /cancel
	if msg.Text == "/cancel" {
		delete(b.broadcastPending, adminID)
		reply := tgbotapi.NewMessage(chatID, "Рассылка отменена")
		b.bot.Send(reply)
		return
	}

	delete(b.broadcastPending, adminID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	b.log.Info("starting broadcast", "admin_id", adminID)

	userIDs, err := b.adminService.GetAllUserTgIDs(ctx)
	if err != nil {
		b.log.Error("failed to get user IDs", "error", err)
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("Ошибка: %v", err))
		b.bot.Send(reply)
		return
	}

	if len(userIDs) == 0 {
		reply := tgbotapi.NewMessage(chatID, "Нет пользователей для рассылки")
		b.bot.Send(reply)
		return
	}

	// Send progress message
	progressMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Начинаю рассылку %d пользователям...", len(userIDs)))
	b.bot.Send(progressMsg)

	sent := 0
	failed := 0
	blocked := 0

	for _, tgID := range userIDs {
		var err error

		// Check if it's a photo message
		if msg.Photo != nil && len(msg.Photo) > 0 {
			// Get the largest photo
			photo := msg.Photo[len(msg.Photo)-1]
			photoMsg := tgbotapi.NewPhoto(tgID, tgbotapi.FileID(photo.FileID))
			photoMsg.Caption = msg.Caption
			photoMsg.ParseMode = "HTML"
			_, err = b.bot.Send(photoMsg)
		} else {
			// Text message
			textMsg := tgbotapi.NewMessage(tgID, msg.Text)
			textMsg.ParseMode = "HTML"
			textMsg.DisableWebPagePreview = true
			_, err = b.bot.Send(textMsg)
		}

		if err != nil {
			if strings.Contains(err.Error(), "blocked") || strings.Contains(err.Error(), "deactivated") {
				blocked++
			} else {
				b.log.Error("failed to send broadcast", "tg_id", tgID, "error", err)
			}
			failed++
		} else {
			sent++
		}

		// Rate limiting - 20 messages per second
		time.Sleep(50 * time.Millisecond)
	}

	b.log.Info("broadcast complete", "sent", sent, "failed", failed, "blocked", blocked)

	result := fmt.Sprintf(`<b>Рассылка завершена</b>

Отправлено: %d
Не доставлено: %d
Заблокировали бота: %d`, sent, failed-blocked, blocked)

	reply := tgbotapi.NewMessage(chatID, result)
	reply.ParseMode = "HTML"
	b.bot.Send(reply)
}

// handleUsers returns list of all users
func (b *AdminBot) handleUsers(ctx context.Context, args string) string {
	page := 1
	if args != "" {
		if n, err := strconv.Atoi(args); err == nil && n > 0 {
			page = n
		}
	}

	limit := 20
	offset := (page - 1) * limit

	users, total, err := b.adminService.GetAllUsers(ctx, limit, offset)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(users) == 0 {
		return "Пользователи не найдены"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Пользователи (стр. %d, всего: %d)</b>\n\n", page, total))

	for i, u := range users {
		username := u.Username
		if username == "" {
			username = u.FirstName
		}
		if username == "" {
			username = fmt.Sprintf("id:%d", u.TgID)
		}

		num := offset + i + 1
		sb.WriteString(fmt.Sprintf("%d. @%s | gems:%d | coins:%d\n", num, username, u.Gems, u.Coins))
	}

	totalPages := (total + limit - 1) / limit
	if totalPages > 1 {
		sb.WriteString(fmt.Sprintf("\nСтраница %d/%d. Используйте /users %d", page, totalPages, page+1))
	}

	return sb.String()
}

func (b *AdminBot) handleUserGames(ctx context.Context, args string) string {
	if args == "" {
		return "Использование: /usergames <@username|tg_id>"
	}

	tgID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Неверный Telegram ID"
	}

	user, err := b.adminService.GetUserByTgID(ctx, tgID)
	if err != nil {
		return fmt.Sprintf("Пользователь не найден: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Игры @%s</b>\n\n", user.Username))

	// Get gems games
	gemsGames, err := b.adminService.GetUserGamesByTgID(ctx, tgID, "gems", 10)
	if err != nil {
		sb.WriteString(fmt.Sprintf("Ошибка: %v\n", err))
	} else {
		sb.WriteString("<b>Последние 10 игр на гемы:</b>\n")
		if len(gemsGames) == 0 {
			sb.WriteString("Нет игр\n")
		} else {
			for _, g := range gemsGames {
				status := "[GAME]"
				if g.Result == "win" {
					status = "[WIN]"
				} else if g.Result == "lose" {
					status = "[LOSE]"
				}
				sb.WriteString(fmt.Sprintf("%s %s | ставка: %d | %+d\n", status, g.GameType, g.BetAmount, g.WinAmount))
			}
		}
	}

	sb.WriteString("\n")

	// Get coins games
	coinsGames, err := b.adminService.GetUserGamesByTgID(ctx, tgID, "coins", 10)
	if err != nil {
		sb.WriteString(fmt.Sprintf("Ошибка: %v\n", err))
	} else {
		sb.WriteString("<b>Последние 10 игр на коины:</b>\n")
		if len(coinsGames) == 0 {
			sb.WriteString("Нет игр\n")
		} else {
			for _, g := range coinsGames {
				status := "[GAME]"
				if g.Result == "win" {
					status = "[WIN]"
				} else if g.Result == "lose" {
					status = "[LOSE]"
				}
				sb.WriteString(fmt.Sprintf("%s %s | ставка: %d | %+d\n", status, g.GameType, g.BetAmount, g.WinAmount))
			}
		}
	}

	return sb.String()
}

func (b *AdminBot) handleTopUserGames(ctx context.Context, args string) string {
	limit := 20
	if args != "" {
		if n, err := strconv.Atoi(args); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	stats, err := b.adminService.GetTopUsersByWins(ctx, limit)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(stats) == 0 {
		return "Нет пользователей с победами"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Топ %d по победам</b>\n\n", limit))
	sb.WriteString("Игрок | Gems | Coins\n")
	sb.WriteString("─────────────────────────\n")

	for i, s := range stats {
		username := s.Username
		if username == "" {
			username = fmt.Sprintf("id:%d", s.UserID)
		}
		sb.WriteString(fmt.Sprintf("%d. @%s | %d | %d\n", i+1, username, s.GemsWins, s.CoinsWins))
	}

	return sb.String()
}

func (b *AdminBot) handleAddCoins(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "Использование: /addcoins <@username|tg_id> <сумма>"
	}

	tgID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "Неверный Telegram ID"
	}

	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "Неверная сумма"
	}

	newBalance, err := b.adminService.AddUserCoins(ctx, tgID, amount)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	return fmt.Sprintf("Добавлено %d коинов пользователю (TG: %d). Новый баланс: %d", amount, tgID, newBalance)
}

func (b *AdminBot) handleAddAdmin(args string) string {
	if args == "" {
		return "Использование: /addadmin <tg_id>"
	}

	tgID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return "Неверный Telegram ID"
	}

	if b.isAdmin(tgID) {
		return fmt.Sprintf("Пользователь %d уже админ", tgID)
	}

	b.adminIDs = append(b.adminIDs, tgID)
	b.log.Info("added new admin", "tg_id", tgID)

	return fmt.Sprintf("Добавлен админ %d\n\nЭто временно до перезапуска. Добавьте в ADMIN_TELEGRAM_IDS для постоянного доступа.", tgID)
}

// SendNotification sends a notification to a specific user
func (b *AdminBot) SendNotification(tgID int64, message string) error {
	msg := tgbotapi.NewMessage(tgID, message)
	msg.ParseMode = "HTML"
	_, err := b.bot.Send(msg)
	return err
}

// NotifyAdminsNewWithdrawal notifies all admins about a new withdrawal request
func (b *AdminBot) NotifyAdminsNewWithdrawal(ctx context.Context, withdrawalID int64) {
	w, err := b.adminService.GetWithdrawalNotification(ctx, withdrawalID)
	if err != nil {
		b.log.Error("failed to get withdrawal for notification", "error", err)
		return
	}

	message := fmt.Sprintf(`<b>Новый запрос на вывод!</b>

Пользователь: @%s (TG: %d)
Сумма: %d coins (%.4f TON)
Кошелек: <code>%s</code>

ID: #%d

/approve %d - одобрить
/reject %d причина - отклонить`,
		w.Username, w.TgID, w.CoinsAmount, w.TonAmount, w.WalletAddress, w.ID, w.ID, w.ID)

	for _, adminID := range b.adminIDs {
		msg := tgbotapi.NewMessage(adminID, message)
		msg.ParseMode = "HTML"
		if _, err := b.bot.Send(msg); err != nil {
			b.log.Error("failed to notify admin", "admin_id", adminID, "error", err)
		}
	}
}

func (b *AdminBot) handleReferralStats(ctx context.Context, args string) string {
	limit := 20
	if args != "" {
		if n, err := strconv.Atoi(args); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	stats, err := b.adminService.GetReferralStats(ctx, limit)
	if err != nil {
		return fmt.Sprintf("Ошибка: %v", err)
	}

	if len(stats) == 0 {
		return "Нет пользователей с рефералами"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Топ %d по рефералам</b>\n\n", limit))

	for i, s := range stats {
		username := s.Username
		if username == "" {
			username = s.FirstName
		}
		if username == "" {
			username = fmt.Sprintf("id:%d", s.UserID)
		}
		sb.WriteString(fmt.Sprintf("%d. @%s — %d рефералов\n", i+1, username, s.Count))
	}

	return sb.String()
}
