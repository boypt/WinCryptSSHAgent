package utils

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

const (
	servicePort          = 0x22223333
	HyperVServiceRegPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
)

var HyperVServiceGUID = winio.VsockServiceID(servicePort)

// ConnectHyperV dials the Hyper-V host's vsock agent service (guest→host).
// Succeeds when this process runs inside a guest VM (e.g. a Windows VM under
// Hyper-V), forwarding SSH agent requests to the host's WinCryptSSHAgent.
// Fails on the physical host (HvsockGUIDParent is not supported there), in
// which case the caller falls back to local CAPI signing.
//
// The same vsock channel is also used by WSL2/Linux guests via socat
// (SOCKET-CONNECT:40:0:<host-vmid>x02000000x00000000) — see app/vsock.go.
//
// The caller supplies ctx to control timeout and cancellation; pass
// context.Background() for a blocking dial with no deadline.
func ConnectHyperV(ctx context.Context) (net.Conn, error) {
	addr := winio.HvsockAddr{VMID: winio.HvsockGUIDParent(), ServiceID: HyperVServiceGUID}
	return winio.Dial(ctx, &addr)
}
