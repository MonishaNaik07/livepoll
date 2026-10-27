package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Users ----------

type UserRepo struct{ c *mongo.Collection }

func NewUserRepo(db *mongo.Database) *UserRepo { return &UserRepo{db.Collection("users")} }

func (r *UserRepo) Create(ctx context.Context, u *models.User) error {
	u.CreatedAt = time.Now().UTC()
	res, err := r.c.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrEmailTaken
	}
	if err != nil {
		return err
	}
	u.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.c.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrNotFound
	}
	return &u, err
}

func (r *UserRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var u models.User
	err := r.c.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrNotFound
	}
	return &u, err
}

// ---------- Polls ----------

type PollRepo struct{ c *mongo.Collection }

func NewPollRepo(db *mongo.Database) *PollRepo { return &PollRepo{db.Collection("polls")} }

func (r *PollRepo) Create(ctx context.Context, p *models.Poll) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	res, err := r.c.InsertOne(ctx, p)
	if err != nil {
		return err
	}
	p.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *PollRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Poll, error) {
	var p models.Poll
	err := r.c.FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrNotFound
	}
	return &p, err
}

func (r *PollRepo) ListByOwner(ctx context.Context, owner primitive.ObjectID, limit, skip int64) ([]models.Poll, int64, error) {
	filter := bson.M{"ownerId": owner}
	total, err := r.c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.c.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit).SetSkip(skip))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	out := []models.Poll{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// SetStatus is owner-scoped in the query itself, so ownership can't be bypassed by a race.
func (r *PollRepo) SetStatus(ctx context.Context, id, owner primitive.ObjectID, s models.PollStatus) error {
	res, err := r.c.UpdateOne(ctx, bson.M{"_id": id, "ownerId": owner},
		bson.M{"$set": bson.M{"status": s, "updatedAt": time.Now().UTC()}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (r *PollRepo) Delete(ctx context.Context, id, owner primitive.ObjectID) error {
	res, err := r.c.DeleteOne(ctx, bson.M{"_id": id, "ownerId": owner})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// ---------- Votes ----------

type VoteRepo struct{ c *mongo.Collection }

func NewVoteRepo(db *mongo.Database) *VoteRepo { return &VoteRepo{db.Collection("votes")} }

// Insert relies on the unique (pollId, voterId) index: concurrent duplicates cannot both succeed.
func (r *VoteRepo) Insert(ctx context.Context, v *models.Vote) error {
	v.CreatedAt = time.Now().UTC()
	v.Applied = false
	res, err := r.c.InsertOne(ctx, v)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrAlreadyVoted
	}
	if err != nil {
		return err
	}
	v.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *VoteRepo) MarkApplied(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"applied": true}})
	return err
}

func (r *VoteRepo) HasVoted(ctx context.Context, poll primitive.ObjectID, voter string) (bool, string, error) {
	var v models.Vote
	err := r.c.FindOne(ctx, bson.M{"pollId": poll, "voterId": voter}).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, "", nil
	}
	return err == nil, v.OptionID, err
}

// CountsByOption is the authoritative tally used to rebuild Redis.
func (r *VoteRepo) CountsByOption(ctx context.Context, poll primitive.ObjectID) (map[string]int64, []string, error) {
	cur, err := r.c.Find(ctx, bson.M{"pollId": poll}, options.Find().SetProjection(bson.M{"optionId": 1}))
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)
	counts := map[string]int64{}
	ids := []string{}
	for cur.Next(ctx) {
		var v models.Vote
		if err := cur.Decode(&v); err != nil {
			return nil, nil, err
		}
		counts[v.OptionID]++
		ids = append(ids, v.ID.Hex())
	}
	return counts, ids, cur.Err()
}

func (r *VoteRepo) Unapplied(ctx context.Context, olderThan time.Time, limit int64) ([]models.Vote, error) {
	cur, err := r.c.Find(ctx, bson.M{"applied": false, "createdAt": bson.M{"$lt": olderThan}}, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.Vote
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *VoteRepo) DeleteByPoll(ctx context.Context, poll primitive.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, bson.M{"pollId": poll})
	return err
}
