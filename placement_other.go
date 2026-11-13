//go:build !windows

package main

type windowPlacement struct{}

func (*windowPlacement) attach(any, string) {}
func (*windowPlacement) save(string)        {}
