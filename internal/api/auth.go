package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authcore "github.com/xiaobei/singbox-manager/internal/auth"
	"github.com/xiaobei/singbox-manager/internal/storage"
)

type authStatus struct {
	Bootstrapped  bool `json:"bootstrapped"`
	Authenticated bool `json:"authenticated"`
}

type loginRequest struct {
	Password string `json:"password"`
}

type bootstrapRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (s *Server) registerPublicAuthRoutes(api *gin.RouterGroup) {
	api.GET("/auth/me", s.getAuthStatus)
	api.POST("/auth/bootstrap", s.bootstrapAuthentication)
	api.POST("/auth/login", s.login)
	api.POST("/auth/logout", s.logout)
}

func (s *Server) requireAuthentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		status, err := s.resolveAuthStatus(c)
		if err != nil {
			writeAuthError(c, err)
			return
		}
		if !status.Authenticated {
			writeAuthenticationRequired(c)
			return
		}

		c.Next()
	}
}

func (s *Server) getAuthStatus(c *gin.Context) {
	status, err := s.resolveAuthStatus(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": status})
}

func (s *Server) bootstrapAuthentication(c *gin.Context) {
	if s.isAuthenticationBootstrapped() {
		c.JSON(http.StatusConflict, gin.H{"error": "authentication has already been initialized"})
		return
	}

	var req bootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password confirmation does not match"})
		return
	}

	hashedPassword, err := authcore.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings := s.cloneSettings()
	settings.AdminPasswordHash = hashedPassword
	settings.AuthBootstrappedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.store.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.writeSessionCookie(c, settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": authStatus{Bootstrapped: true, Authenticated: true}})
}

func (s *Server) login(c *gin.Context) {
	settings := s.store.GetSettings()
	if strings.TrimSpace(settings.AdminPasswordHash) == "" {
		writeBootstrapRequired(c)
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := authcore.CheckPassword(settings.AdminPasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := s.writeSessionCookie(c, *settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": authStatus{Bootstrapped: true, Authenticated: true}})
}

func (s *Server) logout(c *gin.Context) {
	s.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (s *Server) resolveAuthStatus(c *gin.Context) (authStatus, error) {
	if !s.isAuthenticationBootstrapped() {
		return authStatus{Bootstrapped: false, Authenticated: false}, nil
	}

	cookie, err := c.Cookie(authcore.CookieName)
	if err != nil {
		return authStatus{Bootstrapped: true, Authenticated: false}, nil
	}

	settings := s.store.GetSettings()
	if _, err := authcore.ValidateSessionToken(cookie, settings.AdminPasswordHash, time.Now()); err != nil {
		if errors.Is(err, authcore.ErrSessionExpired) || errors.Is(err, authcore.ErrSessionInvalid) {
			s.clearSessionCookie(c)
			return authStatus{Bootstrapped: true, Authenticated: false}, nil
		}
		return authStatus{}, err
	}

	return authStatus{Bootstrapped: true, Authenticated: true}, nil
}

func (s *Server) isAuthenticationBootstrapped() bool {
	settings := s.store.GetSettings()
	if settings == nil {
		return false
	}

	return strings.TrimSpace(settings.AdminPasswordHash) != ""
}

func (s *Server) cloneSettings() storage.Settings {
	currentSettings := s.store.GetSettings()
	if currentSettings == nil {
		return *storage.DefaultSettings()
	}

	clonedSettings := *currentSettings
	return clonedSettings
}

func (s *Server) writeSessionCookie(c *gin.Context, settings storage.Settings) error {
	token, err := authcore.IssueSessionToken(settings.AdminPasswordHash, s.sessionTTL(settings), time.Now())
	if err != nil {
		return err
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authcore.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(s.sessionTTL(settings).Seconds()),
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(c.Request),
	})
	return nil
}

func (s *Server) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authcore.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(c.Request),
	})
}

func (s *Server) sessionTTL(settings storage.Settings) time.Duration {
	if settings.SessionTTLMinutes <= 0 {
		return time.Duration(storage.DefaultSettings().SessionTTLMinutes) * time.Minute
	}

	return time.Duration(settings.SessionTTLMinutes) * time.Minute
}

func writeAuthError(c *gin.Context, err error) {
	if errors.Is(err, authcore.ErrSessionExpired) {
		writeAuthenticationRequired(c)
		return
	}
	if errors.Is(err, authcore.ErrSessionInvalid) {
		writeAuthenticationRequired(c)
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	c.Abort()
}

func writeBootstrapRequired(c *gin.Context) {
	c.JSON(http.StatusPreconditionRequired, gin.H{"error": "authentication bootstrap required"})
	c.Abort()
}

func writeAuthenticationRequired(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	c.Abort()
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
