// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package chroot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func (da diskAttacher) WaitForDevice(ctx context.Context, lun int64) (device string, err error) {
	scsiPath := fmt.Sprintf("/dev/disk/azure/scsi1/lun%d", lun)
	nvmePath := fmt.Sprintf("/dev/disk/azure/nvme/lun%d", lun)

	for {

		if resolved, err := filepath.EvalSymlinks(nvmePath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				return resolved, nil
			}
		} else if err != os.ErrNotExist {
			if pe, ok := err.(*os.PathError); ok && pe.Err != syscall.ENOENT {
				return "", err
			}
		}

		// Check SCSI
		if resolved, err := filepath.EvalSymlinks(scsiPath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				return resolved, nil
			}
		} else if err != os.ErrNotExist {
			if pe, ok := err.(*os.PathError); ok && pe.Err != syscall.ENOENT {
				return "", err
			}
		}

		select {
		case <-time.After(100 * time.Millisecond):
			// continue
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
