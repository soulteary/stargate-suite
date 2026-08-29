// Package main: component manifest loading. config/components.yaml is the single
// source of truth for component versions, images, ports and health paths (M-01).
// This helper loads it from the active asset FS (embedded or --config-dir
// override) and feeds container ports into composegen so generation no longer
// hardcodes them.
package main

import (
	"fmt"
	"strings"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/contract"
)

type manifestImageSource struct {
	component  string
	dependency string
}

// pageImageSources maps Web UI env vars to the authoritative component
// manifest entry. Version strings must never be duplicated here.
var pageImageSources = map[string]manifestImageSource{
	"STARGATE_IMAGE":        {component: "stargate"},
	"WARDEN_IMAGE":          {component: "warden"},
	"HERALD_IMAGE":          {component: "herald"},
	"HERALD_TOTP_IMAGE":     {component: "herald-totp"},
	"HERALD_DINGTALK_IMAGE": {component: "herald-dingtalk"},
	"HERALD_SMTP_IMAGE":     {component: "herald-smtp"},
	"HERALD_REDIS_IMAGE":    {dependency: "redis"},
	"WARDEN_REDIS_IMAGE":    {dependency: "redis"},
	"STARGATE_REDIS_IMAGE":  {dependency: "redis"},
	"PROTECTED_IMAGE":       {dependency: "protected"},
}

// loadManifest reads config/components.yaml from the active asset FS.
func loadManifest() (*contract.Manifest, error) {
	return contract.LoadManifest(assetFS(), contract.ManifestPath)
}

// applyManifestToComposegen loads the authoritative manifest and pushes its
// container ports into composegen. Invalid overrides are fatal so generation
// cannot silently fall back to stale built-in values.
func applyManifestToComposegen() error {
	m, err := loadManifest()
	if err != nil {
		return err
	}
	ports := make(map[string]int, len(m.Components))
	for name, c := range m.Components {
		ports[name] = c.ContainerPort
	}
	composegen.SetContainerPorts(ports)
	return nil
}

// applyManifestToPage replaces every Web UI image default and placeholder with
// the ref from components.yaml. This makes imported/session values and the
// manifest authoritative; the split UI YAML files no longer carry release
// versions that can drift or remain stale in an embedded binary.
func applyManifestToPage(page *pageYAML, manifest *contract.Manifest) error {
	if page == nil || manifest == nil {
		return fmt.Errorf("page config and component manifest are required")
	}
	refs := make(map[string]string, len(pageImageSources))
	for envName, source := range pageImageSources {
		var ref string
		if source.component != "" {
			component, ok := manifest.Components[source.component]
			if !ok {
				return fmt.Errorf("Web UI image %s references missing component %q", envName, source.component)
			}
			ref = component.Ref()
		} else {
			dependency, ok := manifest.Dependencies[source.dependency]
			if !ok {
				return fmt.Errorf("Web UI image %s references missing dependency %q", envName, source.dependency)
			}
			ref = dependency.Ref()
		}
		if ref == "" {
			return fmt.Errorf("Web UI image %s resolved to an empty manifest ref", envName)
		}
		refs[envName] = ref
	}

	apply := func(envName string, defaultValue *interface{}, placeholder *string) error {
		if envName == "" || (!strings.HasSuffix(envName, "_IMAGE") && envName != "PROTECTED_IMAGE") {
			return nil
		}
		ref, ok := refs[envName]
		if !ok {
			return fmt.Errorf("Web UI image field %s has no components.yaml mapping", envName)
		}
		*defaultValue = ref
		*placeholder = ref
		return nil
	}

	for sectionIndex := range page.ConfigSections {
		for optionIndex := range page.ConfigSections[sectionIndex].Options {
			option := &page.ConfigSections[sectionIndex].Options[optionIndex]
			if option.Type != "imageEnv" {
				continue
			}
			if err := apply(option.EnvName, &option.Default, &option.Placeholder); err != nil {
				return err
			}
		}
	}
	for serviceIndex := range page.Services {
		if err := applyManifestToEnvVars(page.Services[serviceIndex].Sections, apply); err != nil {
			return err
		}
	}
	for providerIndex := range page.Providers {
		if err := applyManifestToEnvVars(page.Providers[providerIndex].Sections, apply); err != nil {
			return err
		}
	}
	return nil
}

func applyManifestToEnvVars(sections []pageSection, apply func(string, *interface{}, *string) error) error {
	for sectionIndex := range sections {
		for variableIndex := range sections[sectionIndex].EnvVars {
			variable := &sections[sectionIndex].EnvVars[variableIndex]
			if err := apply(variable.Env, &variable.Default, &variable.Placeholder); err != nil {
				return err
			}
		}
	}
	return nil
}
