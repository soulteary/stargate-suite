package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWebUIImageDefaultsComeFromManifest(t *testing.T) {
	resetConfigDir(t)
	configDirOverride = ""

	page, err := loadPageData(pageYAMLPath)
	if err != nil {
		t.Fatalf("load page data: %v", err)
	}
	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	type imageValue struct {
		defaultValue string
		placeholder  string
	}
	actual := make(map[string]imageValue)
	record := func(envName string, defaultValue interface{}, placeholder string) {
		if !strings.HasSuffix(envName, "_IMAGE") {
			return
		}
		value := imageValue{defaultValue: fmt.Sprint(defaultValue), placeholder: placeholder}
		if previous, ok := actual[envName]; ok && previous != value {
			t.Fatalf("image %s has inconsistent Web UI defaults: %#v and %#v", envName, previous, value)
		}
		actual[envName] = value
	}
	for _, section := range page.ConfigSections {
		for _, option := range section.Options {
			if option.Type == "imageEnv" {
				record(option.EnvName, option.Default, option.Placeholder)
			}
		}
	}
	for _, serviceGroup := range [][]pageService{page.Services, page.Providers} {
		for _, service := range serviceGroup {
			for _, section := range service.Sections {
				for _, variable := range section.EnvVars {
					record(variable.Env, variable.Default, variable.Placeholder)
				}
			}
		}
	}

	if len(actual) != len(pageImageSources) {
		t.Fatalf("found %d Web UI image fields, want %d: %#v", len(actual), len(pageImageSources), actual)
	}
	for envName, source := range pageImageSources {
		expected := ""
		if source.component != "" {
			expected = manifest.Components[source.component].Ref()
		} else {
			expected = manifest.Dependencies[source.dependency].Ref()
		}
		got, ok := actual[envName]
		if !ok {
			t.Errorf("Web UI is missing manifest-backed image field %s", envName)
			continue
		}
		if got.defaultValue != expected || got.placeholder != expected {
			t.Errorf("%s = default %q, placeholder %q; want %q from components.yaml", envName, got.defaultValue, got.placeholder, expected)
		}
	}
}

func TestWizardRenderRestoresImportedImageAndIncludesDescriptionFallback(t *testing.T) {
	resetConfigDir(t)
	configDirOverride = ""

	page, err := loadPageData(pageYAMLPath)
	if err != nil {
		t.Fatalf("load page data: %v", err)
	}
	tmpl, err := parsePageTemplates(page)
	if err != nil {
		t.Fatalf("parse page templates: %v", err)
	}

	clone := *page
	clone.Page = "wizard-2"
	clone.PageContent = "content-wizard-2"
	clone.Session = &SessionData{EnvOverrides: map[string]string{
		"STARGATE_IMAGE": "registry.example/stargate:imported",
	}}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "base", &clone); err != nil {
		t.Fatalf("render wizard: %v", err)
	}
	html := output.String()
	if !strings.Contains(html, `value="registry.example/stargate:imported"`) {
		t.Fatal("rendered wizard did not restore the imported STARGATE_IMAGE value")
	}
	if !strings.Contains(html, `data-env="STARGATE_IMAGE" data-session-value="true"`) {
		t.Fatal("rendered imported value was not protected from client-side scenario defaults")
	}
	if !strings.Contains(html, "Redis 数据使用 Docker 命名卷，便于备份与迁移。") {
		t.Fatal("rendered wizard omitted the server-side redisVolumeDesc fallback")
	}
}

func TestValidatePageI18NRejectsMissingDescription(t *testing.T) {
	page := &pageYAML{
		I18N: map[string]map[string]string{
			"zh": {"modeLabel": "模式", "redisVolumeDesc": "描述"},
			"en": {"modeLabel": "Mode"},
		},
		Modes: []pageMode{{LabelKey: "modeLabel", DescKey: "redisVolumeDesc"}},
	}
	err := validatePageI18N(page, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "en.redisVolumeDesc") {
		t.Fatalf("validatePageI18N error = %v, want missing en.redisVolumeDesc", err)
	}
}

