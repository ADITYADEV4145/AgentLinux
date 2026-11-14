//go:build !darwin

// SPDX-License-Identifier: Apache-2.0
package checkpoint

import "os"

func clone(_ *os.Root, _ *os.File, _ string) (bool, error) { return false, nil }
