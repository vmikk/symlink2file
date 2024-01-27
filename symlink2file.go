package main

import (
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"
)

func main() {
    // Parsing command-line arguments
    noBackup := flag.Bool("no-backup", false, "Disable backup of symlinks")
    brokenSymlinks := flag.String("broken-symlinks", "keep", "How to handle broken symlinks: keep or delete")
    noRecurse := flag.Bool("no-recurse", false, "Disable recursive traversal of subdirectories")
    flag.Parse()

    // Target directory is the first non-flag argument
    if flag.NArg() != 1 {
        fmt.Println("Usage: symlink2file [OPTIONS] <directory>")
        os.Exit(1)
    }
    targetDir := flag.Arg(0)

    // Backup directory setup
    backupDir := filepath.Join(targetDir, "symlink_backup_"+time.Now().Format("20060102150405"))
    if !*noBackup {
        if err := os.MkdirAll(backupDir, 0755); err != nil {
            fmt.Println("Error creating backup directory:", err)
            os.Exit(1)
        }
    }

    // Function to process each path
    processPath := func(path string, info os.FileInfo, err error) error {
        if err != nil {
            fmt.Println("Error accessing path:", path, err)
            return nil
        }

        if info.Mode()&os.ModeSymlink != 0 {
            resolvedPath, err := filepath.EvalSymlinks(path)
            if err != nil {
                if *brokenSymlinks == "delete" {
                    if removeErr := os.Remove(path); removeErr != nil {
                        fmt.Println("Error removing broken symlink:", path, removeErr)
                    } else {
                        fmt.Println("Removed broken symlink:", path)
                    }
                } else {
                    fmt.Println("Keeping broken symlink:", path)
                }
                return nil
            }

            if !*noBackup {
                backupPath := filepath.Join(backupDir, filepath.Base(path))
                if backupErr := os.Rename(path, backupPath); backupErr != nil {
                    fmt.Println("Error backing up symlink:", path, backupErr)
                    return nil
                }
            }

            inputFile, err := os.Open(resolvedPath)
            if err != nil {
                fmt.Println("Error opening target file:", resolvedPath, err)
                return nil
            }
            defer inputFile.Close()

            outputFile, err := os.Create(path)
            if err != nil {
                fmt.Println("Error creating file:", path, err)
                return nil
            }
            defer outputFile.Close()

            if _, err = io.Copy(outputFile, inputFile); err != nil {
                fmt.Println("Error copying file:", err)
                return nil
            }

            // Attempting to preserve creation time
            if stat, err := inputFile.Stat(); err == nil {
                os.Chtimes(path, stat.ModTime(), stat.ModTime())
            }
        }

        return nil
    }

    // Walk function for directory traversal
    walkFunc := func(path string, info os.FileInfo, err error) error {
        if !info.IsDir() || path == targetDir {
            return processPath(path, info, err)
        }
        if *noRecurse {
            return filepath.SkipDir
        }
        return nil
    }

    // Process symlinks in the target directory
    filepath.Walk(targetDir, walkFunc)

    fmt.Println("Symlink replacement complete.")
}
