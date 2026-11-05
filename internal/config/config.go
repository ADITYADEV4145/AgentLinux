// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const LimaVersion = "2.2.1"
const ImageURL = "https://cloud.debian.org/images/cloud/trixie/20261001-2618/debian-13-genericcloud-arm64-20261001-2618.qcow2"
const ImageSHA512 = "d8470b8c6c38fead046c794b5800a5a7b96672d5bcf543cc230ceb0c4b8ace05ed341a0c8928045422243fde26b2f1f2f58e99244c65709ffda2e3d4b674dd5a"
const GiB = uint64(1 << 30)

type Spec struct {
	CPUs        int    `json:"cpus"`
	MemoryGiB   int    `json:"memoryGiB"`
	DiskGiB     int    `json:"diskGiB"`
	Network     string `json:"network"`
	Image       string `json:"image"`
	Digest      string `json:"digest"`
	GuestBinary string `json:"guestBinary"`
}

func Default() Spec {
	return Spec{CPUs: 2, MemoryGiB: 2, DiskGiB: 16, Image: ImageURL, Digest: ImageSHA512}
}
func (s Spec) Validate() error {
	if s.CPUs < 1 || s.CPUs > 16 {
		return errors.New("cpus must be 1..16 and no more than host CPUs")
	}
	if s.MemoryGiB < 1 || s.MemoryGiB > 64 {
		return errors.New("memory must be 1..64 GiB and at most half host RAM")
	}
	if s.DiskGiB < 8 || s.DiskGiB > 128 {
		return errors.New("disk must be 8..128 GiB")
	}
	if s.Network != "nat" {
		return errors.New("explicit --network=nat is required (guest can reach host/LAN); offline mode is not validated")
	}
	if !regexp.MustCompile(`^[0-9a-f]{128}$`).MatchString(s.Digest) {
		return errors.New("image digest must be a lowercase SHA-512 hex value")
	}
	if s.Image == ImageURL && s.Digest != ImageSHA512 {
		return errors.New("built-in image requires its pinned SHA-512")
	}
	if s.Image != ImageURL {
		if !filepath.IsAbs(s.Image) || strings.ContainsAny(s.Image, "\n\r\x00") {
			return errors.New("custom image must be an absolute local path with --image-sha512; remote overrides are not supported")
		}
		if e := Regular(s.Image); e != nil {
			return fmt.Errorf("image: %w", e)
		}
	}
	if e := Regular(s.GuestBinary); e != nil {
		return fmt.Errorf("guest binary: %w; run make build", e)
	}
	return nil
}
func Regular(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return errors.New("expected absolute file path")
	}
	i, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() || i.Size() == 0 {
		return errors.New("expected nonempty regular file, not a symlink")
	}
	return nil
}
