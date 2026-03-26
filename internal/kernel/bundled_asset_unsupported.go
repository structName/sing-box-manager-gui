//go:build !(darwin && amd64) && !(darwin && arm64) && !(linux && 386) && !(linux && amd64) && !(linux && arm64) && !(windows && amd64)

package kernel

const bundledArchiveName = ""

var bundledArchiveData []byte
