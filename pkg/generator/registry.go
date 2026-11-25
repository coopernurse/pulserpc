package generator

import (
	"fmt"
)

var (
	// plugins is the global registry of all registered plugins
	plugins = make(map[string]Plugin)
)

// Register registers a plugin with the registry.
// If a plugin with the same name is already registered, Register panics.
func Register(plugin Plugin) {
	name := plugin.Name()
	if _, exists := plugins[name]; exists {
		panic(fmt.Sprintf("plugin %q is already registered", name))
	}
	plugins[name] = plugin
}

// Get retrieves a plugin by name from the registry.
// Returns the plugin and true if found, nil and false otherwise.
func Get(name string) (Plugin, bool) {
	plugin, ok := plugins[name]
	return plugin, ok
}

// List returns a slice of all registered plugin names.
func List() []string {
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	return names
}
