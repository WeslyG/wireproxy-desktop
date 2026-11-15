//go:build !windows

package desktop

type windowPlacement struct{}

func (*windowPlacement) attach(any, string) {}
func (*windowPlacement) save(string)        {}
