package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/middleware"
	"github.com/your-username/live-polling-app/backend/internal/services"
	"github.com/your-username/live-polling-app/backend/internal/validation"
)

type AuthHandler struct {
	Base
	svc *services.AuthService
	ck  Cookies
}

func NewAuthHandler(b Base, s *services.AuthService, ck Cookies) *AuthHandler {
	return &AuthHandler{Base: b, svc: s, ck: ck}
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var in validation.SignupInput
	if !h.bind(c, &in) {
		return
	}
	u, tok, err := h.svc.Signup(c.Request.Context(), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ck.set(c, middleware.AuthCookie, tok, h.svc.TTL())
	ok(c, 201, u)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var in validation.LoginInput
	if !h.bind(c, &in) {
		return
	}
	u, tok, err := h.svc.Login(c.Request.Context(), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ck.set(c, middleware.AuthCookie, tok, h.svc.TTL())
	ok(c, 200, u)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.ck.clear(c, middleware.AuthCookie)
	ok(c, 200, gin.H{"loggedOut": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	id, found := middleware.UserID(c)
	if !found {
		h.fail(c, apperr.ErrUnauthorized)
		return
	}
	u, err := h.svc.Me(c.Request.Context(), id)
	if err != nil {
		h.fail(c, apperr.ErrUnauthorized) // deleted user with valid token
		return
	}
	ok(c, 200, u)
}
