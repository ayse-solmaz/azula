package auth

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	SessionCookie = "azula_session"
	UICookie      = "azula_ui"
)

type writerKey struct{}
type requestKey struct{}

func WithWriter(ctx context.Context, w http.ResponseWriter) context.Context {
	return context.WithValue(ctx, writerKey{}, w)
}

func WriterFrom(ctx context.Context) http.ResponseWriter {
	w, _ := ctx.Value(writerKey{}).(http.ResponseWriter)
	return w
}

func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

func RequestFrom(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	return r
}

func IssueSession(ctx context.Context, webURL, token string, ttl time.Duration) {
	SetSessionCookies(WriterFrom(ctx), RequestFrom(ctx), webURL, token, ttl)
}

func ClearSession(ctx context.Context, webURL string) {
	ClearSessionCookies(WriterFrom(ctx), RequestFrom(ctx), webURL)
}

func cookiesSecure(webURL string, r *http.Request) bool {
	if strings.HasPrefix(strings.ToLower(webURL), "https://") {
		return true
	}
	return r != nil && r.TLS != nil
}

func SetSessionCookies(w http.ResponseWriter, r *http.Request, webURL, token string, ttl time.Duration) {
	if w == nil || token == "" {
		return
	}
	secure := cookiesSecure(webURL, r)
	maxAge := int(ttl.Seconds())
	if maxAge < 60 {
		maxAge = 60
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     UICookie,
		Value:    "1",
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookies(w http.ResponseWriter, r *http.Request, webURL string) {
	if w == nil {
		return
	}
	secure := cookiesSecure(webURL, r)
	expired := &http.Cookie{Path: "/", MaxAge: -1, Secure: secure, SameSite: http.SameSiteLaxMode}
	c1 := *expired
	c1.Name = SessionCookie
	c1.HttpOnly = true
	c2 := *expired
	c2.Name = UICookie
	http.SetCookie(w, &c1)
	http.SetCookie(w, &c2)
}

func TokenFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if raw != "" {
			return raw
		}
	}
	if c, err := r.Cookie(SessionCookie); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return ""
}
