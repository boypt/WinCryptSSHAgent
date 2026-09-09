package utils

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"os"
	"syscall"
)

func CheckSCardSvrStatus() (bool, error) {
	mgr, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false, err
	}
	defer windows.CloseServiceHandle(mgr)
	namePtr, err := syscall.UTF16PtrFromString("SCardSvr")
	if err != nil {
		return false, err
	}
	svc, err := windows.OpenService(mgr, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false, err
	}
	defer windows.CloseServiceHandle(svc)
	// SERVICE_STATUS_PROCESS.CurrentState sits at offset 4.
	buf := make([]byte, 8)
	var needed uint32
	if err := windows.QueryServiceStatusEx(svc, windows.SC_STATUS_PROCESS_INFO, &buf[0], uint32(len(buf)), &needed); err != nil {
		return false, err
	}
	return binary.LittleEndian.Uint32(buf[4:8]) == windows.SERVICE_RUNNING, nil
}

func StartSCardSvr() error {
	cwd, _ := os.Getwd()
	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString("net")
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString("start scardsvr")

	return windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, 0)
}
