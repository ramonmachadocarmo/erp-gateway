package domain

import (
	"errors"
	"strings"
)

var (
	ErrNotFound       = errors.New("unknown route")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrSessionRevoked = errors.New("session revoked")
)

type Route struct {
	Name   string
	Prefix string
	Target string
	// Public lists rest-paths (Match()'s stripped-prefix output) that bypass auth
	// entirely, e.g. "/auth/login".
	Public []string
	// AuthOnly lists rest-paths that need a valid, non-revoked session but skip
	// the module-permission check, e.g. "/auth/me", "/auth/logout".
	AuthOnly []string
}

func Match(routes []Route, path string) (Route, string, error) {
	best := Route{}
	found := false
	for _, r := range routes {
		if path == r.Prefix || strings.HasPrefix(path, r.Prefix+"/") {
			if !found || len(r.Prefix) > len(best.Prefix) {
				best = r
				found = true
			}
		}
	}
	if !found {
		return Route{}, "", ErrNotFound
	}
	rest := strings.TrimPrefix(path, best.Prefix)
	if rest == "" {
		rest = "/"
	}
	return best, rest, nil
}
