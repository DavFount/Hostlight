package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var icmpDLL = windows.NewLazySystemDLL("iphlpapi.dll")
var createEcho = icmpDLL.NewProc("IcmpCreateFile")
var createEcho6 = icmpDLL.NewProc("Icmp6CreateFile")
var closeEcho = icmpDLL.NewProc("IcmpCloseHandle")
var sendEcho = icmpDLL.NewProc("IcmpSendEcho")
var sendEcho6 = icmpDLL.NewProc("Icmp6SendEcho2")

// Uses Windows IP Helper rather than raw sockets or localized command output.
// Synchronous native calls finish within the timeout; cancellation suppresses
// their results. The scheduler keeps the concurrency slot until the call returns.
func nativePing(ctx context.Context, address net.IPAddr, timeout time.Duration) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if timeout <= 0 {
		return 0, context.DeadlineExceeded
	}
	v4 := address.IP.To4()
	create := createEcho
	if v4 == nil {
		create = createEcho6
	}
	handle, _, err := create.Call()
	if handle == ^uintptr(0) || handle == 0 {
		return 0, &probeSetupError{fmt.Sprintf("Windows could not open a ping handle: %v", err)}
	}
	defer closeEcho.Call(handle)
	payload := []byte("Hostlight network echo check     ")
	reply := make([]byte, 512)
	milliseconds := uintptr((timeout + time.Millisecond - 1) / time.Millisecond)
	var replies uintptr
	statusOffset := 4
	if v4 != nil {
		replies, _, err = sendEcho.Call(handle, uintptr(binary.LittleEndian.Uint32(v4)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), milliseconds)
	} else {
		source := windows.RawSockaddrInet6{Family: windows.AF_INET6}
		dest := windows.RawSockaddrInet6{Family: windows.AF_INET6}
		copy(dest.Addr[:], address.IP.To16())
		if address.Zone != "" {
			scope, e := strconv.ParseUint(address.Zone, 10, 32)
			if e != nil {
				iface, e := net.InterfaceByName(address.Zone)
				if e != nil {
					return 0, &probeSetupError{"IPv6 interface was not found"}
				}
				scope = uint64(iface.Index)
			}
			dest.Scope_id = uint32(scope)
		}
		replies, _, err = sendEcho6.Call(handle, 0, 0, 0, uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&dest)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), milliseconds)
		runtime.KeepAlive(source)
		runtime.KeepAlive(dest)
		// IPV6_ADDRESS_EX occupies 28 bytes with the Windows SDK's default alignment.
		statusOffset = 28
	}
	runtime.KeepAlive(payload)
	runtime.KeepAlive(reply)
	if e := ctx.Err(); e != nil {
		return 0, e
	}
	if replies == 0 {
		if errno, ok := err.(windows.Errno); ok {
			if uint32(errno) >= 11000 && uint32(errno) <= 11050 {
				return 0, pingStatusError(uint32(errno))
			}
		}
		return 0, &probeSetupError{fmt.Sprintf("Windows ping failed: %v", err)}
	}
	status := binary.LittleEndian.Uint32(reply[statusOffset:])
	if e := pingStatusError(status); e != nil {
		return 0, e
	}
	return float64(binary.LittleEndian.Uint32(reply[statusOffset+4:])), nil
}
