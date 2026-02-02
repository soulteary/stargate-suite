// Package main: serve command and Web UI (loadPageData, /api/generate handler).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/the-gate/internal/composegen"
	"gopkg.in/yaml.v3"
)

func cacheControlHandler(value string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}

func loadPageData(yamlPath string) (*pageData, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	var raw pageYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	jsonI18N, err := json.Marshal(raw.I18N)
	if err != nil {
		return nil, err
	}
	title := "Stargate Suite - Compose 生成"
	if t, ok := raw.I18N["zh"]["title"]; ok && t != "" {
		title = t
	}
	return &pageData{
		I18N:           template.JS(jsonI18N),
		Title:          title,
		Lang:           "zh-CN",
		Modes:          raw.Modes,
		ConfigSections: raw.ConfigSections,
		Services:       raw.Services,
		Providers:      raw.Providers,
	}, nil
}

func cmdServe() error {
	root := projectRoot()
	pagePath := filepath.Join(root, pageYAMLPath)
	page, err := loadPageData(pagePath)
	if err != nil {
		if cwd, e := os.Getwd(); e == nil {
			fallback := filepath.Join(cwd, pageYAMLPath)
			page, err = loadPageData(fallback)
		}
		if err != nil {
			return fmt.Errorf("load page config (tried %s and ./%s): %w", pagePath, pageYAMLPath, err)
		}
	}
	tmpl, err := template.ParseFS(staticFS, "static/index.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse index template: %w", err)
	}
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static sub FS: %w", err)
	}
	cacheStatic := "public, max-age=3600"
	staticHandler := cacheControlHandler(cacheStatic, http.FileServer(http.FS(subFS)))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, page); err != nil {
			fmt.Fprintf(os.Stderr, "template execute: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	})
	mux.Handle("/static/", http.StripPrefix("/static", staticHandler))
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if len(req.Modes) == 0 {
			http.Error(w, "modes required", http.StatusBadRequest)
			return
		}
		root := projectRoot()
		fullPath := filepath.Join(root, canonicalCompose)
		full, err := composegen.LoadCompose(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load compose: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		opts := reqOptionsToComposegen(req.Options)
		gen, err := composegen.Generate(full, req.Modes, req.EnvOverride, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		res := map[string]interface{}{
			"composes": make(map[string]string),
			"env":      string(gen.Env),
		}
		for mode, yml := range gen.Composes {
			res["composes"].(map[string]string)[mode] = string(yml)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	addr := ":" + servePort
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		}
	}()
	fmt.Printf("Web UI: http://localhost%s\n", addr)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	fmt.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
