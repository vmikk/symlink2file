#!/bin/bash

## Create test files and symlinks
mkdir -p ./test_files ./test_symlinks
for i in {1..10}; do
    head -c 100 </dev/urandom > "./test_files/file_$i.txt"
    ln -s "$(pwd)/test_files/file_$i.txt" "./test_symlinks/file_$i.txt"
done

## Function to calculate MD5 checksum
calculate_md5() {
    md5sum "$1" | awk '{ print $1 }'
}

## Test 1: Regular usage
symlink2file ./test_symlinks
echo "Testing regular usage..."
for i in {1..10}; do
    if [ -f "./test_symlinks/file_$i.txt" ] && [ ! -L "./test_symlinks/file_$i.txt" ]; then
        original_md5=$(calculate_md5 "./test_files/file_$i.txt")
        replaced_md5=$(calculate_md5 "./test_symlinks/file_$i.txt")
        if [ "$original_md5" == "$replaced_md5" ]; then
            echo "File $i: PASSED"
        else
            echo "File $i: FAILED"
        fi
    else
        echo "File $i: MISSING or NOT A REGULAR FILE"
    fi
done


# ## Clean up for next test
# rm -rf ./test_symlinks
# mkdir ./test_symlinks
# for i in {1..10}; do
#     ln -s "$(pwd)/test_files/file_$i.txt" "./test_symlinks/file_$i.txt"
# done
# 
# ## Test 2: No-backup option
# symlink2file --no-backup ./test_symlinks
# echo "Testing no-backup option..."
# [ ! -d "./test_symlinks/symlink_backup_*" ] && echo "Backup not created: PASSED" || echo "Backup not created: FAILED"

