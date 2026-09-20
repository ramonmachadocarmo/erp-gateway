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

func TestModulesForGrants(t *testing.T) {
	r := Route{Name: "config", Grants: []Grant{
		{PathPrefix: "/customers", Module: "sales"},
		{PathPrefix: "/suppliers", Module: "purchasing"},
	}}
	if got := r.ModulesFor("/suppliers/7"); len(got) != 2 || got[1] != "purchasing" {
		t.Fatalf("suppliers should also accept purchasing, got %v", got)
	}
	if got := r.ModulesFor("/customers/42"); len(got) != 2 || got[1] != "sales" {
		t.Fatalf("customers must not pick up purchasing, got %v", got)
	}
	if got := r.ModulesFor("/customers/42"); len(got) != 2 || got[1] != "sales" {
		t.Fatalf("customers should also accept sales, got %v", got)
	}
	if got := r.ModulesFor("/customers"); len(got) != 2 {
		t.Fatalf("exact prefix should match, got %v", got)
	}
	if got := r.ModulesFor("/customers-archive"); len(got) != 1 {
		t.Fatalf("prefix must match whole segments, got %v", got)
	}
	if got := r.ModulesFor("/units"); len(got) != 1 || got[0] != "config" {
		t.Fatalf("other paths stay config-only, got %v", got)
	}
}
