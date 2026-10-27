package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/middleware"
	"github.com/your-username/live-polling-app/backend/internal/models"
	"github.com/your-username/live-polling-app/backend/internal/services"
	"github.com/your-username/live-polling-app/backend/internal/validation"
)

type PollHandler struct {
	Base
	polls  *services.PollService
	votes  *services.VoteService
	ck     Cookies
	appURL string
}

func NewPollHandler(b Base, p *services.PollService, v *services.VoteService, ck Cookies, appURL string) *PollHandler {
	return &PollHandler{Base: b, polls: p, votes: v, ck: ck, appURL: strings.TrimRight(appURL, "/")}
}

// PollView never exposes ownerId.
type PollView struct {
	ID          string         `json:"id"`
	Question    string         `json:"question"`
	Description string         `json:"description,omitempty"`
	Options     []models.Option `json:"options"`
	Status      models.PollStatus `json:"status"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	IsOwner     bool           `json:"isOwner"`
	ShareURL    string         `json:"shareUrl"`
	Results     *models.Results `json:"results,omitempty"`
	HasVoted    *bool          `json:"hasVoted,omitempty"`
	MyOptionID  string         `json:"myOptionId,omitempty"`
}

func (h *PollHandler) view(p *models.Poll, caller string, res *models.Results) PollView {
	return PollView{
		ID: p.ID.Hex(), Question: p.Question, Description: p.Description, Options: p.Options,
		Status: services.EffectiveStatus(p, time.Now()), ExpiresAt: p.ExpiresAt, CreatedAt: p.CreatedAt,
		IsOwner: caller != "" && caller == p.OwnerID.Hex(), ShareURL: h.appURL + "/p/" + p.ID.Hex(), Results: res,
	}
}

func callerHex(c *gin.Context) string {
	if id, ok := middleware.UserID(c); ok {
		return id.Hex()
	}
	return ""
}

func (h *PollHandler) Create(c *gin.Context) {
	owner, _ := middleware.UserID(c)
	var in validation.CreatePollInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.polls.Create(c.Request.Context(), owner, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusCreated, h.view(p, owner.Hex(), nil))
}

func (h *PollHandler) List(c *gin.Context) {
	owner, _ := middleware.UserID(c)
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "12"), 10, 64)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 12
	}
	polls, total, err := h.polls.List(c.Request.Context(), owner, page, size)
	if err != nil {
		h.fail(c, err)
		return
	}
	items := make([]PollView, 0, len(polls))
	for i := range polls {
		res, rerr := h.polls.Results(c.Request.Context(), &polls[i])
		if rerr != nil {
			h.Log.Warn("results unavailable for list", "poll", polls[i].ID.Hex(), "err", rerr)
		}
		items = append(items, h.view(&polls[i], owner.Hex(), res))
	}
	ok(c, 200, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

// Get is public: anyone with the link can view; owner flag and vote state are caller-specific.
func (h *PollHandler) Get(c *gin.Context) {
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	p, err := h.polls.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	res, err := h.polls.Results(c.Request.Context(), p)
	if err != nil {
		h.fail(c, err)
		return
	}
	v := h.view(p, callerHex(c), res)
	voted, opt, verr := h.votes.MyVote(c.Request.Context(), id, h.ck.VoterID(c))
	if verr == nil {
		v.HasVoted, v.MyOptionID = &voted, opt
	}
	ok(c, 200, v)
}

func (h *PollHandler) Results(c *gin.Context) {
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	p, err := h.polls.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	res, err := h.polls.Results(c.Request.Context(), p)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, 200, res)
}

func (h *PollHandler) Share(c *gin.Context) {
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	if _, err := h.polls.Get(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	ok(c, 200, gin.H{"shareUrl": h.appURL + "/p/" + id.Hex()})
}

func (h *PollHandler) setStatus(c *gin.Context, st models.PollStatus) {
	owner, _ := middleware.UserID(c)
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	p, err := h.polls.SetStatus(c.Request.Context(), id, owner, st)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, 200, h.view(p, owner.Hex(), nil))
}

func (h *PollHandler) Close(c *gin.Context)  { h.setStatus(c, models.PollClosed) }
func (h *PollHandler) Reopen(c *gin.Context) { h.setStatus(c, models.PollOpen) }

func (h *PollHandler) Delete(c *gin.Context) {
	owner, _ := middleware.UserID(c)
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	if err := h.polls.Delete(c.Request.Context(), id, owner); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ExportCSV (owner only). Cells starting with = + - @ are prefixed to block spreadsheet formula injection.
func (h *PollHandler) ExportCSV(c *gin.Context) {
	owner, _ := middleware.UserID(c)
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	p, err := h.polls.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	if p.OwnerID != owner {
		h.fail(c, apperr.ErrForbidden)
		return
	}
	res, err := h.polls.Results(c.Request.Context(), p)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="poll-%s.csv"`, id.Hex()))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"option", "votes", "percent"})
	for _, o := range p.Options {
		n := res.Counts[o.ID]
		pct := 0.0
		if res.Total > 0 {
			pct = float64(n) * 100 / float64(res.Total)
		}
		_ = w.Write([]string{csvSafe(o.Text), strconv.FormatInt(n, 10), fmt.Sprintf("%.1f", pct)})
	}
	w.Flush()
}

func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (h *PollHandler) Vote(c *gin.Context) {
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		h.fail(c, e)
		return
	}
	var in validation.VoteInput
	if !h.bind(c, &in) {
		return
	}
	res, err := h.votes.Cast(c.Request.Context(), id, h.ck.VoterID(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, http.StatusCreated, gin.H{"optionId": in.OptionID, "results": res})
}
