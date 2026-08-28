package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestSessionStoreCopiesMutableState(t *testing.T) {
	store := &sessionStore{sessions: make(map[string]*SessionData)}
	original := &SessionData{
		Modes: []string{"traefik"},
		Options: map[string]interface{}{
			"healthCheck": true,
			"nested":      map[string]interface{}{"value": "original"},
		},
		EnvOverrides:  map[string]string{"AUTH_HOST": "auth.example.com"},
		KeysOverrides: map[string]string{"API_KEY": "secret"},
		ImportApplied: &ImportApplied{
			EnvVars:        map[string]string{"IMPORTED": "value"},
			SuggestedModes: []string{"traefik-herald"},
		},
	}
	store.Set("session", original)

	original.Modes[0] = "changed"
	original.Options["healthCheck"] = false
	original.Options["nested"].(map[string]interface{})["value"] = "changed"
	original.EnvOverrides["AUTH_HOST"] = "changed.example.com"
	original.KeysOverrides["API_KEY"] = "changed"
	original.ImportApplied.EnvVars["IMPORTED"] = "changed"

	first, ok := store.Get("session")
	if !ok {
		t.Fatal("stored session not found")
	}
	if first.Modes[0] != "traefik" || first.Options["healthCheck"] != true ||
		first.Options["nested"].(map[string]interface{})["value"] != "original" ||
		first.EnvOverrides["AUTH_HOST"] != "auth.example.com" || first.KeysOverrides["API_KEY"] != "secret" ||
		first.ImportApplied.EnvVars["IMPORTED"] != "value" {
		t.Fatalf("store retained caller-owned mutable state: %#v", first)
	}

	first.EnvOverrides["AUTH_HOST"] = "mutated-after-get.example.com"
	first.Options["nested"].(map[string]interface{})["value"] = "mutated-after-get"
	second, ok := store.Get("session")
	if !ok || second.EnvOverrides["AUTH_HOST"] != "auth.example.com" ||
		second.Options["nested"].(map[string]interface{})["value"] != "original" {
		t.Fatalf("Get returned store-owned state: %#v", second)
	}
}

func TestSessionStoreConcurrentGetAndSet(t *testing.T) {
	store := &sessionStore{sessions: make(map[string]*SessionData)}
	store.Set("session", &SessionData{EnvOverrides: map[string]string{"COUNT": "0"}})

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				data, ok := store.Get("session")
				if !ok {
					t.Errorf("worker %d: session disappeared", worker)
					return
				}
				data.EnvOverrides["COUNT"] = fmt.Sprintf("%d-%d", worker, iteration)
				store.Set("session", data)
			}
		}(worker)
	}
	wg.Wait()

	if _, ok := store.Get("session"); !ok {
		t.Fatal("session missing after concurrent access")
	}
}
