package validation

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	MinOptions = 2
	MaxOptions = 10
)

type SignupInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreatePollInput struct {
	Question    string     `json:"question"`
	Description string     `json:"description"`
	Options     []string   `json:"options"`
	ExpiresAt   *time.Time `json:"expiresAt"`
}

type VoteInput struct {
	OptionID string `json:"optionId"`
}

func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func between(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

func (in *SignupInput) Validate() *apperr.Error {
	f := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = NormalizeEmail(in.Email)
	if !between(in.Name, 1, 60) {
		f["name"] = "Name must be 1-60 characters"
	}
	if len(in.Email) > 254 {
		f["email"] = "Email is too long"
	} else if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email {
		f["email"] = "Enter a valid email address"
	}
	// bcrypt only uses the first 72 bytes; reject longer to avoid silent truncation.
	if len(in.Password) < 8 || len(in.Password) > 72 {
		f["password"] = "Password must be 8-72 bytes"
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	return nil
}

func (in *LoginInput) Validate() *apperr.Error {
	in.Email = NormalizeEmail(in.Email)
	if in.Email == "" || in.Password == "" || len(in.Email) > 254 || len(in.Password) > 72 {
		return apperr.Validation(map[string]string{"credentials": "Email and password are required"})
	}
	return nil
}

func (in *CreatePollInput) Validate(now time.Time) *apperr.Error {
	f := map[string]string{}
	in.Question = strings.TrimSpace(in.Question)
	in.Description = strings.TrimSpace(in.Description)
	if !between(in.Question, 3, 200) {
		f["question"] = "Question must be 3-200 characters"
	}
	if utf8.RuneCountInString(in.Description) > 500 {
		f["description"] = "Description must be at most 500 characters"
	}
	if len(in.Options) < MinOptions || len(in.Options) > MaxOptions {
		f["options"] = "Provide between 2 and 10 options"
	} else {
		seen := map[string]bool{}
		for i := range in.Options {
			in.Options[i] = strings.TrimSpace(in.Options[i])
			k := strings.ToLower(in.Options[i])
			if !between(in.Options[i], 1, 100) {
				f["options"] = "Each option must be 1-100 characters"
				break
			}
			if seen[k] {
				f["options"] = "Options must be unique"
				break
			}
			seen[k] = true
		}
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now.Add(time.Minute)) {
		f["expiresAt"] = "Expiry must be at least 1 minute in the future"
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	return nil
}

func (in *VoteInput) Validate() *apperr.Error {
	in.OptionID = strings.TrimSpace(in.OptionID)
	if in.OptionID == "" || len(in.OptionID) > 16 {
		return apperr.Validation(map[string]string{"optionId": "optionId is required"})
	}
	return nil
}

func ParseObjectID(s string) (primitive.ObjectID, *apperr.Error) {
	id, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		return primitive.NilObjectID, apperr.ErrBadID
	}
	return id, nil
}
