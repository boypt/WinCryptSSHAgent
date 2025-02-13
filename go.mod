module github.com/buptczq/WinCryptSSHAgent

go 1.23

toolchain go1.24.0

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/bi-zone/wmi v1.1.4
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/hattya/go.notify v0.0.0-20250130120447-04e15319a783
	golang.org/x/crypto v0.33.0
	golang.org/x/sys v0.30.0
)

require (
	github.com/bi-zone/go-ole v1.2.5 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/scjalliance/comshim v0.0.0-20250111221056-b2ef9d8d7e0f // indirect
)

// replace github.com/hattya/go.notify v0.0.0-20200507123844-18670158b53e => github.com/buptczq/go.notify v0.0.0-20210108030838-37adc71f67d9
// replace github.com/Microsoft/go-winio v0.4.16 => github.com/buptczq/go-winio v0.4.16-1
