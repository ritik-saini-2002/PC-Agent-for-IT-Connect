// Package embed holds files that are baked into the binary via go:embed.
package embed

import _ "embed"

//go:embed viewer.html
var ViewerHTML []byte

//go:embed admincontrol.html
var AdminControlHTML []byte
