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
		coloredPrintf(redColor, "Error processing symlinks:", err)
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

	// Check for required non-flag argument (target directory)
	if flag.NArg() != 1 {
		fmt.Println("Usage: " + blueColor + "symlink2file" + resetColor + " [OPTIONS] <directory>")
		os.Exit(1)
	}
	targetDir = flag.Arg(0)

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
	// Open the target file for reading
	inputFile, err := os.Open(targetFilePath)
	if err != nil {
		return fmt.Errorf("error opening target file %q: %w", targetFilePath, err)
	}
	defer inputFile.Close()

	// Remove the symlink
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("error removing symlink %q: %w", symlinkPath, err)
	}

	// Create a new file at the original symlink path
	outputFile, err := os.Create(symlinkPath)
	if err != nil {
		return fmt.Errorf("error creating file at %q: %w", symlinkPath, err)
	}
	defer outputFile.Close()

	// Copy the content to the new file
	if _, err = io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("error copying data to new file at %q: %w", symlinkPath, err)
	}

	// Get the original file's metadata to replicate it
	originalFileInfo, err := os.Stat(targetFilePath)
	if err != nil {
		return fmt.Errorf("error getting file info for %q: %w", targetFilePath, err)
	}

	// Set the file metadata to match the original file
	if err := os.Chtimes(symlinkPath, originalFileInfo.ModTime(), originalFileInfo.ModTime()); err != nil {
		return fmt.Errorf("error setting file times for %q: %w", symlinkPath, err)
	}

	if err := os.Chmod(symlinkPath, originalFileInfo.Mode()); err != nil {
		return fmt.Errorf("error setting file mode for %q: %w", symlinkPath, err)
	}

	return nil
}
