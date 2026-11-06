// Package router decides which model tier should answer a prompt.
package router

import (
	"encoding/json"
	"os"
	"strings"
)

type Config struct {
	MaxSmallChars int                `json:"max_small_chars"`
	HardKeywords  []string           `json:"hard_keywords"`
	Costs         map[string]float64 `json:"costs"`
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// Choose returns "small" or "large". Rules only; swap in a classifier later.
func (c Config) Choose(prompt string) string {
	if len(prompt) > c.MaxSmallChars {
		return "large"
	}
	l := strings.ToLower(prompt)
	for _, k := range c.HardKeywords {
		if strings.Contains(l, k) {
			return "large"
		}
	}
	return "small"
}

func Other(m string) string {
	if m == "small" {
		return "large"
	}
	return "small"
}
