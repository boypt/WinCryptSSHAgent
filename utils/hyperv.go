package utils

import (
	"net"
	"context"

	"github.com/Microsoft/go-winio"
)

const (
	servicePort          = 0x22223333
	HyperVServiceRegPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
)

var HyperVServiceGUID = winio.VsockServiceID(servicePort)

func ConnectHyperV() (net.Conn, error) {
	addr := winio.HvsockAddr{VMID: winio.HvsockGUIDParent(), ServiceID: HyperVServiceGUID}

	// would it better to pass context from upper action ?
	conn, err := winio.Dial(context.Background(), &addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
