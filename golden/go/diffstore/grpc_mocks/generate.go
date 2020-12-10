// Package grpc_mocks houses mocks that cannot be in the normal place due to
// dependency cycles
package grpc_mocks

//go:generate mockery --name DiffServiceClient --dir ../ --output . --outpkg grpc_mocks
