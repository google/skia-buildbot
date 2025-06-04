package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateServer_Success(t *testing.T) {
	flags := &mcpFlags{
		ServiceName: string(HelloWorld),
	}

	server, err := createMcpServer(flags)
	assert.Nil(t, err)
	assert.NotNil(t, server)
}

func TestCreateServer_Invalid(t *testing.T) {
	flags := &mcpFlags{
		ServiceName: "random",
	}

	server, err := createMcpServer(flags)
	assert.NotNil(t, err)
	assert.Nil(t, server)
}
