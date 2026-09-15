package httpadapter

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"erp/pkg/audit"
	"erp/pkg/httpserver"
	"erp/pkg/pagination"
	"erp/pkg/rbac"
	"erp/services/gateway-service/internal/application"
	"erp/services/gateway-service/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// auditedMethods are the mutations worth recording — reads (GET/HEAD/OPTIONS) and creates
// (POST) are left out on purpose: this log exists to answer "who deleted/changed this and
// when", not to be a full request log.
var auditedMethods = map[string]bool{http.MethodDelete: true, http.MethodPut: true, http.MethodPatch: true}

type Handler struct {
	svc       *application.Service
	proxies   map[string]*httputil.ReverseProxy
	jwtSecret string
	jwtIssuer string
	redis     *redis.Client
	audit     *audit.Logger
}

func New(svc *application.Service, routes []domain.Route, jwtSecret, jwtIssuer string, redisClient *redis.Client, auditLogger *audit.Logger) *Handler {
	h := &Handler{svc: svc, proxies: map[string]*httputil.ReverseProxy{}, jwtSecret: jwtSecret, jwtIssuer: jwtIssuer, redis: redisClient, audit: auditLogger}
	for _, r := range routes {
		target, err := url.Parse(r.Target)
		if err != nil {
			continue
		}
		p := httputil.NewSingleHostReverseProxy(target)
		p.Transport = &http.Transport{
			ResponseHeaderTimeout: 60 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		}
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
		}
		p.Director = func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		}
		h.proxies[r.Name] = p
	}
	return h
}

func (h *Handler) Register(r *gin.Engine) {
	r.Any("/api/*path", h.dispatch)
}

func (h *Handler) dispatch(c *gin.Context) {
	// Served directly by the gateway itself — it's the only thing holding the Mongo audit
	// connection, and there's no backend service for it to proxy to. Handled inline here
	// rather than as its own gin route, since a static route can't coexist with the
	// "/api/*path" catch-all registered in Register() below.
	if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/api/audit/logs" {
		h.listAuditLogs(c)
		return
	}
	route, rest, err := h.svc.Resolve(c.Request.URL.Path)
	if err != nil {
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	var claims *rbac.Claims
	if !contains(route.Public, rest) {
		var ok bool
		claims, ok = h.authenticate(c)
		if !ok {
			return
		}
		if !contains(route.AuthOnly, rest) && !h.authorize(c, claims, route.Name) {
			return
		}
	}
	proxy, ok := h.proxies[route.Name]
	if !ok {
		httpserver.Error(c, http.StatusBadGateway, domain.ErrNotFound)
		return
	}
	audited := h.audit != nil && auditedMethods[c.Request.Method]
	var body []byte
	if audited && c.Request.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(c.Request.Body, 64*1024))
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}
	fullPath := route.Prefix + rest
	method := c.Request.Method
	c.Request.URL.Path = rest
	c.Request.Header.Set("X-Forwarded-Prefix", route.Prefix)
	proxy.ServeHTTP(c.Writer, c.Request)
	if audited {
		entry := audit.Entry{
			Method: method, Module: route.Name, Path: fullPath,
			StatusCode: c.Writer.Status(), Body: string(body),
		}
		if claims != nil {
			entry.UserEmail = claims.Email
			entry.UserRole = claims.RoleCode
		}
		go h.audit.Log(entry)
	}
}

func (h *Handler) authenticate(c *gin.Context) (*rbac.Claims, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httpserver.Error(c, http.StatusUnauthorized, domain.ErrUnauthorized)
		return nil, false
	}
	claims := &rbac.Claims{}
	tok, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), claims, func(*jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	}, jwt.WithIssuer(h.jwtIssuer))
	if err != nil || !tok.Valid {
		httpserver.Error(c, http.StatusUnauthorized, domain.ErrUnauthorized)
		return nil, false
	}
	if !claims.IsMaster() {
		n, err := h.redis.Exists(c.Request.Context(), rbac.SessionKey(claims.SessionID)).Result()
		if err != nil || n == 0 {
			httpserver.Error(c, http.StatusUnauthorized, domain.ErrSessionRevoked)
			return nil, false
		}
	}
	return claims, true
}

func (h *Handler) authorize(c *gin.Context, claims *rbac.Claims, module string) bool {
	if claims.IsMaster() {
		return true
	}
	if claims.Modules[module] < rbac.RequiredLevel(c.Request.Method) {
		httpserver.Error(c, http.StatusForbidden, domain.ErrForbidden)
		return false
	}
	return true
}

func (h *Handler) listAuditLogs(c *gin.Context) {
	claims, ok := h.authenticate(c)
	if !ok {
		return
	}
	if !h.authorize(c, claims, "audit") {
		return
	}
	if h.audit == nil {
		c.JSON(http.StatusOK, pagination.New([]audit.Entry{}, 0, pagination.Params{Page: 1, Limit: 10}))
		return
	}
	p := pagination.Parse(c, 10, 100)
	entries, total, err := h.audit.List(c.Request.Context(), audit.Query{
		Module:    c.Query("module"),
		Method:    c.Query("method"),
		UserEmail: c.Query("user_email"),
	}, p)
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pagination.New(entries, total, p))
}

func contains(list []string, s string) bool {
	for _, p := range list {
		if p == s {
			return true
		}
	}
	return false
}
