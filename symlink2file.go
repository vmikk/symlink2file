package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// Colors for the verbose output
const (
	redColor   = "\033[31m"       // Red for errors
	greenColor = "\033[38;5;150m" // Pastel green
	blueColor  = "\033[38;5;110m" // Pastel blue
	greyColor  = "\033[38;5;246m" // Soft gray for commands
	resetColor = "\033[0m"        // Reset to default color
)

const (
	version = "1.2.0" // Program version
)

// Global flag variables
var (
	noBackup       bool
	brokenSymlinks string
	noRecurse      bool
)

// Check if the output is a terminal
func isTerminal(f *os.File) bool {
	if stat, err := f.Stat(); err == nil {
		mode := stat.Mode()
		return (mode & os.ModeCharDevice) == os.ModeCharDevice
	}
	return false
}

// Print with color (if the output is a terminal)
func coloredPrintf(color string, format string, a ...interface{}) {
	// Only use colors if stdout is a terminal
	if isTerminal(os.Stdout) {
		fmt.Printf(color+format+resetColor, a...)
	} else {
		fmt.Printf(format, a...)
	}
}

// Help message
var customUsageTemplate = `%ssymlink2file%s - converts symbolic links to regular files

%sUsage:%s
  %ssymlink2file%s [directory] %s[flags]%s

%sOptions:%s
{{.LocalFlags.FlagUsages}}
%sExamples:%s
  # Convert all symlinks in current directory and subdirectories
  %ssymlink2file .%s

  # Convert symlinks in /path/to/dir, delete broken ones, no backups
  %ssymlink2file --broken-symlinks delete --no-backup /path/to/dir%s

  # Convert symlinks in current directory only (no subdirectories)
  %ssymlink2file --no-recurse .%s

%sMore information:%s
  %shttps://github.com/vmikk/symlink2file%s
`

// Root command definition
var rootCmd = &cobra.Command{
	Use:     "symlink2file [directory]",
	Short:   "",
	Long:    "",
	Version: version,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Convert to absolute path
		targetDir, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("error resolving path: %w", err)
		}

		// Create context that can be cancelled by signals
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle signals for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			coloredPrintf(blueColor, "\nReceived interrupt signal, stopping...\n")
			cancel()
		}()

		// Process symlinks
		processedSymlinks := make(map[string]bool)
		if err := processSymlinks(ctx, targetDir, &noBackup, &noRecurse, brokenSymlinks, processedSymlinks); err != nil {
			if ctx.Err() != nil {
				coloredPrintf(blueColor, "Operation cancelled.\n")
				os.Exit(130) // Exit code for interrupted by signal
			}
			return fmt.Errorf("processing symlinks: %w", err)
		}

		// Count the number of processed symlinks
		count := 0
		for _, processed := range processedSymlinks {
			if processed {
				count++
			}
		}

		coloredPrintf(greenColor, "Symlink replacement complete. Processed %d symlinks.\n", count)
		return nil
	},
}

func init() {
	// Define flags with both short and long versions
	rootCmd.Flags().BoolVarP(&noBackup, "no-backup", "b", false, "Skip creating backups of replaced symlinks")
	rootCmd.Flags().BoolVarP(&noRecurse, "no-recurse", "r", false, "Process only the specified directory, skip subdirectories")
	rootCmd.Flags().StringVarP(&brokenSymlinks, "broken-symlinks", "s", "keep", "Action for broken symlinks: 'keep' or 'delete'")

	// Disable automatic flag sorting
	rootCmd.Flags().SortFlags = false

	// Set custom usage template with colors
	tmpl := fmt.Sprintf(customUsageTemplate,
		blueColor, resetColor,    // Title
		greenColor, resetColor,   // Usage header
		blueColor, resetColor,    // cmd
		greyColor, resetColor,    // Flags
		greenColor, resetColor,   // Options header
		greenColor, resetColor,   // Examples header
		greyColor, resetColor,    // example 1
		greyColor, resetColor,    // example 2
		greyColor, resetColor,    // example 3
		greenColor, resetColor,   // More information header
		greyColor, resetColor,    // URL link
	)
	rootCmd.SetUsageTemplate(tmpl)

	// Add flag completion for broken-symlinks
	rootCmd.RegisterFlagCompletionFunc("broken-symlinks", func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return []cobra.Completion{
			cobra.CompletionWithDesc("keep", "Keep broken symlinks as-is"),
			cobra.CompletionWithDesc("delete", "Delete broken symlinks"),
		}, cobra.ShellCompDirectiveDefault
	})

	// Validate broken-symlinks flag in PreRunE
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if brokenSymlinks != "keep" && brokenSymlinks != "delete" {
			return fmt.Errorf("invalid value for --broken-symlinks: %s. Must be 'keep' or 'delete'", brokenSymlinks)
		}
		return nil
	}
}

