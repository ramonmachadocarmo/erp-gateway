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
	// Grants lets another module's permission also authorize a rest-path prefix, on
	// top of this route's own module. It lets e.g. a sales user register a customer
	// address (config-service) without holding edit access to the whole Configurador.
	Grants []Grant
}

// Grant: holding Module at the level the request needs authorizes any rest-path
// equal to, or nested under, PathPrefix.
type Grant struct {
	PathPrefix string
	Module     string
}

// ModulesFor returns every module whose permission may authorize rest: the
// route's own first, then any matching grant.
func (r Route) ModulesFor(rest string) []string {
	mods := []string{r.Name}
	for _, g := range r.Grants {
		if rest == g.PathPrefix || strings.HasPrefix(rest, g.PathPrefix+"/") {
			mods = append(mods, g.Module)
		}
	}
	return mods
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
