package redis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/your-username/live-polling-app/backend/internal/models"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	return New(mr.Addr(), "", false)
}

func TestApplyVoteIsIdempotent(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	_ = s.InitPoll(ctx, "p1", []string{"o1", "o2"})

	applied, seq, err := s.ApplyVote(ctx, "p1", "vote-A", "o1")
	if err != nil || !applied || seq != 1 {
		t.Fatalf("first apply: %v %v %v", applied, seq, err)
	}
	applied, _, _ = s.ApplyVote(ctx, "p1", "vote-A", "o1") // retry / reconciler replay
	if applied {
		t.Fatal("replayed vote must not be counted twice")
	}
	r, _ := s.Snapshot(ctx, "p1", models.PollOpen)
	if r.Counts["o1"] != 1 || r.Total != 1 || r.Seq != 1 {
		t.Fatalf("bad snapshot: %+v", r)
	}
}

func TestConcurrentDistinctVotesAllCounted(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	_ = s.InitPoll(ctx, "p1", []string{"o1", "o2"})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, _ = s.ApplyVote(ctx, "p1", fmt.Sprintf("v%d", i), "o2")
		}(i)
	}
	wg.Wait()
	r, _ := s.Snapshot(ctx, "p1", models.PollOpen)
	if r.Counts["o2"] != 50 || r.Seq != 50 {
		t.Fatalf("lost updates: %+v", r)
	}
}

func TestRebuildRestoresStateAndStaysIdempotent(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if err := s.Rebuild(ctx, "p1", map[string]int64{"o1": 3, "o2": 1}, []string{"a", "b", "c", "d"}); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Snapshot(ctx, "p1", models.PollOpen)
	if r.Counts["o1"] != 3 || r.Total != 4 {
		t.Fatalf("rebuild wrong: %+v", r)
	}
	if applied, _, _ := s.ApplyVote(ctx, "p1", "a", "o1"); applied {
		t.Fatal("vote already in rebuilt set must not be re-counted")
	}
	if ok, _ := s.Exists(ctx, "p1"); !ok {
		t.Fatal("poll should exist after rebuild")
	}
}

func TestPublishReachesSubscriber(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	ps := s.PSubscribe(ctx)
	defer ps.Close()
	if _, err := ps.Receive(ctx); err != nil { // subscription confirmation
		t.Fatal(err)
	}
	_ = s.Publish(ctx, "p1", &models.Results{PollID: "p1", Counts: map[string]int64{"o1": 1}, Total: 1, Seq: 1})
	select {
	case m := <-ps.Channel():
		if m.Channel != "poll:p1" {
			t.Fatalf("wrong channel %s", m.Channel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event received")
	}
}
