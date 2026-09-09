package utils

import (
	"encoding/binary"
	"reflect"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const afHvSock = 34      // AF_HYPERV
const sHvProtocolRaw = 1 // HV_PROTOCOL_RAW

var (
	modntdll                      = syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationProcess = modntdll.NewProc("NtQueryInformationProcess")
)

func CheckHVService() bool {
	gcs, err := registry.OpenKey(registry.LOCAL_MACHINE, HyperVServiceRegPath, registry.READ)
	if err != nil {
		return false
	}
	defer gcs.Close()

	agentSrv, err := registry.OpenKey(gcs, HyperVServiceGUID.String(), registry.READ)
	if err != nil {
		return false
	}
	agentSrv.Close()
	return true
}

// snapshotProcesses lists pid → exe name via Toolhelp.
func snapshotProcesses() (map[uint32]string, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	out := make(map[uint32]string)
	var pe windows.ProcessEntry32
	pe.Size = uint32(reflect.TypeOf(pe).Size())
	err = windows.Process32First(snap, &pe)
	for err == nil {
		out[pe.ProcessID] = windows.UTF16ToString(pe.ExeFile[:])
		err = windows.Process32Next(snap, &pe)
	}
	return out, nil
}

func readProcessMemory(h windows.Handle, addr uintptr, size int) ([]byte, error) {
	buf := make([]byte, size)
	var n uintptr
	if err := windows.ReadProcessMemory(h, addr, &buf[0], uintptr(size), &n); err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// decodeUint decodes a pointer-sized little-endian uint.
func decodeUint(b []byte) uintptr {
	if len(b) >= 8 {
		return uintptr(binary.LittleEndian.Uint64(b))
	}
	if len(b) >= 4 {
		return uintptr(binary.LittleEndian.Uint32(b))
	}
	return 0
}

func structFieldOffset(strct interface{}, field string) uintptr {
	f, ok := reflect.TypeOf(strct).FieldByName(field)
	if !ok {
		return 0
	}
	return f.Offset
}

// processCommandLine reads PEB → RTL_USER_PROCESS_PARAMETERS.CommandLine of
// pid. Same-user handle, no elevation needed. Empty on any failure.
func processCommandLine(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	ptrSize := int(reflect.TypeOf(uintptr(0)).Size())
	pbiSize := int(reflect.TypeOf(windows.PROCESS_BASIC_INFORMATION{}).Size())
	pbiOff := structFieldOffset(windows.PROCESS_BASIC_INFORMATION{}, "PebBaseAddress")
	pbi := make([]byte, pbiSize)
	var retLen uint32
	st, _, _ := procNtQueryInformationProcess.Call(
		uintptr(h),
		uintptr(windows.ProcessBasicInformation),
		uptr(&pbi[0]),
		uintptr(len(pbi)),
		uptr(&retLen),
	)
	if st != 0 {
		return ""
	}
	pebOff := structFieldOffset(windows.PEB{}, "ProcessParameters")
	peb, err := readProcessMemory(h, decodeUint(pbi[pbiOff:pbiOff+uintptr(ptrSize)]), int(pebOff)+ptrSize)
	if err != nil {
		return ""
	}
	params := decodeUint(peb[pebOff:])
	clOff := structFieldOffset(windows.RTL_USER_PROCESS_PARAMETERS{}, "CommandLine")
	clSize := int(reflect.TypeOf(windows.NTUnicodeString{}).Size())
	bufOff := structFieldOffset(windows.NTUnicodeString{}, "Buffer")
	cl, err := readProcessMemory(h, params, int(clOff)+clSize)
	if err != nil {
		return ""
	}
	cl = cl[clOff:]
	length := int(binary.LittleEndian.Uint16(cl[0:2]))
	strBuf, err := readProcessMemory(h, decodeUint(cl[bufOff:]), length)
	if err != nil {
		return ""
	}
	u16 := make([]uint16, 0, len(strBuf)/2)
	for i := 0; i+1 < len(strBuf); i += 2 {
		if c := binary.LittleEndian.Uint16(strBuf[i:]); c != 0 {
			u16 = append(u16, c)
		} else {
			break
		}
	}
	return string(utf16.Decode(u16))
}

func GetVMIDs() []string {
	procs, err := snapshotProcesses()
	if err != nil {
		return nil
	}

	guids := make(map[string]interface{})

	for pid, name := range procs {
		if !strings.EqualFold(name, "wslhost.exe") {
			continue
		}
		cmd := processCommandLine(pid)
		if cmd == "" {
			continue
		}
		args := strings.Split(cmd, " ")
		for i := len(args) - 1; i >= 0; i-- {
			if strings.Contains(args[i], "{") {
				guids[args[i]] = nil
				break
			}
		}
	}

	results := make([]string, 0)
	for k := range guids {
		results = append(results, k[1:len(k)-1])
	}
	return results
}

func CheckHvSocket() bool {
	fd, err := syscall.Socket(afHvSock, syscall.SOCK_STREAM, sHvProtocolRaw)
	if err != nil {
		println(err.Error())
		return false
	}
	syscall.Close(fd)
	return true
}
