package sso

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/ayse-solmaz/azula/internal/auth"
	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

type discovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type Handler struct {
	cfg  config.Config
	auth *auth.Service
}

func New(cfg config.Config, authSvc *auth.Service) *Handler {
	return &Handler{cfg: cfg, auth: authSvc}
}

func (h *Handler) Enabled() bool {
	return h.cfg.OIDCIssuer != "" && h.cfg.OIDCClientID != "" && h.cfg.OIDCClientSecret != ""
}

func (h *Handler) redirectURL() string {
	if h.cfg.OIDCRedirectURL != "" {
		return h.cfg.OIDCRedirectURL
	}
	return "http://localhost:" + h.cfg.APIPort + "/auth/oidc/callback"
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled() {
		http.Error(w, "sso is not configured", http.StatusNotFound)
		return
	}
	disc, err := fetchDiscovery(r.Context(), h.cfg.OIDCIssuer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	nonce := randomHex(16)
	state, err := h.signState(map[string]string{
		"n":  randomHex(16),
		"nc": nonce,
		"d":  r.URL.Query().Get("deviceId"),
		"dn": r.URL.Query().Get("deviceName"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := url.Values{}
	q.Set("client_id", h.cfg.OIDCClientID)
	q.Set("redirect_uri", h.redirectURL())
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	http.Redirect(w, r, disc.AuthorizationEndpoint+"?"+q.Encode(), http.StatusFound)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled() {
		http.Error(w, "sso is not configured", http.StatusNotFound)
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	state := r.URL.Query().Get("state")
	claims, err := h.parseState(state)
	if err != nil {
		http.Error(w, "invalid sso state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	disc, err := fetchDiscovery(r.Context(), h.cfg.OIDCIssuer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	email, sub, nonce, err := h.exchange(r.Context(), disc, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if want := claims["nc"]; want != "" && want != nonce {
		http.Error(w, "invalid sso nonce", http.StatusBadRequest)
		return
	}
	user, token, err := h.auth.LoginOrRegisterSSO(r.Context(), email, sub, claims["d"], claims["dn"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	_ = user
	auth.SetSessionCookies(w, r, h.cfg.WebURL, token, h.cfg.JWTExpiry)
	dest := strings.TrimRight(h.cfg.WebURL, "/") + "/login?sso=1"
	webHost := ""
	if wu, err := url.Parse(h.cfg.WebURL); err == nil {
		webHost = wu.Host
	}
	if webHost != "" && r.Host != webHost {
		dest += "&ssoToken=" + url.QueryEscape(token)
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (h *Handler) exchange(ctx context.Context, disc discovery, code string) (email, sub, nonce string, err error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.redirectURL())
	form.Set("client_id", h.cfg.OIDCClientID)
	form.Set("client_secret", h.cfg.OIDCClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("oidc token: %s", strings.TrimSpace(string(body)))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", "", "", err
	}
	if tok.IDToken == "" {
		return "", "", "", fmt.Errorf("oidc: missing id_token")
	}
	return parseIDToken(ctx, disc.JWKSURI, tok.IDToken, h.cfg.OIDCClientID, h.cfg.OIDCIssuer)
}

func (h *Handler) signState(data map[string]string) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (h *Handler) parseState(state string) (map[string]string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return nil, domain.ErrUnauthorized
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, domain.ErrUnauthorized
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func fetchDiscovery(ctx context.Context, issuer string) (discovery, error) {
	var d discovery
	u := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return d, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return d, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return d, fmt.Errorf("oidc discovery: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return d, err
	}
	return d, nil
}

func parseIDToken(ctx context.Context, jwksURL, raw, clientID, issuer string) (email, sub, nonce string, err error) {
	keys, err := fetchJWKS(ctx, jwksURL)
	if err != nil {
		return "", "", "", err
	}
	parsed, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			for _, k := range keys {
				return k, nil
			}
			return nil, domain.ErrUnauthorized
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !parsed.Valid {
		return "", "", "", domain.ErrUnauthorized
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", "", domain.ErrUnauthorized
	}
	if aud, _ := claims["aud"].(string); aud != "" && aud != clientID {
		return "", "", "", domain.ErrUnauthorized
	}
	if iss, _ := claims["iss"].(string); iss != "" && strings.TrimRight(iss, "/") != strings.TrimRight(issuer, "/") {
		return "", "", "", domain.ErrUnauthorized
	}
	email, _ = claims["email"].(string)
	if email == "" {
		email, _ = claims["preferred_username"].(string)
	}
	sub, _ = claims["sub"].(string)
	nonce, _ = claims["nonce"].(string)
	if email == "" || sub == "" {
		return "", "", "", fmt.Errorf("oidc token missing email or sub")
	}
	return strings.ToLower(email), sub, nonce, nil
}

type jwksDoc struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func fetchJWKS(ctx context.Context, jwksURL string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublic(k.N, k.E)
		if err != nil {
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("oidc: no rsa keys")
	}
	return out, nil
}

func rsaPublic(nB64, eB64 string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		nb, err = base64.URLEncoding.DecodeString(nB64)
		if err != nil {
			return nil, err
		}
	}
	eb, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		eb, err = base64.URLEncoding.DecodeString(eB64)
		if err != nil {
			return nil, err
		}
	}
	n := new(big.Int).SetBytes(nb)
	var eInt int
	if len(eb) < 8 {
		buf := make([]byte, 8)
		copy(buf[8-len(eb):], eb)
		eInt = int(binary.BigEndian.Uint64(buf))
	} else {
		eInt = int(new(big.Int).SetBytes(eb).Int64())
	}
	return &rsa.PublicKey{N: n, E: eInt}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
