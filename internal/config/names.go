package config

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

var adjectives = []string{
	"swift", "quick", "silent", "hidden", "shadow",
	"bright", "dark", "rapid", "fast", "eager",
	"quiet", "stealth", "brave", "bold", "calm",
	"cool", "deep", "wild", "free", "pure",
	"safe", "sharp", "smart", "soft", "warm",
	"wise", "frost", "storm", "night", "dawn",
}

var nouns = []string{
	"tunnel", "stream", "channel", "bridge", "gateway",
	"path", "route", "link", "portal", "passage",
	"conduit", "relay", "proxy", "node", "point",
	"eagle", "falcon", "hawk", "raven", "wolf",
	"tiger", "lion", "bear", "fox", "owl",
	"river", "ocean", "cloud", "star", "moon",
}

// GenerateName generates a random adjective-noun name.
func GenerateName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return adj + "-" + noun
}

// GenerateUniqueTag generates a unique tag that doesn't conflict with existing tunnels.
func GenerateUniqueTag(tunnels []TunnelConfig) string {
	maxAttempts := 100
	existingTags := make(map[string]bool)
	for _, t := range tunnels {
		existingTags[t.Tag] = true
	}

	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if !existingTags[tag] {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}

// ValidateTag validates a tag.
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}

	if len(tag) < 3 {
		return fmt.Errorf("tag must be at least 3 characters")
	}

	if len(tag) > 63 {
		return fmt.Errorf("tag must be at most 63 characters")
	}

	if !tagRegex.MatchString(tag) {
		return fmt.Errorf("tag must start with a lowercase letter and contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}

// NormalizeTag normalizes a tag to lowercase and replaces underscores with hyphens.
func NormalizeTag(tag string) string {
	tag = strings.ToLower(tag)
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	return tag
}
