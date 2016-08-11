// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build !gccgo

package vsphere

import (
	"github.com/juju/utils/arch"

	"github.com/juju/juju/constraints"
)

// PrecheckInstance verifies that the provided series and constraints
// are valid for use in creating an instance in this environment.
func (env *environ) PrecheckInstance(series string, cons constraints.Value, placement string) error {
	if placement != "" {
		if _, err := env.parsePlacement(placement); err != nil {
			return err
		}
	}
	return nil
}

var unsupportedConstraints = []string{
	constraints.Tags,
	constraints.VirtType,
}

// This is provided to avoid double hard-coding of provider specific architecture for
// use in constraints validator and metadata lookup params (used to validate images).
var providerSupportedArchitectures = []string{
	arch.AMD64,
	arch.ARM,
	arch.ARM64,
	arch.S390X,
}

// ConstraintsValidator returns a Validator value which is used to
// validate and merge constraints.
func (env *environ) ConstraintsValidator() (constraints.Validator, error) {
	validator := constraints.NewValidator()

	// unsupported

	validator.RegisterUnsupported(unsupportedConstraints)

	// vocab

	validator.RegisterVocabulary(constraints.Arch, providerSupportedArchitectures)

	return validator, nil
}

// SupportNetworks returns whether the environment has support to
// specify networks for applications and machines.
func (env *environ) SupportNetworks() bool {
	return false
}
