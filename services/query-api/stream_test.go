package main

import "testing"

func TestOriginAllowedProductionUsesAllowListOnly(t *testing.T) {
	s := &Server{env: "production", cors: []string{"https://pulselog.example.com"}}
	if !s.originAllowed("https://pulselog.example.com") {
		t.Fatal("configured origin should be allowed")
	}
	if s.originAllowed("http://localhost:3000") {
		t.Fatal("localhost must not be implicitly allowed in production")
	}
	if s.originAllowed("https://evil.example") {
		t.Fatal("unknown origin must be rejected")
	}
}

func TestOriginAllowedDevelopmentKeepsLocalhost(t *testing.T) {
	s := &Server{env: "development"}
	if !s.originAllowed("http://127.0.0.1:3000") {
		t.Fatal("dev localhost should be allowed")
	}
	if s.originAllowed("https://evil.example") {
		t.Fatal("unknown origin must be rejected")
	}
}
