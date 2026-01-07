package handlers

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// MyProfile returns current user's profile including gems
func (h *Handler) MyProfile(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    repo := repository.NewUserRepository(h.DB)
    ctx := c.Request.Context()
    user, err := repo.GetByID(ctx, userID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }

    // fetch transactions
    rows, _ := h.DB.Query(ctx, `SELECT type, amount, meta, created_at FROM transactions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
    var history []map[string]interface{}
    if rows != nil {
        defer rows.Close()
        for rows.Next() {
            var tType string
            var amount int64
            var metaBytes []byte
            var createdAt time.Time
            _ = rows.Scan(&tType, &amount, &metaBytes, &createdAt)
            var meta interface{}
            _ = json.Unmarshal(metaBytes, &meta)
            history = append(history, map[string]interface{}{"type": tType, "amount": amount, "meta": meta, "date": createdAt})
        }
    }

    c.JSON(http.StatusOK, gin.H{"id": user.ID, "tg_id": user.TgID, "username": user.Username, "first_name": user.FirstName, "created_at": user.CreatedAt, "gems": user.Gems, "history": history})
}

// UpdateBalance adjusts user's gems balance by delta (can be negative)
func (h *Handler) UpdateBalance(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    var req struct{ Delta int64 `json:"delta"` }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
        return
    }

    // Use transaction to update and return new balance
    tx, err := h.DB.BeginTx(context.Background(), pgx.TxOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer func() { _ = tx.Rollback(context.Background()) }()

    var newGems int64
    // ensure non-negative balance
    if req.Delta < 0 {
        // attempt atomic deduction only if enough balance
        row := tx.QueryRow(context.Background(), `UPDATE users SET gems = gems + $1 WHERE id=$2 AND gems + $1 >= 0 RETURNING gems`, req.Delta, userID)
        if err := row.Scan(&newGems); err != nil {
            if err == pgx.ErrNoRows {
                c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
                return
            }
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    } else {
        row := tx.QueryRow(context.Background(), `UPDATE users SET gems = gems + $1 WHERE id=$2 RETURNING gems`, req.Delta, userID)
        if err := row.Scan(&newGems); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    // insert transaction record
    meta := map[string]interface{}{"reason": "manual"}
    metaB, _ := json.Marshal(meta)
    if _, err := tx.Exec(context.Background(), `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, "balance_adjust", req.Delta, metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    if err := tx.Commit(context.Background()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"gems": newGems})
}

