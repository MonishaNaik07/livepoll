package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/your-username/live-polling-app/backend/internal/models"
)

// Key layout (all per poll):
//   poll:{id}:counts   HASH   optionId -> count         (live counts)
//   poll:{id}:applied  SET    voteIds already counted   (idempotency)
//   poll:{id}:seq      STRING monotonic event sequence
//   channel poll:{id}  PUBSUB results snapshot after each applied vote / status change
func countsKey(id string) string  { return "poll:" + id + ":counts" }
func appliedKey(id string) string { return "poll:" + id + ":applied" }
func seqKey(id string) string     { return "poll:" + id + ":seq" }
func Channel(id string) string    { return "poll:" + id }

const ChannelPrefix = "poll:"

type Store struct{ C *redis.Client }

func New(addr, password string, useTLS bool) *Store {
	o := &redis.Options{Addr: addr, Password: password, DialTimeout: 5e9, ReadTimeout: 3e9, WriteTimeout: 3e9}
	if useTLS {
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Store{C: redis.NewClient(o)}
}

func (s *Store) Ping(ctx context.Context) error { return s.C.Ping(ctx).Err() }
func (s *Store) Close() error                   { return s.C.Close() }

// Atomic + idempotent: counts a vote at most once per voteId, even if called repeatedly
// (retries, reconciler). Returns {applied(0|1), seq}.
var applyVote = redis.NewScript(`
if redis.call('SADD', KEYS[2], ARGV[1]) == 0 then
  return {0, tonumber(redis.call('GET', KEYS[3]) or '0')}
end
redis.call('HINCRBY', KEYS[1], ARGV[2], 1)
local seq = redis.call('INCR', KEYS[3])
return {1, seq}
`)

// ApplyVote returns (newlyApplied, seq).
func (s *Store) ApplyVote(ctx context.Context, pollID, voteID, optionID string) (bool, int64, error) {
	res, err := applyVote.Run(ctx, s.C, []string{countsKey(pollID), appliedKey(pollID), seqKey(pollID)}, voteID, optionID).Int64Slice()
	if err != nil {
		return false, 0, fmt.Errorf("apply vote: %w", err)
	}
	return res[0] == 1, res[1], nil
}

// InitPoll creates zeroed counters so the key exists (distinguishes "no votes" from "lost data").
func (s *Store) InitPoll(ctx context.Context, pollID string, optionIDs []string) error {
	args := make([]interface{}, 0, len(optionIDs)*2)
	for _, id := range optionIDs {
		args = append(args, id, 0)
	}
	pipe := s.C.TxPipeline()
	pipe.HSet(ctx, countsKey(pollID), args...)
	pipe.SetNX(ctx, seqKey(pollID), 0, 0)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Exists(ctx context.Context, pollID string) (bool, error) {
	n, err := s.C.Exists(ctx, countsKey(pollID)).Result()
	return n == 1, err
}

// Snapshot reads live counts. Caller must ensure the poll is initialised (see Exists/Rebuild).
func (s *Store) Snapshot(ctx context.Context, pollID string, status models.PollStatus) (*models.Results, error) {
	pipe := s.C.Pipeline()
	h := pipe.HGetAll(ctx, countsKey(pollID))
	sq := pipe.Get(ctx, seqKey(pollID))
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	r := &models.Results{PollID: pollID, Status: status, Counts: map[string]int64{}}
	for k, v := range h.Val() {
		n, _ := strconv.ParseInt(v, 10, 64)
		r.Counts[k] = n
		r.Total += n
	}
	r.Seq, _ = strconv.ParseInt(sq.Val(), 10, 64)
	return r, nil
}

// Rebuild replaces Redis state from Mongo-derived data (counts + all vote ids). Used on cache loss
// and by the reconciler. Preserves seq monotonicity by incrementing it.
func (s *Store) Rebuild(ctx context.Context, pollID string, counts map[string]int64, voteIDs []string) error {
	pipe := s.C.TxPipeline()
	pipe.Del(ctx, countsKey(pollID), appliedKey(pollID))
	args := make([]interface{}, 0, len(counts)*2)
	for k, v := range counts {
		args = append(args, k, v)
	}
	if len(args) > 0 {
		pipe.HSet(ctx, countsKey(pollID), args...)
	}
	for i := 0; i < len(voteIDs); i += 500 {
		end := i + 500
		if end > len(voteIDs) {
			end = len(voteIDs)
		}
		members := make([]interface{}, 0, end-i)
		for _, v := range voteIDs[i:end] {
			members = append(members, v)
		}
		pipe.SAdd(ctx, appliedKey(pollID), members...)
	}
	pipe.Incr(ctx, seqKey(pollID))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Publish(ctx context.Context, pollID string, r *models.Results) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.C.Publish(ctx, Channel(pollID), b).Err()
}

func (s *Store) Delete(ctx context.Context, pollID string) error {
	return s.C.Del(ctx, countsKey(pollID), appliedKey(pollID), seqKey(pollID)).Err()
}

// PSubscribe returns a pattern subscription for all poll channels (one per instance, fan-out in-process).
func (s *Store) PSubscribe(ctx context.Context) *redis.PubSub {
	return s.C.PSubscribe(ctx, ChannelPrefix+"*")
}
