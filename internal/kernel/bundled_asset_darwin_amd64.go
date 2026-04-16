//go:build darwin && amd64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-e7cfc42-ssr-darwin-amd64.tar.gz"

//go:embed assets/sing-box-e7cfc42-ssr-darwin-amd64.tar.gz
var bundledArchiveData []byte
