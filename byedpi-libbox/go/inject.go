package byedpi

import (
	"encoding/json"
	"fmt"
)

const (
	byedpiTag       = "byedpi"
	byedpiHost      = "127.0.0.1"
	experimentalKey = "experimental"
)

var skipTypes = map[string]bool{
	"direct":    true,
	"block":     true,
	"dns":       true,
	"selector":  true,
	"urltest":   true,
	"hysteria":  true,
	"hysteria2": true,
	"tuic":      true,
	"wireguard": true,
}

func isSkipType(t string) bool { return skipTypes[t] }

// extractSettings reads experimental.byedpi from the profile JSON.
// Returns zero Settings when absent.
func extractSettings(content []byte) (Settings, error) {
	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return Settings{}, err
	}
	experimental, ok := config[experimentalKey].(map[string]any)
	if !ok {
		return Settings{}, nil
	}
	raw, ok := experimental["byedpi"]
	if !ok {
		return Settings{}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(encoded, &settings); err != nil {
		return Settings{}, fmt.Errorf("invalid experimental.byedpi block: %w", err)
	}
	return settings, nil
}

// injectConfig ports ByeDpiConfigInjector.inject: it removes any previous
// byedpi outbound and detours, appends a fresh local socks outbound and adds
// detour=byedpi to every eligible outbound.
func injectConfig(content []byte, port int) ([]byte, error) {
	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	rawOutbounds, exists := config["outbounds"]
	outbounds, ok := rawOutbounds.([]any)
	if !exists || !ok {
		outbounds = make([]any, 0)
	}

	// removeByeDpiOutbound + clearDetours
	cleaned := make([]any, 0, len(outbounds))
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			cleaned = append(cleaned, item)
			continue
		}
		if tagString(outbound["tag"]) == byedpiTag {
			continue // dropped entirely
		}
		if tagString(outbound["detour"]) == byedpiTag {
			delete(outbound, "detour")
		}
		cleaned = append(cleaned, outbound)
	}
	outbounds = cleaned

	// createByeDpiOutbound
	outbounds = append(outbounds, map[string]any{
		"type":        "socks",
		"tag":         byedpiTag,
		"server":      byedpiHost,
		"server_port": port,
		"network":     "tcp",
	})

	// injectDetour
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tag := tagString(outbound["tag"])
		typ := tagString(outbound["type"])
		if tag == byedpiTag || isSkipType(typ) {
			continue
		}
		injectDomainResolver(outbound)
		outbound["detour"] = byedpiTag
	}

	config["outbounds"] = outbounds
	return json.Marshal(config)
}

func tagString(value any) string {
	s, _ := value.(string)
	return s
}

// injectDomainResolver ports ByeDpiConfigInjector.injectDomainResolver:
// hostname servers resolve through the local resolver instead of letting
// ciadpi's native getaddrinfo handle them.
func injectDomainResolver(outbound map[string]any) {
	if _, has := outbound["domain_resolver"]; has {
		return
	}
	server := tagString(outbound["server"])
	if server == "" {
		return
	}
	// Kotlin quirk kept for behavior parity: numeric-only servers are skipped,
	// and anything containing ':' (IPv6 literals) is skipped.
	if isAllDigits(server) {
		return
	}
	if containsColon(server) {
		return
	}
	outbound["domain_resolver"] = "local"
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}

// PrepareAndStart is the single libbox hook:
//  1. detect experimental.byedpi in the profile content;
//  2. start/restart the embedded proxy when enabled;
//  3. return rewritten config with the byedpi detour injected.
//
// When the feature is absent/disabled the original content is returned
// untouched and any running instance is stopped.
func PrepareAndStart(configContent string) (string, error) {
	settings, err := extractSettings([]byte(configContent))
	if err != nil {
		// Not valid top-level JSON or unreadable marker: pass through.
		return configContent, nil
	}
	if !settings.Enabled {
		Stop()
		return configContent, nil
	}

	port := settings.ListenPort
	if port <= 0 {
		port = defaultPort
	}

	injected, err := injectConfig([]byte(configContent), port)
	if err != nil {
		return "", fmt.Errorf("byedpi: inject config: %w", err)
	}

	if err := RestartIfNeeded(settings); err != nil {
		return "", err
	}
	return string(injected), nil
}
