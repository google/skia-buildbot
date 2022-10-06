package protoheader

//go:generate bazelisk run //:protoc -- --go_opt=paths=source_relative --go_out=. ./header.proto
//go:generate bazelisk run //:goimports "--run_under=cd $PWD &&" -- -w header.pb.go
