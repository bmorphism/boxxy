//go:build darwin

// Package vz provides Joker bindings for Apple Virtualization.framework
// This file is the Go backing implementation for std/vz.joke
package vz

import (
	_ "github.com/bmorphism/boxxy/internal/vm"
)

// Note: The actual Joker namespace registration is done in internal/vm/vm.go
// via vm.RegisterNamespace(). This package exists for organizational purposes
// and could be extended with additional vz-specific utilities.

// This file intentionally left minimal - the namespace bindings are
// registered directly in internal/vm to avoid circular imports and
// keep the VM abstraction clean.
