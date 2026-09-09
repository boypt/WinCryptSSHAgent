module github.com/buptczq/WinCryptSSHAgent

go 1.24.0

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/hattya/go.notify v0.1.0
	golang.org/x/crypto v0.41.0
	golang.org/x/sys v0.41.0
)

replace github.com/hattya/go.notify => github.com/boypt/go.notify v0.1.1
