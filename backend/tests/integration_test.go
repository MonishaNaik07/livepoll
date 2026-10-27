//go:build integration

// Run with real services:
//   TEST_MONGO_URI=mongodb://localhost:27017 TEST_REDIS_ADDR=localhost:6379 go test -tags integration ./tests/...
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/your-username/live-polling-app/backend/internal/config"
	"github.com/your-username/live-polling-app/backend/internal/database"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/repositories"
	"github.com/your-username/live-polling-app/backend/internal/routes"
	"github.com/your-username/live-polling-app/backend/internal/services"
	ws "github.com/your-username/live-polling-app/backend/internal/websocket"
)

const origin = "http://localhost:5173"

type env struct {
	db    *mongo.Database
	srv   *httptest.Server
	store *rds.Store
	polls *services.PollService
	votes *services.VoteService
}

func setup(t *testing.T) *env {
	t.Helper()
	mongoURI, redisAddr := os.Getenv("TEST_MONGO_URI"), os.Getenv("TEST_REDIS_ADDR")
	if mongoURI == "" || redisAddr == "" {
		t.Skip("TEST_MONGO_URI / TEST_REDIS_ADDR not set")
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{Env: "test", FrontendOrigin: origin, PublicAppURL: origin, JWTSecret: "0123456789abcdef0123456789abcdef",
		JWTTTL: time.Hour, MaxBodyBytes: 65536, CookieSameSite: "lax"}
	dbName := fmt.Sprintf("livepoll_test_%d", time.Now().UnixNano())
	mc, db, err := database.ConnectMongo(context.Background(), mongoURI, dbName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Drop(context.Background()); _ = mc.Disconnect(context.Background()) })
	store := rds.New(redisAddr, "", false)
	t.Cleanup(func() { _ = store.Close() })

	ur, pr, vr := repositories.NewUserRepo(db), repositories.NewPollRepo(db), repositories.NewVoteRepo(db)
	auth := services.NewAuthService(ur, cfg.JWTSecret, cfg.JWTTTL)
	polls := services.NewPollService(pr, vr, store, log)
	votes := services.NewVoteService(polls, vr, store, log)
	hub := ws.NewHub(store, polls, log, origin)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go hub.Run(ctx)
	srv := httptest.NewServer(routes.New(routes.Deps{Cfg: cfg, Log: log, Mongo: mc, Store: store, Auth: auth, Polls: polls, Votes: votes, Hub: hub}))
	t.Cleanup(srv.Close)
	return &env{db, srv, store, polls, votes}
}

type client struct {
	t   *testing.T
	c   *http.Client
	url string
}

func (e *env) newClient(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t, &http.Client{Jar: jar}, e.srv.URL}
}

func (c *client) do(method, path string, body interface{}) (int, map[string]interface{}) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.url+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "livepoll")
	req.Header.Set("Origin", origin)
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func data(m map[string]interface{}) map[string]interface{} { d, _ := m["data"].(map[string]interface{}); return d }

func signup(t *testing.T, c *client, email string) {
	if st, b := c.do("POST", "/api/auth/signup", map[string]string{"name": "T", "email": email, "password": "password123"}); st != 201 {
		t.Fatalf("signup %d %v", st, b)
	}
}

func createPoll(t *testing.T, owner *client) string {
	st, b := owner.do("POST", "/api/polls", map[string]interface{}{"question": "Favourite?", "options": []string{"Go", "Rust", "Zig"}})
	if st != 201 {
		t.Fatalf("create %d %v", st, b)
	}
	return data(b)["id"].(string)
}

func TestAuthRequiredAndOwnership(t *testing.T) {
	e := setup(t)
	anon, alice, bob := e.newClient(t), e.newClient(t), e.newClient(t)
	if st, _ := anon.do("POST", "/api/polls", map[string]interface{}{"question": "Q???", "options": []string{"a", "b"}}); st != 401 {
		t.Fatalf("anonymous create must be 401, got %d", st)
	}
	signup(t, alice, "alice@example.com")
	signup(t, bob, "bob@example.com")
	if st, _ := e.newClient(t).do("POST", "/api/auth/signup", map[string]string{"name": "x", "email": "ALICE@example.com", "password": "password123"}); st != 409 {
		t.Fatalf("duplicate email must be 409, got %d", st)
	}
	id := createPoll(t, alice)
	if st, _ := bob.do("POST", "/api/polls/"+id+"/close", nil); st != 403 {
		t.Fatalf("non-owner close must be 403, got %d", st)
	}
	if st, _ := bob.do("DELETE", "/api/polls/"+id, nil); st != 403 {
		t.Fatalf("non-owner delete must be 403, got %d", st)
	}
	if st, _ := alice.do("POST", "/api/polls/"+id+"/close", nil); st != 200 {
		t.Fatalf("owner close failed: %d", st)
	}
}

