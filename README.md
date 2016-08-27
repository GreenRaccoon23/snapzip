# snapzip
### Install
    go get github.com/GreenRaccoon23/snapzip
### Clone
    git clone https://github.com/GreenRaccoon23/snapzip.git
### Description
Command-line program to compress/decompress files into Snappy archives.  
Written in the language invented by Google, [Go](https://golang.org/), for the compression format invented by Google, [Snappy](https://github.com/google/snappy). [Snappy](https://github.com/google/snappy) aims to be ***FAST*** and stable while still maintaining reasonable compression.  
### Compatibility
This program works on **Linux** and **Android**, but it does *NOT* work on **Windows**. It also works for *both* **32-bit** and **64-bit** processors (including arm). Although I haven't tested it, it should work on **Mac** as well.  
  
**Android** users can use [this](https://github.com/GreenRaccoon23/snapzip/raw/master/Android_32/snapzip) precompiled binary. It was compiled for armv7 processors, which almost every Android device uses currently. Test it first to make sure it works.  
  
**32-bit Linux** users can use [this](https://github.com/GreenRaccoon23/snapzip/raw/master/Linux_32/snapzip) precompiled binary. Test it first to make sure it works.  

**64-bit Linux** users can use [this](https://github.com/GreenRaccoon23/snapzip/raw/master/Linux_64/snapzip) precompiled binary. Test it first to make sure it works.  
  
**All other systems** need to have [Go](https://golang.org/dl/) installed in order to use this program. Go will build and install the program automatically with this command:

    go get github.com/GreenRaccoon23/snapzip

### Usage
I wrote this program, `snapzip`, to make things easy and simple. It automatically tests whether a file should be compressed or decompressed (based on file signatures, not just file extensions), which means that commandline switches are unneeded. Just run:  

    snapzip file1.txt file2.sz file3.tar.sz directory

^ This command will:  
1. *compress* `file1.txt` to `file1.txt.sz`  
2. *uncompress* `file2.sz` to `file2`  
3. *uncompress and untar* `file3.tar.sz` to `file3`  
4. *tar and compress* `directory` to `directory.tar.sz`  

### Additional Notes
[Snappy](https://github.com/google/snappy) compression is **extremely stable**. Personally, I've compressed and decompressed a few terabytes so far with this program and have **never** had a single corrupt file. :smile:  
  
Also, as an added convenience, `snapzip` will **never** overwrite another file; it automatically generates an unused name when creating a file. For example, when running:  

    snapzip file.js

if `file.js.sz` already exists, the compressed file will be named `file(1).js.sz` (unless that one already exists too, then the name will be `file(2).js.sz`, and so on).  

### Resources
I uploaded this program for simplicity's and portability's sake (installation only requires one command and 3 seconds). For a more robust and even faster alternative written in C, go to:  
[https://github.com/kubo/snzip](https://github.com/kubo/snzip)  
The REAL credit for this program goes to those who've translated the Snappy library into Go:
[https://github.com/golang/snappy/blob/master/AUTHORS](https://github.com/golang/snappy/blob/master/AUTHORS)
Also, Docker's source code was a huge help:
[https://github.com/docker/docker/blob/master/pkg/archive](https://github.com/docker/docker/blob/master/pkg/archive)
