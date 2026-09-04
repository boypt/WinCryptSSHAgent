package utils

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	procGetForegroundWindow      = moduser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = moduser32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = moduser32.NewProc("AttachThreadInput")
	procSetForegroundWindow      = moduser32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = moduser32.NewProc("BringWindowToTop")
	procIsWindowVisible          = moduser32.NewProc("IsWindowVisible")
	procEnumThreadWindows        = moduser32.NewProc("EnumThreadWindows")
	procGetClassNameW            = moduser32.NewProc("GetClassNameW")
	procGetCurrentThreadId       = modkernel32.NewProc("GetCurrentThreadId")
)

// dialogClassName is the standard Win32 class of message-box / dialog windows.
const dialogClassName = "#32770"

// currentThreadId returns the Win32 thread id of the OS thread the caller is
// executing on.
func currentThreadId() uintptr {
	tid, _, _ := procGetCurrentThreadId.Call()
	return tid
}

// raiseDialogWhenShown watches (from a helper goroutine) for the modal dialog
// that a subsequent blocking MessageBox* call on the given OS thread will
// create, and forces it into the foreground with keyboard focus.
//
// Because this process runs as a background tray app, Windows' foreground-lock
// restrictions can leave the dialog visible but unfocused (MB_SETFOREGROUND /
// MB_SYSTEMMODAL are no-ops for non-foreground processes on modern Windows).
// The workaround is to temporarily share the input queue of the current
// foreground thread, which lets SetForegroundWindow succeed.
//
// The dialog is identified by its creating thread rather than by caption, so
// concurrent signing requests each raise their own dialog. The watcher is
// one-shot: it exits after raising the dialog or after a 3s deadline if the
// dialog never appears.
func raiseDialogWhenShown(threadID uintptr) {
	go func() {
		var found uintptr
		var classBuf [64]uint16
		// One callback slot per watcher; EnumThreadWindows invokes it
		// synchronously on this goroutine's thread.
		callback := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
			n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&classBuf[0])), uintptr(len(classBuf)))
			if n == 0 || syscall.UTF16ToString(classBuf[:n]) != dialogClassName {
				return 0
			}
			if vis, _, _ := procIsWindowVisible.Call(hwnd); vis != 0 {
				found = hwnd
				return 1 // stop enumeration
			}
			return 0
		})
		findDialog := func() uintptr {
			found = 0
			procEnumThreadWindows.Call(threadID, callback, 0)
			return found
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if hwnd := findDialog(); hwnd != 0 {
				forceForegroundWindow(hwnd)
				// Re-raise shortly after: the message box may still be
				// finishing its own activation and swallow the first raise.
				// Re-enumerate instead of reusing the handle, in case the
				// dialog was dismissed and the HWND reused in the meantime.
				time.Sleep(50 * time.Millisecond)
				if hwnd := findDialog(); hwnd != 0 {
					forceForegroundWindow(hwnd)
				}
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
	}()
}

// forceForegroundWindow raises hwnd above the current foreground window by
// temporarily attaching to the foreground thread's input queue.
func forceForegroundWindow(hwnd uintptr) {
	fg, _, _ := procGetForegroundWindow.Call()
	curTid, _, _ := procGetCurrentThreadId.Call()
	var fgTid uintptr
	if fg != 0 {
		fgTid, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
	}
	attached := fgTid != 0 && fgTid != curTid
	if attached {
		procAttachThreadInput.Call(curTid, fgTid, 1)
	}
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	if attached {
		procAttachThreadInput.Call(curTid, fgTid, 0)
	}
}
