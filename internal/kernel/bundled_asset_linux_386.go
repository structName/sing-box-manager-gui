//go:build linux && 386

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-e7cfc42-ssr-linux-386.tar.gz"

//go:embed assets/sing-box-e7cfc42-ssr-linux-386.tar.gz
var bundledArchiveData []byte
