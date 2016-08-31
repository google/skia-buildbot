// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package cli is a helper package for "github.com/maruel/subcommands".
//
// It adds a non-intrusive integration with context.Context.
package cli

import (
	"io"
	"os"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
)

// ContextModificator takes a context, adds something, and returns a new one.
//
// It is implemented by Application and can optionally by implemented by
// subcommands.CommandRun instances. It will be called by GetContext to modify
// initial context.
type ContextModificator interface {
	ModifyContext(context.Context) context.Context
}

// GetContext sniffs ContextModificator in the app and in the cmd and uses them
// to derive a context for the command.
//
// Subcommands can use it to get an initial context in their 'Run' methods.
//
// Returns the background context if app doesn't implement ContextModificator.
func GetContext(app subcommands.Application, cmd subcommands.CommandRun) context.Context {
	ctx := context.Background()
	if m, _ := app.(ContextModificator); m != nil {
		ctx = m.ModifyContext(ctx)
	}
	if m, _ := cmd.(ContextModificator); m != nil {
		ctx = m.ModifyContext(ctx)
	}
	return ctx
}

// Application is like subcommands.DefaultApplication, except it also implements
// ContextModificator.
type Application struct {
	Name     string
	Title    string
	Context  func(context.Context) context.Context
	Commands []*subcommands.Command
}

var _ interface {
	subcommands.Application
	ContextModificator
} = (*Application)(nil)

// GetName implements interface subcommands.Application.
func (a *Application) GetName() string {
	return a.Name
}

// GetTitle implements interface subcommands.Application.
func (a *Application) GetTitle() string {
	return a.Title
}

// GetCommands implements interface subcommands.Application.
func (a *Application) GetCommands() []*subcommands.Command {
	return a.Commands
}

// GetOut implements interface subcommands.Application.
func (a *Application) GetOut() io.Writer {
	return os.Stdout
}

// GetErr implements interface subcommands.Application.
func (a *Application) GetErr() io.Writer {
	return os.Stderr
}

// ModifyContext implements interface ContextModificator.
//
// 'ctx' here is always context.Background().
func (a *Application) ModifyContext(ctx context.Context) context.Context {
	if a.Context != nil {
		return a.Context(ctx)
	}
	return ctx
}
