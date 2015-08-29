# sz
### Install
    go get github.com/GreenRaccoon23/sz
### Clone
    git clone https://github.com/GreenRaccoon23/sz.git
### Description
Simple command-line program to compress/decompress files into Snappy archives.  
Written in the language invented by Google, [Go](https://golang.org/), for the compression format invented by Google, [Snappy](https://github.com/google/snappy). [Snappy](https://github.com/google/snappy) aims to be ***FAST*** and stable while still maintaining reasonable compression.  
### Compatibility
This program works in *Linux* and *Android*, but it does **not** work in *Windows*. It also works for **both** 32-bit and 64-bit systems.  
The uploaded [sz](https://github.com/GreenRaccoon23/sz/blob/master/sz) binary is for *64-bit Linux* only. (For other systems, just run `go build sz.go` or `go install sz.go` to build your own binary.)  
### Usage
I wrote this program, `sz`, to make things easy and simple. It automatically tests whether a file should be compressed or decompressed (based on file signatures, not just file extensions), which means that commandline switches are unneeded. Just run:  

    sz file1.txt file2.sz file3.tar.sz directory

^ This command will compress the first file, uncompress the second, uncompress and untar the third, and create a `.tar.sz` archive of the fourth.  
  
Also, `sz` will **never** overwrite another file; whenever it creates a new file, if another one exists with the same name, it will rename the new one automatically. For example, when running:  

    sz file.apk

if `file.apk.sz` already exists, the compressed file will be named `file(1).apk.sz` (unless that one already exists too, then the name will be `file(2).apk.sz`, and so on).  
### Resources
I uploaded this program for simplicity's and portability's sake (installation only requires one command and 3 seconds). For a more robust and even faster alternative written in C, go to:  
[https://github.com/kubo/snzip](https://github.com/kubo/snzip)  
The REAL credit for this program goes to those who've translated the Snappy library into Go:
[https://github.com/golang/snappy/blob/master/AUTHORS](https://github.com/golang/snappy/blob/master/AUTHORS)
