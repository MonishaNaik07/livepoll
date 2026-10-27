package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/models"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/repositories"
	"github.com/your-username/live-polling-app/backend/internal/validation"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PollService struct {
	polls *repositories.PollRepo
	votes *repositories.VoteRepo
	store *rds.Store
	log   *slog.Logger
}

func NewPollService(p *repositories.PollRepo, v *repositories.VoteRepo, s *rds.Store, l *slog.Logger) *PollService {
	return &PollService{polls: p, votes: v, store: s, log: l}
}

// EffectiveStatus folds expiry into status so an expired poll behaves as closed.
func EffectiveStatus(p *models.Poll, now time.Time) models.PollStatus {
	if p.Status == models.PollOpen && p.ExpiresAt != nil && !p.ExpiresAt.After(now) {
		return models.PollClosed
	}
	return p.Status
}

func (s *PollService) Create(ctx context.Context, owner primitive.ObjectID, in validation.CreatePollInput) (*models.Poll, error) {
	if e := in.Validate(time.Now()); e != nil {
		return nil, e
	}
	opts := make([]models.Option, len(in.Options))
	ids := make([]string, len(in.Options))
	for i, t := range in.Options {
		opts[i] = models.Option{ID: fmt.Sprintf("o%d", i+1), Text: t}
		ids[i] = opts[i].ID
	}
	p := &models.Poll{OwnerID: owner, Question: in.Question, Description: in.Description,
		Options: opts, Status: models.PollOpen, ExpiresAt: in.ExpiresAt}
	if err := s.polls.Create(ctx, p); err != nil {
		return nil, err
	}
	// Best effort: if Redis is down, EnsureLive lazily rebuilds from Mongo on first read.
	if err := s.store.InitPoll(ctx, p.ID.Hex(), ids); err != nil {
		s.log.Warn("redis init failed; will rebuild lazily", "poll", p.ID.Hex(), "err", err)
	}
	return p, nil
}

func (s *PollService) Get(ctx context.Context, id primitive.ObjectID) (*models.Poll, error) {
	return s.polls.FindByID(ctx, id)
}

func (s *PollService) List(ctx context.Context, owner primitive.ObjectID, page, size int64) ([]models.Poll, int64, error) {
	return s.polls.ListByOwner(ctx, owner, size, (page-1)*size)
}

func (s *PollService) owned(ctx context.Context, id, owner primitive.ObjectID) (*models.Poll, error) {
	p, err := s.polls.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.OwnerID != owner {
		return nil, apperr.ErrForbidden
	}
	return p, nil
}

func (s *PollService) SetStatus(ctx context.Context, id, owner primitive.ObjectID, st models.PollStatus) (*models.Poll, error) {
	if _, err := s.owned(ctx, id, owner); err != nil {
		return nil, err
	}
	if err := s.polls.SetStatus(ctx, id, owner, st); err != nil {
		return nil, err
	}
	p, err := s.polls.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Push status change to every connected viewer through Redis Pub/Sub.
	if r, err := s.Results(ctx, p); err == nil {
		if _, seq, e := s.bumpSeq(ctx, p.ID.Hex()); e == nil {
			r.Seq = seq
		}
		_ = s.store.Publish(ctx, p.ID.Hex(), r)
	}
	return p, nil
}

func (s *PollService) bumpSeq(ctx context.Context, pollID string) (bool, int64, error) {
	n, err := s.store.C.Incr(ctx, "poll:"+pollID+":seq").Result()
	return true, n, err
}

func (s *PollService) Delete(ctx context.Context, id, owner primitive.ObjectID) error {
	if _, err := s.owned(ctx, id, owner); err != nil {
		return err
	}
	if err := s.polls.Delete(ctx, id, owner); err != nil {
		return err
	}
	_ = s.votes.DeleteByPoll(ctx, id)
	_ = s.store.Delete(ctx, id.Hex())
	return nil
}

// Results returns the live snapshot from Redis, rebuilding from Mongo if Redis lost the data.
func (s *PollService) Results(ctx context.Context, p *models.Poll) (*models.Results, error) {
	pid := p.ID.Hex()
	ok, err := s.store.Exists(ctx, pid)
	if err == nil && !ok {
		s.log.Warn("redis counters missing; rebuilding from mongo", "poll", pid)
		if err = s.Rebuild(ctx, p); err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	r, err := s.store.Snapshot(ctx, pid, EffectiveStatus(p, time.Now()))
	if err != nil {
		return nil, err
	}
	for _, o := range p.Options { // guarantee every option has an entry
		if _, ok := r.Counts[o.ID]; !ok {
			r.Counts[o.ID] = 0
		}
	}
	return r, nil
}

// Rebuild derives Redis state purely from Mongo's durable vote records.
func (s *PollService) Rebuild(ctx context.Context, p *models.Poll) error {
	counts, voteIDs, err := s.votes.CountsByOption(ctx, p.ID)
	if err != nil {
		return err
	}
	for _, o := range p.Options {
		if _, ok := counts[o.ID]; !ok {
			counts[o.ID] = 0
		}
	}
	return s.store.Rebuild(ctx, p.ID.Hex(), counts, voteIDs)
}
