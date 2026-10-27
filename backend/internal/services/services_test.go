package services

import (
	"strings"
	"testing"
	"time"

	"github.com/your-username/live-polling-app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestTokenRoundTripAndRejection(t *testing.T) {
	s := NewAuthService(nil, secret, time.Hour)
	id := primitive.NewObjectID()
	tok, err := s.issue(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Parse(tok)
	if err != nil || got != id {
		t.Fatalf("round trip failed: %v %v", got, err)
	}
	if _, err := NewAuthService(nil, strings.Repeat("x", 32), time.Hour).Parse(tok); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
	expired := NewAuthService(nil, secret, -time.Minute)
	etok, _ := expired.issue(id)
	if _, err := s.Parse(etok); err == nil {
		t.Fatal("expired token must be rejected")
	}
	if _, err := s.Parse("garbage"); err == nil {
		t.Fatal("garbage token must be rejected")
	}
}

func TestPasswordHashing(t *testing.T) {
	h, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.DefaultCost)
	if err != nil || string(h) == "correct horse" {
		t.Fatal("password must be hashed")
	}
	if bcrypt.CompareHashAndPassword(h, []byte("correct horse")) != nil {
		t.Fatal("verify failed")
	}
	if bcrypt.CompareHashAndPassword(h, []byte("wrong")) == nil {
		t.Fatal("wrong password accepted")
	}
}

func TestEffectiveStatus(t *testing.T) {
	now := time.Now()
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	cases := []struct {
		p    models.Poll
		want models.PollStatus
	}{
		{models.Poll{Status: models.PollOpen}, models.PollOpen},
		{models.Poll{Status: models.PollOpen, ExpiresAt: &future}, models.PollOpen},
		{models.Poll{Status: models.PollOpen, ExpiresAt: &past}, models.PollClosed},
		{models.Poll{Status: models.PollClosed, ExpiresAt: &future}, models.PollClosed},
	}
	for i, c := range cases {
		if got := EffectiveStatus(&c.p, now); got != c.want {
			t.Errorf("case %d: got %s want %s", i, got, c.want)
		}
	}
}
