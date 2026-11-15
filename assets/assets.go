package assets

import _ "embed"

//go:embed icon.png
var ApplicationIcon []byte

//go:embed tray-idle.png
var TrayIdlePNG []byte

//go:embed tray-template.png
var TrayTemplate []byte

//go:embed favicon.ico
var ApplicationICO []byte

//go:embed tray-idle.ico
var TrayIdleICO []byte
