//go:build linux && arm64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-e7cfc42-ssr-linux-arm64.tar.gz"

//go:embed assets/sing-box-e7cfc42-ssr-linux-arm64.tar.gz
var bundledArchiveData []byte
