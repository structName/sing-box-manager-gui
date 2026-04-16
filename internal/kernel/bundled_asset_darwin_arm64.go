//go:build darwin && arm64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-e7cfc42-ssr-darwin-arm64.tar.gz"

//go:embed assets/sing-box-e7cfc42-ssr-darwin-arm64.tar.gz
var bundledArchiveData []byte
