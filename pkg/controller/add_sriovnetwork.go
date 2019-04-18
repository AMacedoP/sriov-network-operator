package controller

import (
	"github.com/pliurh/sriov-network-operator/pkg/controller/sriovnetwork"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, sriovnetwork.Add)
}
