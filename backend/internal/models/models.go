package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Email        string             `bson:"email" json:"email"` // normalized: trimmed + lowercased
	PasswordHash string             `bson:"passwordHash" json:"-"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}

type PollStatus string

const (
	PollOpen   PollStatus = "open"
	PollClosed PollStatus = "closed"
)

type Option struct {
	ID   string `bson:"id" json:"id"` // stable id, e.g. "o1"
	Text string `bson:"text" json:"text"`
}

type Poll struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID     primitive.ObjectID `bson:"ownerId" json:"ownerId"`
	Question    string             `bson:"question" json:"question"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Options     []Option           `bson:"options" json:"options"`
	Status      PollStatus         `bson:"status" json:"status"`
	ExpiresAt   *time.Time         `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Vote is the durable source of truth. Unique index on (pollId, voterId).
// Applied=false means Mongo accepted the vote but Redis has not been updated yet;
// a reconciler applies it and flips the flag (outbox-style recovery).
type Vote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PollID    primitive.ObjectID `bson:"pollId" json:"pollId"`
	OptionID  string             `bson:"optionId" json:"optionId"`
	VoterID   string             `bson:"voterId" json:"-"`
	Applied   bool               `bson:"applied" json:"-"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// Results is the payload sent over REST and WebSocket.
type Results struct {
	PollID string           `json:"pollId"`
	Status PollStatus       `json:"status"`
	Total  int64            `json:"total"`
	Counts map[string]int64 `json:"counts"` // optionId -> count
	Seq    int64            `json:"seq"`    // monotonic; clients drop stale/duplicate events
}
