//go:build windows && amd64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-e7cfc42-ssr-windows-amd64.zip"

//go:embed assets/sing-box-e7cfc42-ssr-windows-amd64.zip
var bundledArchiveData []byte