func TestWizardStepLabelNavigationSavesBeforeRedirect(t *testing.T) {
	sessionID := "test-step-label-navigation"
	defaultStore.Delete(sessionID)
	t.Cleanup(func() { defaultStore.Delete(sessionID) })
	session := &SessionData{}

	form := url.Values{
		"STARGATE_IMAGE": {"registry.example/stargate:saved"},
		"redisVolume":    {"true"},
		"_next":          {"/wizard/step-4"},
	}
	request := httptest.NewRequest(http.MethodPost, "/wizard/step-2", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(WithSessionID(WithSession(request.Context(), session), sessionID))
	response := httptest.NewRecorder()

	handleWizardStepPost(response, request, 2)
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/wizard/step-4" {
		t.Fatalf("response = %d Location %q, want 302 /wizard/step-4", response.Code, response.Header().Get("Location"))
	}
	stored, ok := defaultStore.Get(sessionID)
	if !ok {
		t.Fatal("wizard submission was not saved")
	}
	if stored.EnvOverrides["STARGATE_IMAGE"] != "registry.example/stargate:saved" {
		t.Fatalf("stored STARGATE_IMAGE = %q", stored.EnvOverrides["STARGATE_IMAGE"])
	}
	if stored.Options["redisVolume"] != true {
		t.Fatalf("stored redisVolume = %#v, want true", stored.Options["redisVolume"])
	}
	if _, exists := stored.Options["_next"]; exists {
		t.Fatal("navigation target leaked into stored options")
	}
}

func TestWizardRedirectAllowlist(t *testing.T) {
	if got := defaultWizardNext(5); got != "/keys" {
		t.Fatalf("defaultWizardNext(5) = %q, want /keys", got)
	}
	for _, target := range []string{"/wizard/step-1", "/wizard/step-5", "/keys", "/review"} {
		if got := allowedWizardRedirect(target, "/wizard/step-3"); got != target {
			t.Errorf("allowedWizardRedirect(%q) = %q", target, got)
		}
	}
	for _, target := range []string{"https://example.com", "//example.com", "/import", "javascript:alert(1)"} {
		if got := allowedWizardRedirect(target, "/wizard/step-3"); got != "/wizard/step-3" {
			t.Errorf("allowedWizardRedirect(%q) = %q, want fallback", target, got)
		}
	}
}

func TestKeysApplyReturnsStatusAndPersistsValues(t *testing.T) {
	sessionID := "test-keys-apply"
	defaultStore.Delete(sessionID)
	t.Cleanup(func() { defaultStore.Delete(sessionID) })
	session := &SessionData{}
	request := httptest.NewRequest(http.MethodPost, "/keys/apply", strings.NewReader(`{"HERALD_API_KEY":"test-value"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithSessionID(WithSession(request.Context(), session), sessionID))
	response := httptest.NewRecorder()

	handleKeysApply(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("keys apply status = %d, want 204", response.Code)
	}
	stored, ok := defaultStore.Get(sessionID)
	if !ok || stored.KeysOverrides["HERALD_API_KEY"] != "test-value" {
		t.Fatalf("stored keys = %#v", stored)
	}

	missingSession := httptest.NewRecorder()
	handleKeysApply(missingSession, httptest.NewRequest(http.MethodPost, "/keys/apply", strings.NewReader(`{}`)))
	if missingSession.Code != http.StatusUnauthorized {
		t.Fatalf("missing-session status = %d, want 401", missingSession.Code)
	}
}

func TestWebUIIncludesDropAndSaveNavigationHooks(t *testing.T) {
	appJS, err := staticFS.ReadFile("static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	for _, hook := range []string{"bindImportDrop()", "form.requestSubmit()", `next.name = '_next'`, "saveKeysAndNavigate", "data-session-value"} {
		if !bytes.Contains(appJS, []byte(hook)) {
			t.Errorf("app.js is missing interaction hook %q", hook)
		}
	}
	importTemplate, err := staticFS.ReadFile("static/pages/import.tmpl")
	if err != nil {
		t.Fatalf("read import template: %v", err)
	}
	if !bytes.Contains(importTemplate, []byte("import-drop-target")) {
		t.Fatal("import textareas are missing drag-and-drop targets")
	}
}
