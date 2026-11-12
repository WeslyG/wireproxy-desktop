package main

import "golang.org/x/sys/windows"

func preferredLocales() []string {
	const muiLanguageName = 0x8
	locales, err := windows.GetUserPreferredUILanguages(muiLanguageName)
	if err != nil {
		locales, _ = windows.GetSystemPreferredUILanguages(muiLanguageName)
	}
	return locales
}
