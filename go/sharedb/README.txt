To compile the proto buffers definition you need to install version 3 of
protocol buffers.

To install it directly from source:
-----------------------------------

* Check out the repository and compile it run:
  ```
  $ git clone git@github.com:google/protobuf.git
  $ ./configure --diable-shared
  $ make
  $ make check
  $ sudo make install
  ```
  The 'disabled-shared' option is necessary if an older version of
  protoc is already installed on your system.


* Install the necessary go packages:
  ```
  go get -a github.com/golang/protobuf/protoc-gen-go
  go get -u google.golang.org/grpc
  ```

To generate code run in this directory:
```
protoc --go_out=plugins=grpc:. sharedb.proto
```
