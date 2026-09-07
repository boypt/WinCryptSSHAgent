package utils

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	modcomctl32            = syscall.NewLazyDLL("comctl32.dll")
	procTaskDialogIndirect = modcomctl32.NewProc("TaskDialogIndirect")
)

const (
	TDF_ALLOW_DIALOG_CANCELLATION   = 0x0008
	TDF_USE_COMMAND_LINKS           = 0x0010
	TDF_EXPAND_FOOTER_AREA          = 0x0040
	TDF_EXPANDED_BY_DEFAULT         = 0x0080
	TDF_POSITION_RELATIVE_TO_WINDOW = 0x1000
	TDF_SIZE_TO_CONTENT             = 0x01000000

	TDCBF_YES_BUTTON = 0x0002
	TDCBF_NO_BUTTON  = 0x0004

	TD_WARNING_ICON     = ^uintptr(0)
	TD_INFORMATION_ICON = ^uintptr(2)
	TD_SHIELD_ICON      = ^uintptr(3)

	// eCancelledHRESULT is HRESULT_FROM_WIN32(ERROR_CANCELLED), returned by
	// TaskDialogIndirect when the user closes the dialog via Esc / X.
	eCancelledHRESULT = 0x800704C7
)

// taskDialogConfig mirrors TASKDIALOGCONFIG from commctrl.h. The explicit Pad*
// fields keep the layout identical to the C struct on 64-bit Windows, where
// the total size is 176 bytes. CbSize is always set via unsafe.Sizeof so the
// struct stays correct on other architectures as well.
type taskDialogConfig struct {
	CbSize                  uint32
	HwndParent              uintptr
	HInstance               uintptr
	DwFlags                 uint32
	DwCommonButtons         uint32
	PszWindowTitle          *uint16
	MainIcon                uintptr
	PszMainInstruction      *uint16
	PszContent              *uint16
	CButtons                uint32
	Pad1                    uint32
	PButtons                uintptr
	NDefaultButton          int32
	CRadioButtons           uint32
	PRadioButtons           uintptr
	NDefaultRadioButton     int32
	Pad2                    uint32
	PszVerificationText     *uint16
	PszExpandedInformation  *uint16
	PszExpandedControlText  *uint16
	PszCollapsedControlText *uint16
	FooterIcon              uintptr
	PszFooter               *uint16
	PfCallback              uintptr
	LpCallbackData          uintptr
	CxWidth                 uint32
	Pad3                    uint32
}

// utf16PtrOrNil converts s to a UTF-16 pointer, returning nil for empty
// strings so the corresponding dialog section is hidden. A conversion error
// (embedded NUL) is reported as ok == false.
func utf16PtrOrNil(s string) (p *uint16, ok bool) {
	if s == "" {
		return nil, true
	}
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil, false
	}
	return p, true
}

// TaskDialog shows a modern task dialog with Yes/No buttons and returns the
// pressed button (IDYES / IDNO). It returns 0 when TaskDialogIndirect is
// unavailable (pre-Vista comctl32) or the call fails, in which case the
// caller should fall back to MessageBox. Closing the dialog via Esc or the
// X button is mapped to IDNO so it is treated as an explicit deny.
func TaskDialog(title, mainInstruction, content, expandedInfo, footer string) int {
	if err := procTaskDialogIndirect.Find(); err != nil {
		return 0
	}
	pTitle, ok := utf16PtrOrNil(title)
	if !ok {
		return 0
	}
	pMain, ok := utf16PtrOrNil(mainInstruction)
	if !ok {
		return 0
	}
	pContent, ok := utf16PtrOrNil(content)
	if !ok {
		return 0
	}
	pExpanded, ok := utf16PtrOrNil(expandedInfo)
	if !ok {
		return 0
	}
	pFooter, ok := utf16PtrOrNil(footer)
	if !ok {
		return 0
	}
	cfg := taskDialogConfig{
		HwndParent:             0,
		DwFlags:                TDF_ALLOW_DIALOG_CANCELLATION | TDF_POSITION_RELATIVE_TO_WINDOW | TDF_SIZE_TO_CONTENT,
		DwCommonButtons:        TDCBF_YES_BUTTON | TDCBF_NO_BUTTON,
		PszWindowTitle:         pTitle,
		MainIcon:               TD_INFORMATION_ICON,
		PszMainInstruction:     pMain,
		PszContent:             pContent,
		NDefaultButton:         int32(IDNO),
		PszExpandedInformation: pExpanded,
		PszFooter:              pFooter,
	}
	cfg.CbSize = uint32(unsafe.Sizeof(cfg))
	var pnButton int32
	ret, _, _ := procTaskDialogIndirect.Call(
		uintptr(unsafe.Pointer(&cfg)),
		uintptr(unsafe.Pointer(&pnButton)),
		0,
		0)
	// Keep the UTF-16 buffers and the config alive through the call.
	runtime.KeepAlive(pTitle)
	runtime.KeepAlive(pMain)
	runtime.KeepAlive(pContent)
	runtime.KeepAlive(pExpanded)
	runtime.KeepAlive(pFooter)
	runtime.KeepAlive(&cfg)
	if ret == 0 { // S_OK
		return int(pnButton)
	}
	if uint32(ret) == eCancelledHRESULT {
		return IDNO
	}
	return 0
}