// Process the symlinks in the given directory
func processSymlinks(ctx context.Context, targetDir string, noBackup, noRecurse *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {
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
			return processPath(ctx, path, targetDir, noBackup, noRecurse, brokenSymlinks, processedSymlinks)
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
func processPath(ctx context.Context, path, targetDir string, noBackup, noRecurse *bool, brokenSymlinks string, processedSymlinks map[string]bool) error {

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if the symlink has already been processed
	if processedSymlinks[path] {
		coloredPrintf(blueColor, "Symlink already processed, skipping: %s\n", path)
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
			coloredPrintf(redColor, "Removed broken symlink: %s\n", path)
		} else {
			coloredPrintf(redColor, "Keeping broken symlink: %s\n", path)
		}
		return nil
	}

	if !*noBackup {
		if err := backupSymlink(path, targetDir, processedSymlinks); err != nil {
			return fmt.Errorf("failed to backup symlink %q: %w", path, err)
		}
	}

	targetInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to stat target path %q: %w", resolvedPath, err)
	}

	// Replace symlink with a copy of the path it points to
	if err := replaceSymlinkWithTarget(path, resolvedPath, targetInfo); err != nil {
		return fmt.Errorf("failed to replace symlink %q with its target path %q: %w", path, resolvedPath, err)
	}

	processedSymlinks[path] = true

	if targetInfo.IsDir() {
		if err := processSymlinks(ctx, path, noBackup, noRecurse, brokenSymlinks, processedSymlinks); err != nil {
			return err
		}
	}

	return nil
}

// Replace a symlink with the resolved target path
func replaceSymlinkWithTarget(symlinkPath, targetPath string, targetInfo os.FileInfo) error {
	switch {
	case targetInfo.IsDir():
		return replaceSymlinkWithDirectory(symlinkPath, targetPath, targetInfo)
	case targetInfo.Mode().IsRegular():
		return replaceSymlinkWithFile(symlinkPath, targetPath)
	default:
		return fmt.Errorf("unsupported target type for %q: %s", targetPath, targetInfo.Mode().String())
	}
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
		if tempFile != nil {
			tempFile.Close()
		}
		// Only remove temp file if it still exists (not moved)
		if _, err := os.Stat(tempPath); err == nil {
			os.Remove(tempPath)
		}
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

	// Sync to ensure data is written to disk before closing
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("error syncing temporary file: %w", err)
	}

	// Close the temporary file before moving it
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("error closing temporary file: %w", err)
	}
	tempFile = nil // Prevent defer from closing again

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

// Replace a symlink with a copied directory tree
func replaceSymlinkWithDirectory(symlinkPath, targetDirPath string, targetInfo os.FileInfo) error {
	parentDir := filepath.Dir(symlinkPath)
	tempDir, err := os.MkdirTemp(parentDir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer func() {
		if _, err := os.Stat(tempDir); err == nil {
			os.RemoveAll(tempDir)
		}
	}()

	if err := os.Chmod(tempDir, targetInfo.Mode()); err != nil {
		return fmt.Errorf("error setting temporary directory mode: %w", err)
	}

	if err := copyDirectoryContents(targetDirPath, tempDir); err != nil {
		return err
	}

	if err := os.Chtimes(tempDir, targetInfo.ModTime(), targetInfo.ModTime()); err != nil {
		return fmt.Errorf("error setting directory times: %w", err)
	}

	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("error removing symlink %q: %w", symlinkPath, err)
	}

	if err := os.Rename(tempDir, symlinkPath); err != nil {
		return fmt.Errorf("error moving temporary directory to final location: %w", err)
	}

	return nil
}

func copyDirectoryContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("error reading directory %q: %w", srcDir, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		info, err := os.Lstat(srcPath)
		if err != nil {
			return fmt.Errorf("error getting file info for %q: %w", srcPath, err)
		}

		switch {
		case info.IsDir():
			if err := copyDirectory(srcPath, dstPath, info); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			linkDest, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("error reading symlink %q: %w", srcPath, err)
			}
			if err := os.Symlink(linkDest, dstPath); err != nil {
				return fmt.Errorf("error creating symlink %q: %w", dstPath, err)
			}
		case info.Mode().IsRegular():
			if err := copyRegularFile(srcPath, dstPath, info); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported file type at %q: %s", srcPath, info.Mode().String())
		}
	}

	return nil
}

func copyDirectory(srcDir, dstDir string, dirInfo os.FileInfo) error {
	if err := os.Mkdir(dstDir, dirInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("error creating directory %q: %w", dstDir, err)
	}

	if err := os.Chmod(dstDir, dirInfo.Mode()); err != nil {
		return fmt.Errorf("error setting directory mode for %q: %w", dstDir, err)
	}

	if err := copyDirectoryContents(srcDir, dstDir); err != nil {
		return err
	}

	if err := os.Chtimes(dstDir, dirInfo.ModTime(), dirInfo.ModTime()); err != nil {
		return fmt.Errorf("error setting directory times for %q: %w", dstDir, err)
	}

	return nil
}

func copyRegularFile(srcPath, dstPath string, srcInfo os.FileInfo) error {
	inputFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening source file %q: %w", srcPath, err)
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("error creating destination file %q: %w", dstPath, err)
	}

	defer func() {
		if outputFile != nil {
			outputFile.Close()
		}
	}()

	if _, err := io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("error copying file data from %q to %q: %w", srcPath, dstPath, err)
	}

	if err := outputFile.Chmod(srcInfo.Mode()); err != nil {
		return fmt.Errorf("error setting file mode for %q: %w", dstPath, err)
	}

	if err := outputFile.Sync(); err != nil {
		return fmt.Errorf("error syncing file %q: %w", dstPath, err)
	}

	if err := outputFile.Close(); err != nil {
		return fmt.Errorf("error closing file %q: %w", dstPath, err)
	}
	outputFile = nil

	if err := os.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return fmt.Errorf("error setting file times for %q: %w", dstPath, err)
	}

	return nil
}

// Main function - entry point using Cobra
func main() {
	if err := rootCmd.Execute(); err != nil {
		coloredPrintf(redColor, "Error: %v\n", err)
		os.Exit(1)
	}
}
