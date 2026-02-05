package openapi

import "errors"

// This file will contain shared types and interfaces for OpenAPI translation.
// Phase 1: Stub implementation

var (
	// ErrNotImplemented is returned when a feature is not yet implemented.
	ErrNotImplemented = errors.New("not implemented yet")
)

// TODO: Add shared types for OpenAPI translation in Phase 2
// - TypeMapping represents the mapping between OpenAPI and Pulse types
// - TranslationContext holds state during the translation process
// - Warnings collects warnings during translation
