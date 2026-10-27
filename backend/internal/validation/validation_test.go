package validation

import (
	"strings"
	"testing"
	"time"
)

func TestSignupValidation(t *testing.T) {
	cases := []struct {
		name string
		in   SignupInput
		bad  string
	}{
		{"ok", SignupInput{"Sam", "  Sam@Example.com ", "password123"}, ""},
		{"bad email", SignupInput{"Sam", "nope", "password123"}, "email"},
		{"short password", SignupInput{"Sam", "a@b.co", "short"}, "password"},
		{"password over bcrypt limit", SignupInput{"Sam", "a@b.co", strings.Repeat("a", 73)}, "password"},
		{"empty name", SignupInput{"  ", "a@b.co", "password123"}, "name"},
		{"display-name email rejected", SignupInput{"Sam", "Sam <a@b.co>", "password123"}, "email"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate()
			if c.bad == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if c.in.Email != "sam@example.com" {
					t.Fatalf("email not normalized: %q", c.in.Email)
				}
				return
			}
			if err == nil || err.Fields[c.bad] == "" {
				t.Fatalf("expected field error %q, got %+v", c.bad, err)
			}
		})
	}
}

func TestCreatePollValidation(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	cases := []struct {
		name string
		in   CreatePollInput
		bad  string
	}{
		{"ok", CreatePollInput{Question: "Best language?", Options: []string{"Go", "Rust"}}, ""},
		{"one option", CreatePollInput{Question: "Best language?", Options: []string{"Go"}}, "options"},
		{"eleven options", CreatePollInput{Question: "Best language?", Options: make([]string, 11)}, "options"},
		{"empty option", CreatePollInput{Question: "Best language?", Options: []string{"Go", "  "}}, "options"},
		{"duplicate option (case-insens.)", CreatePollInput{Question: "Best language?", Options: []string{"Go", "go"}}, "options"},
		{"short question", CreatePollInput{Question: "hi", Options: []string{"a", "b"}}, "question"},
		{"past expiry", CreatePollInput{Question: "Best language?", Options: []string{"a", "b"}, ExpiresAt: &past}, "expiresAt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate(now)
			if c.bad == "" && err != nil {
				t.Fatalf("unexpected: %+v", err)
			}
			if c.bad != "" && (err == nil || err.Fields[c.bad] == "") {
				t.Fatalf("expected %q error, got %+v", c.bad, err)
			}
		})
	}
}

func TestVoteAndObjectID(t *testing.T) {
	if (&VoteInput{OptionID: ""}).Validate() == nil {
		t.Fatal("empty optionId must fail")
	}
	if _, err := ParseObjectID("not-an-id"); err == nil {
		t.Fatal("bad id must fail")
	}
	if _, err := ParseObjectID("507f1f77bcf86cd799439011"); err != nil {
		t.Fatalf("valid id rejected: %v", err)
	}
}
