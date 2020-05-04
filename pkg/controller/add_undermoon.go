package controller

import (
	"github.com/doyoubi/undermoon-operator/pkg/controller/undermoon"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, undermoon.Add)
}
