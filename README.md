# symlink2file

## Overview

`symlink2file` is a command-line tool designed for Unix-like operating systems. 
Its primary function is to replace symbolic links (symlinks) in a specified directory 
with the actual files they point to. This tool may be useful in scenarios where symlinks 
need to be converted to regular files, such as for archival purposes, 
simplifying file structures, or preparing data for environments that do not support symlinks.

## Features

- Symlink resolving: Recursively resolves symlinks, including those pointing to other symlinks, to ensure the final result is a regular file;
- Backup: Provides an option to backup original symlinks before replacement;
- Optional subdirectory traversal;
- Broken symlink handling: Offers configurable behavior for dealing with broken symlinks - either keep them as-is or delete them.
- Preservation of file attributes: Attempts to preserve the original file attributes (like creation time) where possible.

## Installation

`symlink2file` can be installed either by downloading a pre-compiled binary or by compiling the source code manually. 

### Option 1: Download Pre-compiled Binary

1. Go to the [Releases](https://github.com/vmikk/symlink2file/releases) page;
2. Download the binary;
3. Make the binary executable:
```
chmod +x symlink2file
```

### Option 2: Compile from source

1. Ensure you have [Go](https://go.dev/) (`>=1.21`) installed on your system;
2. Clone the repository and compile the source code:

```
git clone https://github.com/vmikk/symlink2file
cd symlink2file
go build
```

This will create an executable named `symlink2file` in the current directory.

