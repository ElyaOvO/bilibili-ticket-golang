//go:build !cloudcontrol && !production

package cloudbootstrap

import (
	"fmt"

	"bilibili-ticket-golang/lib/cloudcontrol"
)

// New keeps the public source tree buildable without containing the private
// implementation. Release builds must select provider_private.go explicitly.
func New(cloudcontrol.Config) (cloudcontrol.Controller, error) {
	return nil, fmt.Errorf("private cloud-control implementation is not linked; rebuild with -tags cloudcontrol")
}
