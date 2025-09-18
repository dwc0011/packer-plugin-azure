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

	"github.com/hashicorp/packer-plugin-azure/builder/azure/common/log"
)

func (da diskAttacher) WaitForDevice(ctx context.Context, lun int64) (device string, err error) {
	log.Printf("Wait for Linux Device lun %d to show up.", lun)
	scsiPath := fmt.Sprintf("/dev/disk/azure/scsi1/lun%d", lun)
	nvmePath := fmt.Sprintf("/dev/disk/azure/nvme/lun%d", lun)

	for {

		log.Printf("Checking for nvme path %s", nvmePath)
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
		log.Printf("Checking for scsiPath path %s", scsiPath)
		if resolved, err := filepath.EvalSymlinks(scsiPath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				return resolved, nil
			}
		} else if err != os.ErrNotExist {
			if pe, ok := err.(*os.PathError); ok && pe.Err != syscall.ENOENT {
				return "", err
			}
		}

		log.Printf("Not attached yet, checking for scsiPath path %s", scsiPath)
		select {
		case <-time.After(100 * time.Millisecond):
			// continue
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
