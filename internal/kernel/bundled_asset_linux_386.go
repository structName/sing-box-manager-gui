//go:build linux && 386

package kernel

import _ "embed"

const bundledArchiveName = "sing-box-1.13.3-linux-386.tar.gz"

//go:embed assets/sing-box-1.13.3-linux-386.tar.gz
var bundledArchiveData []byte
