package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// The entry point of the program
// - parse command-line flags
// - set up the backup directory,
// - initiate the process of handling symlinks in the specified target directory
func main() {

	noBackup, brokenSymlinks, noRecurse, targetDir := parseFlags()
	backupDir, err := setupBackupDir(targetDir, noBackup) // Handle both returned values
	if err != nil {
		fmt.Printf("Error setting up backup directory: %v\n", err)
		os.Exit(1)
	}

	processedSymlinks := make(map[string]bool)
	if err := processSymlinks(targetDir, backupDir, noBackup, noRecurse, *brokenSymlinks, processedSymlinks); err != nil {
		fmt.Println("Error processing symlinks:", err)
		os.Exit(1)
	}

	// Count the number of processed symlinks
	count := 0
	for _, processed := range processedSymlinks {
		if processed {
			count++
		}
	}

	fmt.Printf("Symlink replacement complete. Processed %d symlinks.\n", count)
}

// Parse command-line flags and return their values
func parseFlags() (noBackup *bool, brokenSymlinks *string, noRecurse *bool, targetDir string) {

	noBackup = flag.Bool("no-backup", false, "Disable backup of symlinks")
	brokenSymlinks = flag.String("broken-symlinks", "keep", "How to handle broken symlinks: keep or delete")
	noRecurse = flag.Bool("no-recurse", false, "Disable recursive traversal of subdirectories")
	flag.Parse()

	// Check for required non-flag argument (target directory)
	if flag.NArg() != 1 {
		fmt.Println("Usage: symlink2file [OPTIONS] <directory>")
		os.Exit(1)
	}
	targetDir = flag.Arg(0)

	return noBackup, brokenSymlinks, noRecurse, targetDir
}

// Create the backup directory if needed
func setupBackupDir(targetDir string, noBackup *bool) (backupDir string, err error) {
	// Backup directory is only needed if backups are enabled
	if !*noBackup {
		backupDir = filepath.Join(targetDir, "symlink_backup_"+time.Now().Format("060102150405"))

		// Create the backup directory with appropriate permissions
		if err = os.MkdirAll(backupDir, 0755); err != nil {
			return "", fmt.Errorf("error creating backup directory: %w", err)
		}
	}

	// Return the backup directory path (empty if no backup)
	return backupDir, nil
}

// Process the symlinks in the given directory
func processSymlinks(targetDir, backupDir string, noBackup, noRecurse *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {

	walkFunc := func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %q: %w", path, err)
		}

		// Skip backup directory and handle no-recurse logic
		if path == backupDir || (info.IsDir() && *noRecurse && path != targetDir) {
			return filepath.SkipDir
		}

		// Process only symlinks
		if info.Type()&os.ModeSymlink != 0 {
			return processPath(path, backupDir, noBackup, brokenSymlinks, processedSymlinks)
		}

		return nil
	}

	return filepath.WalkDir(targetDir, walkFunc)
}

// Processes a given path within the filesystem
// If the path is a symlink, it evaluates the symlink, potentially backs it up (based on user flags),
// and replaces it with a copy of the target file.
// For broken symlinks, it either deletes them or keeps them based on the provided option.
// It also handles the logic to avoid re-processing of already processed symlinks
func processPath(path, backupDir string, noBackup *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {

	// Check if the symlink has already been processed
	if processedSymlinks[path] {
		fmt.Println("Symlink already processed, skipping:", path)
		return nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if brokenSymlinks == "delete" {
			if removeErr := os.Remove(path); removeErr != nil {
				return fmt.Errorf("error removing broken symlink %q: %w", path, removeErr)
			}
			fmt.Println("Removed broken symlink:", path)
		} else {
			fmt.Println("Keeping broken symlink:", path)
		}
		return nil
	}

	if !*noBackup {
		if err := backupSymlink(path, backupDir, processedSymlinks); err != nil {
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

// Create a backup of the symlink
// This function also marks the symlink as processed in the processedSymlinks map.
func backupSymlink(path, backupDir string, processedSymlinks map[string]bool) error {
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
