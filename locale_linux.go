package main

import (
	"os"
	"strings"
)

func preferredLocales() []string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if locale := strings.TrimSpace(os.Getenv(name)); locale != "" {
			return []string{locale}
		}
	}
	return strings.Split(os.Getenv("LANGUAGE"), ":")
}
