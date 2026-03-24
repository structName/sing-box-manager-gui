//go:build windows && amd64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-1.13.3-windows-amd64.zip"

//go:embed assets/sing-box-1.13.3-windows-amd64.zip
var bundledArchiveData []byte
