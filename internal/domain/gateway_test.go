package domain

import "testing"

func TestMatch(t *testing.T) {
	routes := []Route{
		{Name: "identity", Prefix: "/api/identity", Target: "http://identity:8080"},
		{Name: "stock", Prefix: "/api/stock", Target: "http://stock:8080"},
		{Name: "cashflow", Prefix: "/api/cashflow", Target: "http://cashflow:8080"},
		{Name: "assets", Prefix: "/api/assets", Target: "http://assets:8080"},
		{Name: "config", Prefix: "/api/config", Target: "http://config:8080"},
	}
	r, rest, err := Match(routes, "/api/identity/auth/login")
	if err != nil || r.Name != "identity" || rest != "/auth/login" {
		t.Fatalf("got %+v %s %v", r, rest, err)
	}
	r, rest, err = Match(routes, "/api/stock")
	if err != nil || r.Name != "stock" || rest != "/" {
		t.Fatalf("got %+v %s %v", r, rest, err)
	}
	r, rest, err = Match(routes, "/api/cashflow/entries")
	if err != nil || r.Name != "cashflow" || rest != "/entries" {
		t.Fatalf("got %+v %s %v", r, rest, err)
	}
	r, rest, err = Match(routes, "/api/config/payment-terms")
	if err != nil || r.Name != "config" || rest != "/payment-terms" {
		t.Fatalf("got %+v %s %v", r, rest, err)
	}
	if _, _, err = Match(routes, "/api/unknown/x"); err != ErrNotFound {
		t.Fatalf("want not found, got %v", err)
	}
}
