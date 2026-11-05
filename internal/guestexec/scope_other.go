// SPDX-License-Identifier: Apache-2.0
//go:build !linux

package guestexec

import "errors"

func newScope() (scope, error) { return nil, errors.New("command execution is Linux guest only") }
