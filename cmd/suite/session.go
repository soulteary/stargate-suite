// Package main: session store for multi-page wizard state.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	sessionCookieName = "stargate_suite_sid"
	sessionTTL        = 30 * time.Minute
	sessionIDLen      = 24
)

// SessionData holds wizard state across page refreshes.
type SessionData struct {
	// Wizard step 1
	Modes []string `json:"modes"`
	// Wizard step 2+ (options and env overrides)
	Options      map[string]interface{} `json:"options"` // option id -> value (bool, string, etc.)
	EnvOverrides map[string]string      `json:"envOverrides"`
	// Import apply result (from /import -> apply)
	ImportApplied *ImportApplied `json:"importApplied,omitempty"`
	// Keys apply (from /keys -> apply): env name -> value
	KeysOverrides map[string]string `json:"keysOverrides,omitempty"`
	// Timestamp for TTL
	ExpiresAt time.Time `json:"expiresAt"`
}

// ImportApplied is stored after user applies parsed compose/env.
type ImportApplied struct {
	EnvVars        map[string]string `json:"envVars"`
	SuggestedModes []string          `json:"suggestedModes"`
	SuggestedScene string            `json:"suggestedScene"`
}

type sessionContextKey struct{}
type sessionIDContextKey struct{}

// SessionID returns the current request's session ID from context.
func SessionID(ctx context.Context) (string, bool) {
	v := ctx.Value(sessionIDContextKey{})
	if v == nil {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

// WithSessionID returns context with session ID attached.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, id)
}

// sessionStore is an in-memory store with TTL cleanup.
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
}

var defaultStore = &sessionStore{sessions: make(map[string]*SessionData)}

func (s *sessionStore) Get(id string) (*SessionData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.sessions[id]
	if !ok || data == nil {
		return nil, false
	}
	if time.Now().After(data.ExpiresAt) {
		return nil, false
	}
	return data, true
}

func (s *sessionStore) Set(id string, data *SessionData) {
	if data == nil {
		return
	}
	data.ExpiresAt = time.Now().Add(sessionTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*SessionData)
	}
	s.sessions[id] = data
}

func (s *sessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// cleanupExpired removes expired sessions periodically.
func (s *sessionStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, data := range s.sessions {
		if data != nil && now.After(data.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

func newSessionID() (string, error) {
	b := make([]byte, sessionIDLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetSession returns session from context (set by middleware).
func GetSession(ctx context.Context) (*SessionData, bool) {
	v := ctx.Value(sessionContextKey{})
	if v == nil {
		return nil, false
	}
	data, ok := v.(*SessionData)
	return data, ok
}

// WithSession returns context with session attached.
func WithSession(ctx context.Context, data *SessionData) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, data)
}

// SaveSession persists session to store (call from handlers after mutating).
func SaveSession(ctx context.Context, data *SessionData) {
	id, ok := SessionID(ctx)
	if !ok || data == nil {
		return
	}
	defaultStore.Set(id, data)
}
