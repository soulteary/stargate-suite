package main

import (
	"fmt"
	"os"

	"github.com/soulteary/stargate-suite/internal/contract"
)

var lockImageEnvVars = map[string][]string{
	"stargate":        {"STARGATE_IMAGE"},
	"warden":          {"WARDEN_IMAGE"},
	"herald":          {"HERALD_IMAGE"},
	"herald-totp":     {"HERALD_TOTP_IMAGE"},
	"herald-dingtalk": {"HERALD_DINGTALK_IMAGE"},
	"herald-smtp":     {"HERALD_SMTP_IMAGE"},
	"redis":            {"HERALD_REDIS_IMAGE", "WARDEN_REDIS_IMAGE", "STARGATE_REDIS_IMAGE"},
	"protected":        {"PROTECTED_IMAGE"},
}

func loadLockedImageEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	lock, err := contract.ParseLock(data)
	if err != nil {
		return nil, err
	}
	manifest, err := loadManifest()
	if err != nil {
		return nil, fmt.Errorf("load component manifest: %w", err)
	}
	if err := contract.ValidateLock(manifest, lock, true); err != nil {
		return nil, err
	}

	env := make(map[string]string, len(lockImageEnvVars))
	for name, keys := range lockImageEnvVars {
		item, ok := lock.Images[name]
		if !ok {
			return nil, fmt.Errorf("lock is missing image %q", name)
		}
		ref := item.Image + "@" + item.Digest
		for _, key := range keys {
			env[key] = ref
		}
	}
	return env, nil
}
