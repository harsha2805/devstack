//go:build windows

// Package winproc wraps the Windows process APIs devstack needs: hidden and
// detached spawns, Job Objects for whole-tree kills, port owners, and file locks.
package winproc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// HideWindow keeps a console child from opening its own window.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}

// StartDetached starts cmd with no console, outside the caller's console group,
// and (when the parent's job allows it) outside the parent's job, so closing
// the terminal does not take it down. It returns the child's pid.
func StartDetached(cmd *exec.Cmd) (int, error) {
	base := uint32(windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: base | windows.CREATE_BREAKAWAY_FROM_JOB}
	err := cmd.Start()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		// The parent's job forbids breakaway. An exec.Cmd can't be started
		// twice, so retry with a copy.
		retry := exec.Command(cmd.Path, cmd.Args[1:]...)
		retry.Dir, retry.Env = cmd.Dir, cmd.Env
		retry.Stdin, retry.Stdout, retry.Stderr = cmd.Stdin, cmd.Stdout, cmd.Stderr
		retry.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: base}
		cmd = retry
		err = cmd.Start()
	}
	if err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

// Job is a Job Object that kills its processes when terminated or when the
// last handle closes (including when this process dies).
type Job struct{ h windows.Handle }

func NewJob() (*Job, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return &Job{h}, nil
}

// Assign adds a running process (and its future children) to the job.
func (j *Job) Assign(pid int) error {
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess %d: %w", pid, err)
	}
	defer windows.CloseHandle(ph)
	if err := windows.AssignProcessToJobObject(j.h, ph); err != nil {
		return fmt.Errorf("AssignProcessToJobObject %d: %w", pid, err)
	}
	return nil
}

// Terminate kills every process in the job.
func (j *Job) Terminate() error { return windows.TerminateJobObject(j.h, 1) }

func (j *Job) Close() error { return windows.CloseHandle(j.h) }

// Alive reports whether pid is a running process.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied still means the process exists.
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	const stillActive = 259
	return code == stillActive
}

// ProcessName returns the executable base name for pid, or "" if unknown.
func ProcessName(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const tcpTableOwnerPIDListener = 3

// PortOwner returns the pid listening on TCP port on any loopback or wildcard
// address (IPv4 or IPv6), or 0 if the port is free for 127.0.0.1.
//
// A plain bind test is not enough on Windows: binding 127.0.0.1:80 succeeds
// while another process holds 0.0.0.0:80, and then silently steals its
// loopback traffic.
func PortOwner(port int) (int, error) {
	for _, family := range []uint32{windows.AF_INET, windows.AF_INET6} {
		rows, err := listenTable(family)
		if err != nil {
			return 0, err
		}
		for _, r := range rows {
			if r.port == port && r.relevant {
				return r.pid, nil
			}
		}
	}
	return 0, nil
}

type listenRow struct {
	port     int
	pid      int
	relevant bool // wildcard or loopback address
}

func listenTable(family uint32) ([]listenRow, error) {
	size := uint32(0)
	for {
		var buf []byte
		var ptr uintptr
		if size > 0 {
			buf = make([]byte, size)
			ptr = uintptr(unsafe.Pointer(&buf[0]))
		}
		r, _, _ := procGetExtendedTcpTable.Call(ptr, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), tcpTableOwnerPIDListener, 0)
		switch windows.Errno(r) {
		case windows.ERROR_INSUFFICIENT_BUFFER:
			continue
		case 0:
			if buf == nil {
				return nil, nil
			}
			return parseTable(family, buf), nil
		default:
			return nil, fmt.Errorf("GetExtendedTcpTable: %w", windows.Errno(r))
		}
	}
}

func parseTable(family uint32, buf []byte) []listenRow {
	u32 := func(off int) uint32 { return *(*uint32)(unsafe.Pointer(&buf[off])) }
	portOf := func(raw uint32) int { return int(raw&0xff)<<8 | int(raw>>8&0xff) } // network byte order
	n := int(u32(0))
	var rows []listenRow
	if family == windows.AF_INET {
		// MIB_TCPROW_OWNER_PID: state, localAddr, localPort, remoteAddr, remotePort, pid
		const rowSize = 24
		for i := 0; i < n; i++ {
			off := 4 + i*rowSize
			addr := u32(off + 4)
			ip0 := byte(addr) // first octet in memory order
			rows = append(rows, listenRow{
				port:     portOf(u32(off + 8)),
				pid:      int(u32(off + 20)),
				relevant: addr == 0 || ip0 == 127,
			})
		}
		return rows
	}
	// MIB_TCP6ROW_OWNER_PID: localAddr[16], scope, localPort, remoteAddr[16], scope, remotePort, state, pid
	const rowSize = 56
	for i := 0; i < n; i++ {
		off := 4 + i*rowSize
		addr := buf[off : off+16]
		any, loop := true, true
		for k, b := range addr {
			if b != 0 {
				any = false
			}
			if (k < 15 && b != 0) || (k == 15 && b != 1) {
				loop = false
			}
		}
		rows = append(rows, listenRow{
			port:     portOf(u32(off + 20)),
			pid:      int(u32(off + 52)),
			relevant: any || loop,
		})
	}
	return rows
}

// FileVersion returns the major.minor.build.revision version resource of a DLL or exe.
func FileVersion(path string) ([4]int, error) {
	var v [4]int
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil {
		return v, err
	}
	buf := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return v, err
	}
	var info *windows.VS_FIXEDFILEINFO
	var n uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&info), &n); err != nil {
		return v, err
	}
	v[0] = int(info.FileVersionMS >> 16)
	v[1] = int(info.FileVersionMS & 0xffff)
	v[2] = int(info.FileVersionLS >> 16)
	v[3] = int(info.FileVersionLS & 0xffff)
	return v, nil
}

// VCRuntime reports the installed Visual C++ 2015-2022 x64 runtime version,
// which PHP and MySQL builds need. ok is false when it is missing.
func VCRuntime() (version string, ok bool) {
	sys, err := windows.GetSystemDirectory()
	if err != nil {
		return "", false
	}
	v, err := FileVersion(filepath.Join(sys, "vcruntime140.dll"))
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]), true
}

// LongPath expands 8.3 short names (C:\Users\HARSHA~1 -> C:\Users\HarshavardhanC).
// nginx refuses to open files through short-name aliases, so every path
// written into generated config must be long. Returns p unchanged on error.
func LongPath(p string) string {
	in, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return p
		}
		if int(n) <= len(buf) {
			return windows.UTF16ToString(buf[:n])
		}
		buf = make([]uint16, n)
	}
}

// LockFile opens path with no sharing, so a second caller fails until the
// first process exits. Windows releases it automatically on crash.
func LockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
