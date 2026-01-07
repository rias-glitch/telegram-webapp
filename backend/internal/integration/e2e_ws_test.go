package integration

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"telegram_webapp/internal/domain"
	httpserver "telegram_webapp/internal/http"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/service"
)

func applyMigrationsToPool(t *testing.T, dbp *pgxpool.Pool) {
    t.Helper()
    migDir := filepath.Join("..", "..", "internal", "migrations")
    files, err := os.ReadDir(migDir)
    if err != nil {
        t.Fatalf("read migrations: %v", err)
    }
    for _, f := range files {
        b, err := os.ReadFile(filepath.Join(migDir, f.Name()))
        if err != nil {
            t.Fatalf("read file: %v", err)
        }
        if _, err := dbp.Exec(context.Background(), string(b)); err != nil {
            t.Fatalf("apply migration %s: %v", f.Name(), err)
        }
    }
}

func TestE2E_WS_Match(t *testing.T) {
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        t.Skip("DATABASE_URL not set")
    }
    os.Setenv("JWT_SECRET", "test-secret")

    dbp, err := pgxpool.New(context.Background(), dsn)
    if err != nil {
        t.Fatalf("connect db: %v", err)
    }
    defer dbp.Close()

    applyMigrationsToPool(t, dbp)

    // create or reuse two users
    ur := repository.NewUserRepository(dbp)
    ctx := context.Background()
    tgA := int64(1001)
    tgB := int64(1002)

    uA, err := ur.GetByTgID(ctx, tgA)
    if err != nil {
        uA = &domain.User{TgID: tgA, Username: "userA", FirstName: "A"}
        if err := ur.Create(ctx, uA); err != nil {
            t.Fatalf("create userA: %v", err)
        }
    }

    uB, err := ur.GetByTgID(ctx, tgB)
    if err != nil {
        uB = &domain.User{TgID: tgB, Username: "userB", FirstName: "B"}
        if err := ur.Create(ctx, uB); err != nil {
            t.Fatalf("create userB: %v", err)
        }
    }

    service.InitJWT()
    tokenA, err := service.GenerateJWT(uA.ID)
    if err != nil {
        t.Fatalf("gen token A: %v", err)
    }
    tokenB, err := service.GenerateJWT(uB.ID)
    if err != nil {
        t.Fatalf("gen token B: %v", err)
    }

    // start server with real routes
    gin.SetMode(gin.TestMode)
    r := gin.Default()
    httpserver.RegisterRoutes(r, dbp, "dummy-bot-token")
    ts := httptest.NewServer(r)
    defer ts.Close()

    // connect two websocket clients
    wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws?token=" + tokenA
    d := websocket.DefaultDialer
    connA, _, err := d.Dial(wsURL, nil)
    if err != nil {
        t.Fatalf("dial A: %v", err)
    }
    defer connA.Close()

    wsURLB := strings.Replace(ts.URL, "http", "ws", 1) + "/ws?token=" + tokenB
    connB, _, err := d.Dial(wsURLB, nil)
    if err != nil {
        t.Fatalf("dial B: %v", err)
    }
    defer connB.Close()

    // start a single reader goroutine per connection to avoid concurrent ReadMessage calls
    startReader := func(conn *websocket.Conn) chan []byte {
        out := make(chan []byte, 16)
        go func() {
            defer close(out)
            for {
                _, msg, err := conn.ReadMessage()
                if err != nil {
                    return
                }
                out <- msg
            }
        }()
        return out
    }

    chA := startReader(connA)
    chB := startReader(connB)

    // wait for explicit state handshake from both clients (ensures room assigned)
    waitForReady := func(ch chan []byte, tmo time.Duration) bool {
        deadline := time.Now().Add(tmo)
        for time.Now().Before(deadline) {
            select {
            case m, ok := <-ch:
                if !ok {
                    return false
                }
                var obj map[string]any
                _ = json.Unmarshal(m, &obj)
                if obj["type"] == "state" {
                    return true
                }
            case <-time.After(25 * time.Millisecond):
            }
        }
        return false
    }

    if !waitForReady(chA, 2*time.Second) {
        t.Fatalf("A did not receive ready/state")
    }
    if !waitForReady(chB, 2*time.Second) {
        t.Fatalf("B did not receive ready/state")
    }

    // send moves now that both clients signalled readiness
    _ = connA.WriteMessage(websocket.TextMessage, []byte(`{"type":"move","value":"rock"}`))
    _ = connB.WriteMessage(websocket.TextMessage, []byte(`{"type":"move","value":"scissors"}`))

    // read results from the reader channels
    gotA := make(chan []byte, 1)
    go func() {
        deadline := time.Now().Add(10 * time.Second)
        for time.Now().Before(deadline) {
            select {
            case m, ok := <-chA:
                if !ok {
                    return
                }
                var obj map[string]any
                _ = json.Unmarshal(m, &obj)
                if obj["type"] == "result" {
                    gotA <- m
                    return
                }
            case <-time.After(100 * time.Millisecond):
            }
        }
    }()
    gotB := make(chan []byte, 1)
    go func() {
        deadline := time.Now().Add(10 * time.Second)
        for time.Now().Before(deadline) {
            select {
            case m, ok := <-chB:
                if !ok {
                    return
                }
                var obj map[string]any
                _ = json.Unmarshal(m, &obj)
                if obj["type"] == "result" {
                    gotB <- m
                    return
                }
            case <-time.After(100 * time.Millisecond):
            }
        }
    }()

    select {
    case m := <-gotA:
        var obj map[string]any
        _ = json.Unmarshal(m, &obj)
        if obj["type"] != "result" {
            t.Fatalf("A: expected result, got %v", obj)
        }
    case <-time.After(2 * time.Second):
        t.Fatalf("timeout waiting for A result")
    }

    select {
    case m := <-gotB:
        var obj map[string]any
        _ = json.Unmarshal(m, &obj)
        if obj["type"] != "result" {
            t.Fatalf("B: expected result, got %v", obj)
        }
    case <-time.After(2 * time.Second):
        t.Fatalf("timeout waiting for B result")
    }

    // verify game stored
    gr := repository.NewGameRepository(dbp)
    games, err := gr.GetByUser(context.Background(), uA.ID)
    if err != nil {
        t.Fatalf("get games: %v", err)
    }
    if len(games) == 0 {
        t.Fatalf("expected stored game, got 0")
    }
}
