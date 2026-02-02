package generator

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// PrintFileCreated prints the relative path to a file if silent mode is not enabled
func PrintFileCreated(fullPath string, fs *flag.FlagSet) {
	silentFlag := fs.Lookup("silent")
	if silentFlag != nil && silentFlag.Value.String() == "true" {
		return
	}

	// Get relative path from current working directory
	wd, err := os.Getwd()
	relPath, relErr := filepath.Rel(wd, fullPath)
	if err != nil || relErr != nil || relPath == fullPath {
		// Fallback to full path if relative calculation fails
		fmt.Println(fullPath)
	} else {
		fmt.Println(relPath)
	}
}
