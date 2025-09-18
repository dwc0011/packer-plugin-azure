// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package chroot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-azure/builder/azure/common/log"
)

// checkAzurePath is a helper to check a given path and return the real device
func checkAzurePath(path string) (string, bool, error) {
	if info, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("stat %s: %w", path, err)
		}
		return "", false, nil
	} else if info != nil {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", false, fmt.Errorf("eval symlink %s: %w", path, err)
		}
		return realPath, true, nil
	}
	return "", false, nil
}

// GetDeviceByLUN tries all known Azure methods to find a device for a given LUN.
// Returns (device path, true) if found, ("" , false) if not yet attached.
// Returns an error immediately if a real error occurs.
func GetDeviceByLUN(lun int64) (string, bool, error) {
	// 1. Legacy SCSI path /dev/disk/azure/scsi1/lunX
	log.Printf("Check by legacy scsi path /dev/disk/azure/scsi1/lun%d", lun)
	if dev, found, err := checkAzurePath(fmt.Sprintf("/dev/disk/azure/scsi1/lun%d", lun)); err != nil || found {
		return dev, found, err
	}

	// 2. Legacy NVMe/SCSI /dev/disk/azure/lunX
	log.Printf("Check by legacy nvme/scsi path /dev/disk/azure/lun%d", lun)
	if dev, found, err := checkAzurePath(fmt.Sprintf("/dev/disk/azure/lun%d", lun)); err != nil || found {
		return dev, found, err
	}

	// 3. NVMe via serial in /sys/class/nvme
	log.Printf("Check by NVMe via serial in /sys/class/nvme for lun=%d", lun)
	nvmeGlob := "/sys/class/nvme/nvme*/nvme*n1/device/serial"
	nvmeSerials, err := filepath.Glob(nvmeGlob)
	if err != nil {
		return "", false, fmt.Errorf("glob %s: %w", nvmeGlob, err)
	}
	for _, serialPath := range nvmeSerials {
		data, err := os.ReadFile(serialPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", false, fmt.Errorf("read file %s: %w", serialPath, err)
			}
			continue
		}
		serial := strings.TrimSpace(string(data))
		if strings.HasSuffix(serial, fmt.Sprintf("%d", lun+1)) { // adjust if needed
			device := filepath.Base(filepath.Dir(serialPath)) // nvme0n1
			devicePath := fmt.Sprintf("/dev/%s", device)
			log.Printf("Device:%s", devicePath)
			return devicePath, true, nil
		}
	}

	// 4. SCSI via /dev/disk/by-path/
	log.Printf("Check by SCSI via /dev/disk/by-path/ for lun=%d", lun)
	scsiPath := "/dev/disk/by-path"
	entries, err := os.ReadDir(scsiPath)
	if err != nil && !os.IsNotExist(err) {
		return "", false, fmt.Errorf("read dir %s: %w", scsiPath, err)
	} else if err == nil {
		lunStr := fmt.Sprintf("lun%d", lun)
		for _, entry := range entries {
			if strings.Contains(entry.Name(), lunStr) {
				symlink := filepath.Join(scsiPath, entry.Name())
				realPath, err := filepath.EvalSymlinks(symlink)
				if err != nil {
					return "", false, fmt.Errorf("eval symlink %s: %w", symlink, err)
				}
				return realPath, true, nil
			}
		}
	}

	// Not found yet
	return "", false, nil
}

func (da diskAttacher) WaitForDevice(ctx context.Context, lun int64) (device string, err error) {
	log.Printf("Wait for Linux Device lun %d to show up.", lun)

	for {
		device, found, err := GetDeviceByLUN(lun)
		if err != nil {
			return "", err // real error, return immediately
		}
		if found {
			return device, nil
		}

		log.Printf("Not attached yet, waiting")
		select {
		case <-time.After(100 * time.Millisecond):
			// continue
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
