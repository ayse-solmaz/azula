package mcp

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/ayse-solmaz/azula/internal/domain"
)

func validateCloneURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domain.ErrInvalidInput
	}
	if strings.Contains(raw, "..") {
		return domain.ErrPathTraversal
	}
	u, err := url.Parse(raw)
	if err != nil {
		return domain.ErrInvalidInput
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w: only https git URLs are allowed", domain.ErrInvalidInput)
	}
	if u.User != nil {
		return fmt.Errorf("%w: credentials in git URLs are not allowed", domain.ErrInvalidInput)
	}
	if u.Host == "" || strings.EqualFold(u.Scheme, "file") || strings.HasPrefix(raw, "git@") {
		return domain.ErrInvalidInput
	}
	return denyPrivateHost(u.Hostname())
}

func denyPrivateHost(host string) error {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "localhost" || host == "metadata.google.internal" || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("%w: git host is not allowed", domain.ErrInvalidInput)
	}
	if ip := net.ParseIP(host); ip != nil {
		if ipPrivateOrLocal(ip) {
			return fmt.Errorf("%w: git host is not allowed", domain.ErrInvalidInput)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("%w: git host could not be resolved", domain.ErrInvalidInput)
	}
	for _, ip := range ips {
		if ipPrivateOrLocal(ip) {
			return fmt.Errorf("%w: git host resolves to a private address", domain.ErrInvalidInput)
		}
	}
	return nil
}

func ipPrivateOrLocal(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}