func TestCSRFHeaderRequired(t *testing.T) {
	e := setup(t)
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/auth/login", bytes.NewReader([]byte(`{}`)))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 without CSRF header, got %d", resp.StatusCode)
	}
}

func TestVotingFlowAndRules(t *testing.T) {
	e := setup(t)
	owner, v1, v2 := e.newClient(t), e.newClient(t), e.newClient(t)
	signup(t, owner, "owner@example.com")
	id := createPoll(t, owner)

	if st, _ := v1.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "nope"}); st != 422 {
		t.Fatalf("invalid option must be 422, got %d", st)
	}
	if st, _ := v1.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o1"}); st != 201 {
		t.Fatalf("vote failed: %d", st)
	}
	if st, _ := v1.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o2"}); st != 409 {
		t.Fatalf("duplicate vote must be 409, got %d", st)
	}
	if st, _ := v2.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o1"}); st != 201 {
		t.Fatalf("second voter failed: %d", st)
	}
	_, b := owner.do("GET", "/api/polls/"+id+"/results", nil)
	res := data(b)
	if res["total"].(float64) != 2 || res["counts"].(map[string]interface{})["o1"].(float64) != 2 {
		t.Fatalf("wrong results: %v", res)
	}
	if st, _ := e.newClient(t).do("POST", "/api/polls/000000000000000000000000/votes", map[string]string{"optionId": "o1"}); st != 404 {
		t.Fatalf("nonexistent poll must be 404, got %d", st)
	}
	owner.do("POST", "/api/polls/"+id+"/close", nil)
	if st, _ := e.newClient(t).do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o1"}); st != 409 {
		t.Fatalf("closed poll vote must be 409, got %d", st)
	}
}

// Many parallel requests from the SAME voter identity: exactly one may succeed.
func TestConcurrentDuplicateVotes(t *testing.T) {
	e := setup(t)
	owner, voter := e.newClient(t), e.newClient(t)
	signup(t, owner, "owner@example.com")
	id := createPoll(t, owner)
	voter.do("GET", "/api/polls/"+id, nil) // mints the voter cookie

	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if st, _ := voter.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o1"}); st == 201 {
				mu.Lock()
				created++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if created != 1 {
		t.Fatalf("expected exactly 1 accepted vote, got %d", created)
	}
	_, b := owner.do("GET", "/api/polls/"+id+"/results", nil)
	if data(b)["total"].(float64) != 1 {
		t.Fatalf("redis total must be 1: %v", data(b))
	}
}

// Redis loses all data -> counts are rebuilt from Mongo's durable votes.
func TestRedisRecoveryFromMongo(t *testing.T) {
	e := setup(t)
	owner, v1 := e.newClient(t), e.newClient(t)
	signup(t, owner, "owner@example.com")
	id := createPoll(t, owner)
	v1.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o3"})

	if err := e.store.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	_, b := owner.do("GET", "/api/polls/"+id+"/results", nil)
	if data(b)["counts"].(map[string]interface{})["o3"].(float64) != 1 {
		t.Fatalf("not rebuilt from mongo: %v", data(b))
	}
}

// Mongo accepted a vote but Redis never finished applying it (applied=false) -> Reconcile fixes it
// exactly once, whether Redis kept its data or lost it.
func TestReconcileAppliesMissedVotes(t *testing.T) {
	for _, redisLost := range []bool{false, true} {
		t.Run(fmt.Sprintf("redisLost=%v", redisLost), func(t *testing.T) {
			e := setup(t)
			owner, v1 := e.newClient(t), e.newClient(t)
			signup(t, owner, fmt.Sprintf("owner%v@example.com", redisLost))
			id := createPoll(t, owner)
			v1.do("POST", "/api/polls/"+id+"/votes", map[string]string{"optionId": "o2"})

			ctx := context.Background()
			_, err := e.db.Collection("votes").UpdateMany(ctx, bson.M{}, bson.M{"$set": bson.M{"applied": false}})
			if err != nil {
				t.Fatal(err)
			}
			if redisLost {
				_ = e.store.Delete(ctx, id)
			}
			time.Sleep(6 * time.Second) // reconciler only touches votes older than 5s
			e.votes.Reconcile(ctx)
			e.votes.Reconcile(ctx) // twice: must stay idempotent

			_, b := owner.do("GET", "/api/polls/"+id+"/results", nil)
			if c := data(b)["counts"].(map[string]interface{})["o2"].(float64); c != 1 {
				t.Fatalf("expected exactly 1 after reconcile, got %v", c)
			}
		})
	}
}
