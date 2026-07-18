package priority

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

const (
	Idle        = 64
	BelowNormal = 16384
	Normal      = 32
	AboveNormal = 32768
	High        = 128
	RealTime    = 256
)

func PriorityString(priority int) string {
	rc := "???????"
	switch priority {
	case Idle:
		rc = "Idle"
	case BelowNormal:
		rc = "BelowNormal"
	case Normal:
		rc = "Normal"
	case AboveNormal:
		rc = "AboveNormal"
	case High:
		rc = "High"
	case RealTime:
		rc = "RealTime"
	}
	return rc
}

const processAllAccess = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0xffff

func GetPriority() (priority int, err error) {
	handle, err := windows.OpenProcess(processAllAccess, false, uint32(os.Getpid()))
	if err != nil {
		return 0, fmt.Errorf("failed to open process: %w", err)
	}
	defer windows.CloseHandle(handle)
	rawPriority, err := windows.GetPriorityClass(handle)
	if err != nil {
		return 0, fmt.Errorf("failed to get priority class: %w", err)
	}
	priority = int(rawPriority)
	return
}

func SetPriority(priority int) error {
	handle, err := windows.OpenProcess(processAllAccess, false, uint32(os.Getpid()))
	if err != nil {
		return fmt.Errorf("failed to open process: %w", err)
	}
	defer windows.CloseHandle(handle)
	if err = windows.SetPriorityClass(handle, uint32(priority)); err != nil {
		return fmt.Errorf("failed to set priority class: %w", err)
	}
	return nil
}
