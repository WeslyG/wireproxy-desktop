package main

import (
	"os"
	"strings"
)

func preferredLocales() []string {
	if locale := os.Getenv("LC_ALL"); locale != "" {
		return []string{locale}
	}
	return append(strings.Split(os.Getenv("LANGUAGE"), ":"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG"))
}