// AddHistory records a transaction/history entry
func (h *Handler) AddHistory(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    var req struct{ Type string `json:"type"`; Amount int64 `json:"amount"`; Meta map[string]interface{} `json:"meta"` }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
        return
    }

    metaB, _ := json.Marshal(req.Meta)
    if _, err := h.DB.Exec(c.Request.Context(), `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, req.Type, req.Amount, metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetHistory returns recent transactions for the current user
func (h *Handler) GetHistory(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    ctx := c.Request.Context()
    rows, err := h.DB.Query(ctx, `SELECT id, type, amount, meta, created_at FROM transactions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 200`, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer rows.Close()
    var out []map[string]interface{}
    for rows.Next() {
        var id int64
        var tType string
        var amount int64
        var metaB []byte
        var createdAt time.Time
        if err := rows.Scan(&id, &tType, &amount, &metaB, &createdAt); err != nil {
            continue
        }
        var meta interface{}
        _ = json.Unmarshal(metaB, &meta)
        out = append(out, map[string]interface{}{"id": id, "type": tType, "amount": amount, "meta": meta, "date": createdAt})
    }
    c.JSON(http.StatusOK, gin.H{"history": out})
}

// CoinFlip performs a server-side coin flip: 50/50. Expects {bet:int}
func (h *Handler) CoinFlip(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    var req struct{ Bet int64 `json:"bet"` }
    if err := c.BindJSON(&req); err != nil || req.Bet <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bet"})
        return
    }

    ctx := c.Request.Context()
    // perform in transaction: deduct bet if possible, determine result, award if win, insert transaction
    tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer func(){ _ = tx.Rollback(ctx) }()

    // lock and check balance
    var balance int64
    row := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID)
    if err := row.Scan(&balance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    if balance < req.Bet {
        c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
        return
    }

    // deduct bet
    if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, req.Bet, userID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    // coin flip
    win := false
    if (time.Now().UnixNano()%2)==0 { win = true }

    awarded := int64(0)
    if win {
        awarded = req.Bet * 2
        if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    meta := map[string]interface{}{"bet": req.Bet, "awarded": awarded, "win": win}
    metaB, _ := json.Marshal(meta)
    if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, "coinflip", (awarded - req.Bet), metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    // get new balance
    var newBalance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }


    // Record game history
    var gameResult domain.GameResult
    if win {
        gameResult = domain.GameResultWin
    } else {
        gameResult = domain.GameResultLose
    }
    go h.RecordGameResult(userID, domain.GameTypeCoinflip, domain.GameModePVE, gameResult, req.Bet, awarded-req.Bet, meta)

    c.JSON(http.StatusOK, gin.H{"win": win, "awarded": awarded, "gems": newBalance})
}

// RPS: server-side rock-paper-scissors PvE
func (h *Handler) RPS(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    var req struct{ Move string `json:"move"`; Bet int64 `json:"bet"` }
    if err := c.BindJSON(&req); err != nil || (req.Move != "rock" && req.Move != "paper" && req.Move != "scissors") {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    // optional betting logic like coinflip
    ctx := c.Request.Context()
    tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer func(){ _ = tx.Rollback(ctx) }()

    // handle bet deduction if bet>0
    if req.Bet > 0 {
        var balance int64
        if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
        if balance < req.Bet {
            c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
            return
        }
        if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, req.Bet, userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    // bot move
    var botMove string
    switch time.Now().UnixNano() % 3 {
    case 0:
        botMove = "rock"
    case 1:
        botMove = "paper"
    default:
        botMove = "scissors"
    }

    // determine winner: returns 1=user win, 0=draw, -1=bot win
    result := 0
    if req.Move == botMove { result = 0 } else if (req.Move=="rock"&&botMove=="scissors")||(req.Move=="paper"&&botMove=="rock")||(req.Move=="scissors"&&botMove=="paper") { result = 1 } else { result = -1 }

    awarded := int64(0)
    if result == 1 && req.Bet > 0 {
        awarded = req.Bet * 2
        if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    meta := map[string]interface{}{"move": req.Move, "bot": botMove, "result": result}
    metaB, _ := json.Marshal(meta)
    netAmount := awarded - req.Bet
    if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, "rps", netAmount, metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    var newBalance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }


    // Record game history
    var gameResult domain.GameResult
    if result == 1 {
        gameResult = domain.GameResultWin
    } else if result == 0 {
        gameResult = domain.GameResultDraw
    } else {
        gameResult = domain.GameResultLose
    }
    go h.RecordGameResult(userID, domain.GameTypeRPS, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

    c.JSON(http.StatusOK, gin.H{"move": req.Move, "bot": botMove, "result": result, "awarded": awarded, "gems": newBalance})
}

// Mines PvE simple: user picks a cell 1..12; server places 4 mines. If pick is safe, user wins bet*2, else loses.
func (h *Handler) Mines(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    var req struct{ Pick int `json:"pick"`; Bet int64 `json:"bet"` }
    if err := c.BindJSON(&req); err != nil || req.Pick < 1 || req.Pick > 12 || req.Bet <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    ctx := c.Request.Context()
    tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer func(){ _ = tx.Rollback(ctx) }()

    var balance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    if balance < req.Bet {
        c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
        return
    }

    if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, req.Bet, userID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    // place 4 unique mines
    mines := map[int]bool{}
    for len(mines) < 4 {
        n := rand.Intn(12) + 1
        mines[n] = true
    }

    pickIsMine := mines[req.Pick]
    awarded := int64(0)
    if !pickIsMine {
        awarded = req.Bet * 2
        if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    meta := map[string]interface{}{"pick": req.Pick, "mines": mines, "win": !pickIsMine}
    metaB, _ := json.Marshal(meta)
    netAmount := awarded - req.Bet
    if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, "mines", netAmount, metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    var newBalance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }


    // Record game history
    var gameResult domain.GameResult
    if !pickIsMine {
        gameResult = domain.GameResultWin
    } else {
        gameResult = domain.GameResultLose
    }
    go h.RecordGameResult(userID, domain.GameTypeMines, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

    c.JSON(http.StatusOK, gin.H{"win": !pickIsMine, "awarded": awarded, "gems": newBalance})
}

// CaseSpin performs a server-side case/roulette spin with fixed prize distribution
func (h *Handler) CaseSpin(c *gin.Context) {
    uidVal, ok := c.Get("user_id")
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
        return
    }
    var userID int64
    switch v := uidVal.(type) {
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
        return
    }

    // fixed cost matching frontend
    const COST int64 = 100
    // prize distribution
    cases := []struct{ID int; Amount int64; Prob float64}{{1,250,0.5},{2,500,0.2},{3,750,0.15},{4,1000,0.10},{5,5000,0.05}}

    ctx := c.Request.Context()
    tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    defer func(){ _ = tx.Rollback(ctx) }()

    // lock balance
    var balance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }
    if balance < COST {
        c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
        return
    }

    if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, COST, userID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    // weighted pick
    r := rand.Float64()
    acc := 0.0
    var picked struct{ID int; Amount int64; Prob float64}
    for _, cs := range cases {
        acc += cs.Prob
        if r <= acc {
            picked = cs
            break
        }
    }
    if picked.Amount == 0 { picked = cases[len(cases)-1] }

    awarded := picked.Amount
    if awarded > 0 {
        if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
            return
        }
    }

    netAmount := awarded - COST
    meta := map[string]interface{}{"case_id": picked.ID, "prize": awarded, "cost": COST}
    metaB, _ := json.Marshal(meta)
    if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id,type,amount,meta) VALUES ($1,$2,$3,$4)`, userID, "case", netAmount, metaB); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    var newBalance int64
    if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }

    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
        return
    }


    // Record game history
    var gameResult domain.GameResult
    if netAmount >= 0 {
        gameResult = domain.GameResultWin
    } else {
        gameResult = domain.GameResultLose
    }
    go h.RecordGameResult(userID, domain.GameTypeCase, domain.GameModeSolo, gameResult, COST, netAmount, meta)

    c.JSON(http.StatusOK, gin.H{"prize": awarded, "case_id": picked.ID, "gems": newBalance})
}
