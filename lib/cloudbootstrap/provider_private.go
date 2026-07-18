//go:build cloudcontrol

package cloudbootstrap

import (
	privateclient "bilibili-ticket-golang-cloudcontrol/client"

	"bilibili-ticket-golang/lib/cloudcontrol"
)

func New(config cloudcontrol.Config) (cloudcontrol.Controller, error) {
	return privateclient.New(config)
}
