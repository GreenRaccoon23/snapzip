# snapzip
### Install
    go get github.com/GreenRaccoon23/snapzip
### Clone
    git clone https://github.com/GreenRaccoon23/snapzip.git
### Description
Simple command-line program to compress/decompress files into Snappy archives.  
Written in the language invented by Google, [Go](https://golang.org/), for the compression format invented by Google, [Snappy](https://github.com/google/snappy). [Snappy](https://github.com/google/snappy) aims to be ***FAST*** and stable while still maintaining reasonable compression.  
### Compatibility
This program works on **Linux** and **Android**, but it does *NOT* work on **Windows**. It also works for *both* **32-bit** and **64-bit** systems (including **arm**). Although I haven't tested it, it probably works on **Mac** as well.  
  
**NOTE:** *Android* users can use this precompiled [snapzip](https://github.com/GreenRaccoon23/szip/blob/master/Android_32/snapzip) binary. (It was compiled for armv7l processors but should work with just about any Android device. Test it first to make sure it works.). *64-bit Linux* users who don't have Go installed might be able to use the uploaded [snapzip](https://github.com/GreenRaccoon23/szip/blob/master/Linux_64/snapzip) binary. (Test it first to make sure it works.) For all other systems, Go needs to be installed. Then, Go will build and install the program automatically with this command:

    go get https://github.com/GreenRaccoon23/snapzip

### Usage
I wrote this program, `snapzip`, to make things easy and simple. It automatically tests whether a file should be compressed or decompressed (based on file signatures, not just file extensions), which means that commandline switches are unneeded. Just run:  

    snapzip file1.txt file2.sz file3.tar.sz directory

^ This command will:  
1. *compress* `file1.txt` to `file1.txt.sz`  
2. *uncompress* `file2.sz` to `file2`  
3. *uncompress and untar* `file3.tar.sz` to `file3`  
4. *tar and compress* `directory` to `directory.tar.sz`  
  
Also, `snapzip` will **never** overwrite another file; whenever it creates a new file, if another one exists with the same name, it will rename the new one automatically. For example, when running:  

    snapzip file.apk

if `file.apk.sz` already exists, the compressed file will be named `file(1).apk.sz` (unless that one already exists too, then the name will be `file(2).apk.sz`, and so on).  
### Resources
I uploaded this program for simplicity's and portability's sake (installation only requires one command and 3 seconds). For a more robust and even faster alternative written in C, go to:  
[https://github.com/kubo/snzip](https://github.com/kubo/snzip)  
The REAL credit for this program goes to those who've translated the Snappy library into Go:
[https://github.com/golang/snappy/blob/master/AUTHORS](https://github.com/golang/snappy/blob/master/AUTHORS)
