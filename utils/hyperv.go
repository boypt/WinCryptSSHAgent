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

// ConnectHyperV dials the parent VM's vsock service. The caller supplies ctx
// to control timeout and cancellation; pass context.Background() for a
// blocking dial with no deadline.
func ConnectHyperV(ctx context.Context) (net.Conn, error) {
	addr := winio.HvsockAddr{VMID: winio.HvsockGUIDParent(), ServiceID: HyperVServiceGUID}
	return winio.Dial(ctx, &addr)
}
