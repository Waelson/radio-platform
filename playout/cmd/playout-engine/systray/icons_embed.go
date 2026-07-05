//go:build !cli

package apptray

import _ "embed"

//go:embed icon-success.png
var iconSuccess []byte

//go:embed icon-failure.png
var iconFailure []byte
