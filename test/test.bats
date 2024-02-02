
## Load Bats helpers
setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'
    load 'test_helper/bats-file/load'
}

@test "can run symlink2file" {
    ./symlink2file -h
}

@test "symlinking workflow ok" {
    rm -rf ./test_files ./test_symlinks/
    mkdir -p ./test_files ./test_symlinks/
    echo 111 > test_files/111.txt
    ln -s "$(pwd)/test_files/111.txt" "./test_symlinks/111.txt"
    
    ## Check if links are correctly created
    assert_symlink_to test_files/111.txt test_symlinks/111.txt
}


@test "broken links, keep" {
    rm -rf ./test_files ./test_symlinks/
    mkdir -p ./test_files ./test_symlinks/
    echo 111 > test_files/111.txt
    ln -s "$(pwd)/test_files/111.txt" "./test_symlinks/111.txt"
    ln -s "$(pwd)/test_files/222.txt" "./test_symlinks/222.txt"
    
    symlink2file -broken-symlinks keep ./test_symlinks

    ## Original and symlinked files are OK
    assert_exist ./test_files/111.txt
    assert_exist ./test_symlinks/111.txt

    ## Broken link kept
    assert_link_exists ./test_symlinks/222.txt
    assert_file_not_exists ./test_symlinks/222.txt

    ## Backup in place
    assert_link_exists ./test_symlinks/.symlink2file/111.txt

    ## No broken backup
    assert_link_not_exists ./test_symlinks/.symlink2file/222.txt
}

@test "broken links, delete" {
    rm -rf ./test_files ./test_symlinks/
    mkdir -p ./test_files ./test_symlinks/
    echo 111 > test_files/111.txt
    ln -s "$(pwd)/test_files/111.txt" "./test_symlinks/111.txt"
    ln -s "$(pwd)/test_files/222.txt" "./test_symlinks/222.txt"
    
    symlink2file -broken-symlinks delete ./test_symlinks
    
    ## Original and symlinked files are OK
    assert_exist ./test_files/111.txt
    assert_exist ./test_symlinks/111.txt

    ## Broken link removed
    assert_link_not_exists ./test_symlinks/222.txt
    assert_file_not_exists ./test_symlinks/222.txt

    ## Backups in place
    assert_link_exists ./test_symlinks/.symlink2file/111.txt
    assert_link_exists ./test_symlinks/.symlink2file/222.txt
}


@test "backup enabled" {
    rm -rf ./test_files ./test_symlinks/
    mkdir -p ./test_files ./test_symlinks/
    echo 111 > test_files/111.txt
    ln -s "$(pwd)/test_files/111.txt" "./test_symlinks/111.txt"
    
    ## No backup dir prior the run, symlink present
    assert_dir_not_exists ./test_symlinks/.symlink2file/
    assert_link_exists ./test_symlinks/111.txt

    symlink2file ./test_symlinks
    
    ## Backup dir created, symlink inside
    assert_dir_exists ./test_symlinks/.symlink2file/
    assert_link_exists ./test_symlinks/.symlink2file/111.txt

    ## Link was replaced with original file
    assert_link_not_exists ./test_symlinks/111.txt
    assert_file_exists ./test_symlinks/111.txt
}


@test "backup disabled" {
    rm -rf ./test_files ./test_symlinks/
    mkdir -p ./test_files ./test_symlinks/
    echo 111 > test_files/111.txt
    ln -s "$(pwd)/test_files/111.txt" "./test_symlinks/111.txt"
    
    ## No backup dir prior the run, symlink present
    assert_dir_not_exists ./test_symlinks/.symlink2file/
    assert_link_exists ./test_symlinks/111.txt

    symlink2file --no-backup ./test_symlinks
    
    ## Backup dir is missing
    assert_dir_not_exists ./test_symlinks/.symlink2file/

    ## Link was replaced with original file
    assert_link_not_exists ./test_symlinks/111.txt
    assert_file_exists ./test_symlinks/111.txt
}

