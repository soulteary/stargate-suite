package main

import (
	"fmt"
	"sort"
	"strings"
)

// validatePageI18N guarantees that every config-driven label, description and
// placeholder has non-empty text in both supported languages. Dynamic fields
// are rendered with a Chinese server-side fallback, so missing JavaScript or a
// stale cached asset cannot turn descriptions such as redisVolumeDesc blank.
func validatePageI18N(page *pageYAML, keyVars []envVar, ports []portDef) error {
	if page == nil {
		return fmt.Errorf("page config is required")
	}
	required := make(map[string][]string)
	add := func(key, source string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		required[key] = append(required[key], source)
	}
	requirePair := func(labelKey, descKey, source string) error {
		if strings.TrimSpace(labelKey) == "" {
			return fmt.Errorf("%s: labelKey is required", source)
		}
		if strings.TrimSpace(descKey) == "" {
			return fmt.Errorf("%s: descKey is required", source)
		}
		add(labelKey, source+" label")
		add(descKey, source+" description")
		return nil
	}

	for index, mode := range page.Modes {
		if err := requirePair(mode.LabelKey, mode.DescKey, fmt.Sprintf("modes[%d]", index)); err != nil {
			return err
		}
	}
	for sectionIndex, section := range page.ConfigSections {
		add(section.TitleKey, fmt.Sprintf("configSections[%d] title", sectionIndex))
		for optionIndex, option := range section.Options {
			source := fmt.Sprintf("configSections[%d].options[%d]", sectionIndex, optionIndex)
			if option.Type == "redisPaths" {
				for pathIndex, redisPath := range option.Paths {
					if err := requirePair(redisPath.LabelKey, redisPath.DescKey, fmt.Sprintf("%s.paths[%d]", source, pathIndex)); err != nil {
						return err
					}
				}
				continue
			}
			if err := requirePair(option.LabelKey, option.DescKey, source); err != nil {
				return err
			}
			add(option.PlaceholderKey, source+" placeholder")
			for selectIndex, selectOption := range option.Options {
				add(selectOption.LabelKey, fmt.Sprintf("%s.options[%d] label", source, selectIndex))
			}
		}
	}
	for _, group := range []struct {
		name     string
		services []pageService
	}{
		{name: "services", services: page.Services},
		{name: "providers", services: page.Providers},
	} {
		for serviceIndex, service := range group.services {
			add(service.NameKey, fmt.Sprintf("%s[%d] name", group.name, serviceIndex))
			for sectionIndex, section := range service.Sections {
				add(section.TitleKey, fmt.Sprintf("%s[%d].sections[%d] title", group.name, serviceIndex, sectionIndex))
				for variableIndex, variable := range section.EnvVars {
					source := fmt.Sprintf("%s[%d].sections[%d].envVars[%d]", group.name, serviceIndex, sectionIndex, variableIndex)
					if err := requirePair(variable.LabelKey, variable.DescKey, source); err != nil {
						return err
					}
					for selectIndex, selectOption := range variable.Options {
						add(selectOption.LabelKey, fmt.Sprintf("%s.options[%d] label", source, selectIndex))
					}
				}
			}
		}
	}
	for index, variable := range keyVars {
		if err := requirePair(variable.LabelKey, variable.DescKey, fmt.Sprintf("keysStepVars[%d]", index)); err != nil {
			return err
		}
	}
	for index, port := range ports {
		if err := requirePair(port.LabelKey, port.DescKey, fmt.Sprintf("ports[%d]", index)); err != nil {
			return err
		}
	}

	var missing []string
	for _, lang := range []string{"zh", "en"} {
		dictionary, ok := page.I18N[lang]
		if !ok {
			missing = append(missing, lang+" (language dictionary)")
			continue
		}
		for key := range required {
			if strings.TrimSpace(dictionary[key]) == "" {
				missing = append(missing, lang+"."+key)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing or empty i18n entries: %s", strings.Join(missing, ", "))
	}
	return nil
}
