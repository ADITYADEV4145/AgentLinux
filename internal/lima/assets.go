// SPDX-License-Identifier: Apache-2.0
package lima

import _ "embed"

//go:embed assets/desktop.sh
var desktopScript string

//go:embed assets/desktop.service
var desktopUnit string

//go:embed assets/software.sh
var softwareScript string
