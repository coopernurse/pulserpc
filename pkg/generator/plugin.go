package generator

import (
	"flag"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// Plugin defines the interface that all code generation plugins must implement
type Plugin interface {
	// Name returns the unique identifier for this plugin (e.g., "python-client-server")
	Name() string

	// RegisterFlags is called to allow the plugin to register its own CLI flags.
	// This is called before flag.Parse() so the plugin can add its flags to the
	// global FlagSet or create its own FlagSet.
	RegisterFlags(fs *flag.FlagSet)

	// Generate is called with the parsed IDL and the FlagSet to generate code output.
	// The plugin can access any flags it registered via RegisterFlags by querying
	// the provided FlagSet.
	Generate(idl *parser.IDL, fs *flag.FlagSet) error
}
