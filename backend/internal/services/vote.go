package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/models"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/repositories"
	"github.com/your-username/live-polling-app/backend/internal/validation"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VoteService struct {
	polls *PollService
	votes *repositories.VoteRepo
	store *rds.Store
	log   *slog.Logger
}

func NewVoteService(p *PollService, v *repositories.VoteRepo, s *rds.Store, l *slog.Logger) *VoteService {
	return &VoteService{polls: p, votes: v, store: s, log: l}
}

// Cast flow: validate -> Mongo insert (unique index = duplicate guard) -> Redis atomic apply -> publish -> mark applied.
// If anything after the Mongo insert fails, the vote is still durable and Reconcile applies it later.
func (s *VoteService) Cast(ctx context.Context, pollID primitive.ObjectID, voterID string, in validation.VoteInput) (*models.Results, error) {
	if e := in.Validate(); e != nil {
		return nil, e
	}
	p, err := s.polls.Get(ctx, pollID)
	if err != nil {
		return nil, err
	}
	if EffectiveStatus(p, time.Now()) != models.PollOpen {
		return nil, apperr.ErrPollClosed
	}
	valid := false
	for _, o := range p.Options {
		if o.ID == in.OptionID {
			valid = true
			break
		}
	}
	if !valid {
		return nil, apperr.ErrBadOption
	}

	v := &models.Vote{PollID: pollID, OptionID: in.OptionID, VoterID: voterID}
	if err := s.votes.Insert(ctx, v); err != nil {
		return nil, err // ErrAlreadyVoted on duplicate; Redis untouched
	}

	res, err := s.applyAndPublish(ctx, p, v)
	if err != nil {
		// Vote is durable in Mongo (applied=false). Reconciler will finish the job.
		s.log.Error("vote saved but redis apply failed; reconciler will retry", "poll", pollID.Hex(), "vote", v.ID.Hex(), "err", err)
		return &models.Results{PollID: pollID.Hex(), Status: p.Status, Counts: map[string]int64{}}, nil
	}
	return res, nil
}

func (s *VoteService) applyAndPublish(ctx context.Context, p *models.Poll, v *models.Vote) (*models.Results, error) {
	pid := p.ID.Hex()
	if ok, err := s.store.Exists(ctx, pid); err == nil && !ok {
		if err := s.polls.Rebuild(ctx, p); err != nil { // rebuild already includes this vote
			return nil, err
		}
	}
	if _, _, err := s.store.ApplyVote(ctx, pid, v.ID.Hex(), v.OptionID); err != nil {
		return nil, err
	}
	res, err := s.polls.Results(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := s.store.Publish(ctx, pid, res); err != nil {
		return nil, err
	}
	_ = s.votes.MarkApplied(ctx, v.ID)
	return res, nil
}

// Reconcile re-applies votes Mongo accepted but Redis may have missed. ApplyVote is idempotent (SADD guard),
// so running this repeatedly or concurrently on several instances never double counts.
func (s *VoteService) Reconcile(ctx context.Context) {
	pending, err := s.votes.Unapplied(ctx, time.Now().Add(-5*time.Second), 200)
	if err != nil {
		s.log.Error("reconcile scan failed", "err", err)
		return
	}
	for i := range pending {
		v := &pending[i]
		p, err := s.polls.Get(ctx, v.PollID)
		if err != nil {
			if err == apperr.ErrNotFound { // poll deleted
				_ = s.votes.MarkApplied(ctx, v.ID)
			}
			continue
		}
		if _, err := s.applyAndPublish(ctx, p, v); err != nil {
			s.log.Warn("reconcile apply failed", "vote", v.ID.Hex(), "err", err)
		}
	}
}

func (s *VoteService) RunReconciler(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Reconcile(ctx)
		}
	}
}

func (s *VoteService) MyVote(ctx context.Context, pollID primitive.ObjectID, voterID string) (bool, string, error) {
	return s.votes.HasVoted(ctx, pollID, voterID)
}
