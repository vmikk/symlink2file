# symlink2file
[![Go](https://github.com/vmikk/symlink2file/actions/workflows/go.yml/badge.svg)](https://github.com/vmikk/symlink2file/actions/workflows/go.yml)

## Overview

`symlink2file` is a command-line tool designed for Unix-like operating systems. 
Its primary function is to replace symbolic links (symlinks) in a specified directory 
with the actual files they point to. This tool may be useful in scenarios where symlinks 
need to be converted to regular files, such as for archival purposes, 
simplifying file structures, or preparing data for environments that do not support symlinks.

## Features

- Symlink resolving: Recursively resolves symlinks (absolute and relative), including those pointing to other symlinks, to ensure the final result is a regular file;
- Backup: Provides an option to backup original symlinks before replacement;
- Subdirectory traversal (optional);
- Broken symlink handling: Offers configurable behavior for dealing with broken symlinks - either keep them as-is or delete them.
- Preservation of file attributes: Attempts to preserve the original file attributes (like creation time) where possible.


## Usage

Basic usage:
```
./symlink2file [OPTIONS] <directory>
```

Options:
- `--no-backup`: Disable backup of original symlinks;
- `--broken-symlinks=keep|delete`: Define how to handle broken symlinks (default: `keep`);
- `--no-recurse`: Disable recursive traversal of subdirectories.

Example:
```
./symlink2file --no-backup --broken-symlinks delete ./path/to/directory
```

This command will replace all symlinks in `./path/to/directory` with their target files, 
without creating backups, 
and will delete any broken symlinks found.


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
go build -ldflags="-s -w" symlink2file.go
```

This will create an executable named `symlink2file` in the current directory.


### Run tests

Tests are written using [Bats](https://github.com/bats-core/bats-core) and can be run with the following commands:

``` bash
# Pull bats submodules  
git submodule update --init --recursive

# Run tests
./test/bats/bin/bats test/test.bats
```


## Note: Experimental project

> [!CAUTION]
> Please be aware that this is an experimental project, developed primarily for testing purposes and as a means to explore and understand the capabilities of the Go programming language.  
> Therefore, the code might not cover all edge cases or be suitable for production use. Users are encouraged to use this tool in non-critical environments and to thoroughly test it in their specific use cases.  
