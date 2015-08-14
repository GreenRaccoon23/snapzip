# sz
### Install
    go get github.com/GreenRaccoon23/sz
### Clone
    git clone https://github.com/GreenRaccoon23/sz.git
### Description
Simple command-line program to compress files into Snappy archives.  
Written in the language invented by Google, [Go](https://golang.org/), for the compression format invented by Google, [Snappy](https://github.com/google/snappy). [Snappy](https://github.com/google/snappy) aims to be ***FAST*** and stable while still maintaining reasonable compression.  
### Usage
I wrote this program, `sz`, to make things easy and simple. It automatically tests whether a file should be compressed or decompressed (based on file signatures, not just file extensions), which means that commandline switches are unneeded. Just run:  
    sz file1.txt file2.sz file3.tar.sz directory

^ This command will compress the first file, uncompress the second, uncompress and untar the third, and create a `.tar.sz` archive of the fourth.  
Also, `sz` will *never* overwrite another file; whenever it creates a new file, if another one exists with the same name, it will rename the new file automatically. For example, if `file.apk` is being compressed and `file.apk.sz` already exists, `file.apk` will be compressed to `file(1).apk.sz` (unless `file(1).apk.sz` already exists, then it will become `file(2).apk.sz`, and so on).  
### Resources
I uploaded this program for simplicity's and portability's sake (installation only requires one command and 3 seconds). For a more robust and even faster alternative written in C, go to:  
[https://github.com/kubo/snzip](https://github.com/kubo/snzip)
