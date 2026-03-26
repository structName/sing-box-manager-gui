//go:build darwin && amd64

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-1.13.3-darwin-amd64.tar.gz"

//go:embed assets/sing-box-1.13.3-darwin-amd64.tar.gz
var bundledArchiveData []byte
