package router

import (
	"strings"
	"testing"
)

func TestChoose(t *testing.T) {
	c := Config{MaxSmallChars: 50, HardKeywords: []string{"prove"}}
	cases := []struct{ in, want string }{
		{"capital of France?", "small"},
		{"Prove that sqrt(2) is irrational", "large"},
		{strings.Repeat("x", 80), "large"},
	}
	for _, tc := range cases {
		if got := c.Choose(tc.in); got != tc.want {
			t.Errorf("Choose(%q)=%s want %s", tc.in, got, tc.want)
		}
	}
}
