package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Colors for the verbose output
const (
	redColor   = "\033[31m"
	greenColor = "\033[32m"
	blueColor  = "\033[34m"
	resetColor = "\033[0m"
)

func coloredPrintf(color string, format string, a ...interface{}) {
	fmt.Printf(color+format+resetColor, a...)
}

// The entry point of the program
// - parse command-line flags
// - set up the backup directory,
// - initiate the process of handling symlinks in the specified target directory
func main() {

	noBackup, brokenSymlinks, noRecurse, targetDir := parseFlags()

	processedSymlinks := make(map[string]bool)
	if err := processSymlinks(targetDir, noBackup, noRecurse, *brokenSymlinks, processedSymlinks); err != nil {
		coloredPrintf(redColor, "Error processing symlinks: %v\n", err)
		os.Exit(1)
	}

	// Count the number of processed symlinks
	count := 0
	for _, processed := range processedSymlinks {
		if processed {
			count++
		}
	}

	coloredPrintf(greenColor, "Symlink replacement complete. Processed %d symlinks.\n", count)
}

// Parse command-line flags and return their values
func parseFlags() (noBackup *bool, brokenSymlinks *string, noRecurse *bool, targetDir string) {

	noBackup = flag.Bool("no-backup", false, "Disable backup of symlinks")
	brokenSymlinks = flag.String("broken-symlinks", "keep", "How to handle broken symlinks: keep or delete")
	noRecurse = flag.Bool("no-recurse", false, "Disable recursive traversal of subdirectories")
	flag.Parse()

	// Validate broken-symlinks flag
	if *brokenSymlinks != "keep" && *brokenSymlinks != "delete" {
		fmt.Printf(redColor+"Invalid value for -broken-symlinks: %s. Must be 'keep' or 'delete'\n"+resetColor, *brokenSymlinks)
		os.Exit(1)
	}

	// Check for required non-flag argument (target directory)
	if flag.NArg() != 1 {
		fmt.Println("Usage: " + blueColor + "symlink2file" + resetColor + " [OPTIONS] <directory>")
		os.Exit(1)
	}

	// Convert to absolute path
	var err error
	targetDir = flag.Arg(0)
	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		fmt.Printf(redColor+"Error resolving path: %v\n"+resetColor, err)
		os.Exit(1)
	}

	return noBackup, brokenSymlinks, noRecurse, targetDir
}

// Process the symlinks in the given directory
func processSymlinks(targetDir string, noBackup, noRecurse *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {
	walkFunc := func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %q: %w", path, err)
		}

		// Skip .symlink2file directory and handle no-recurse logic
		if strings.Contains(path, ".symlink2file") || (info.IsDir() && *noRecurse && path != targetDir) {
			return filepath.SkipDir
		}

		// Process only symlinks
		if info.Type()&os.ModeSymlink != 0 {
			return processPath(path, targetDir, noBackup, brokenSymlinks, processedSymlinks)
		}

		return nil
	}

	return filepath.WalkDir(targetDir, walkFunc)
}

// Create a backup of the symlink
// This function also marks the symlink as processed in the processedSymlinks map.
func backupSymlink(path, targetDir string, processedSymlinks map[string]bool) error {

	// Create a .symlink2file directory in the same directory as the symlink
	dir := filepath.Dir(path)
	backupDir := filepath.Join(dir, ".symlink2file")
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		if err := os.Mkdir(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

	linkDest, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("failed to read symlink: %w", err)
	}
	backupPath := filepath.Join(backupDir, filepath.Base(path))
	if err := os.Symlink(linkDest, backupPath); err != nil {
		return fmt.Errorf("failed to create backup symlink: %w", err)
	}
	processedSymlinks[path] = true // Mark the symlink as processed
	return nil
}

// Processes a given path within the filesystem
// If the path is a symlink, it evaluates the symlink, potentially backs it up (based on user flags),
// and replaces it with a copy of the target file.
// For broken symlinks, it either deletes them or keeps them based on the provided option.
// It also handles the logic to avoid re-processing of already processed symlinks
func processPath(path, targetDir string, noBackup *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {

	// Check if the symlink has already been processed
	if processedSymlinks[path] {
		fmt.Println("Symlink already processed, skipping:", path)
		return nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil && !*noBackup && brokenSymlinks == "delete" {
		// Backup broken symlink before deleting
		if backupErr := backupSymlink(path, targetDir, processedSymlinks); backupErr != nil {
			return fmt.Errorf("failed to backup broken symlink %q: %w", path, backupErr)
		}
	}

	if err != nil {
		if brokenSymlinks == "delete" {
			if removeErr := os.Remove(path); removeErr != nil {
				return fmt.Errorf("error removing broken symlink %q: %w", path, removeErr)
			}
			coloredPrintf(redColor, "Removed broken symlink: "+resetColor+"%s\n", path)
		} else {
			coloredPrintf(redColor, "Keeping broken symlink: "+resetColor+"%s\n", path)
		}
		return nil
	}

	if !*noBackup {
		if err := backupSymlink(path, targetDir, processedSymlinks); err != nil {
			return fmt.Errorf("failed to backup symlink %q: %w", path, err)
		}
	}

	// Replace symlink with a copy of the file it points to
	if err := replaceSymlinkWithFile(path, resolvedPath); err != nil {
		return fmt.Errorf("failed to replace symlink %q with its target file %q: %w", path, resolvedPath, err)
	}

	processedSymlinks[path] = true
	return nil
}

// Replace a symlink with a regular file
// It also replicates the original file's metadata (modification times and permissions) to the new file
func replaceSymlinkWithFile(symlinkPath, targetFilePath string) error {
	// Create a temporary file in the same directory
	dir := filepath.Dir(symlinkPath)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup in case of errors
	defer func() {
		tempFile.Close()
		os.Remove(tempPath)
	}()

	// Open the target file for reading
	inputFile, err := os.Open(targetFilePath)
	if err != nil {
		return fmt.Errorf("error opening target file %q: %w", targetFilePath, err)
	}
	defer inputFile.Close()

	// Copy the content to the temporary file
	if _, err = io.Copy(tempFile, inputFile); err != nil {
		return fmt.Errorf("error copying data to temporary file: %w", err)
	}

	// Get the original file's metadata to replicate it
	originalFileInfo, err := os.Stat(targetFilePath)
	if err != nil {
		return fmt.Errorf("error getting file info for %q: %w", targetFilePath, err)
	}

	// Set the file metadata to match the original file
	if err := tempFile.Chmod(originalFileInfo.Mode()); err != nil {
		return fmt.Errorf("error setting file mode: %w", err)
	}

	// Close the temporary file before moving it
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("error closing temporary file: %w", err)
	}

	// Remove the symlink
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("error removing symlink %q: %w", symlinkPath, err)
	}

	// Rename temporary file to final location
	if err := os.Rename(tempPath, symlinkPath); err != nil {
		return fmt.Errorf("error moving temporary file to final location: %w", err)
	}

	// Set the file times after the move
	if err := os.Chtimes(symlinkPath, originalFileInfo.ModTime(), originalFileInfo.ModTime()); err != nil {
		return fmt.Errorf("error setting file times: %w", err)
	}

	return nil
}
