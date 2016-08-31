// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Command rpc can make RPCs to pRPC servers and display their description.
//
// Subcommand call
//
// call subcommand reads from stdin, sends a request to a specified service
// at a specified server in a special format (defaults to json) and
// prints the response back to stdout.
//
//  $ echo '{"name": "Lucy"}' | rpc call :8080 helloworld.Greeter.SayHello
//  {
//          "message": "Hello Lucy"
//  }
//
// Subcommand show
//
// show subcommand resolves a name and describes the referenced entity
// in proto-like syntax. If name is not specified, lists available services.
//
//  $ rpc show :8080
//  helloworld.Greeter
//  discovery.Discovery
//
// Show a service:
//
//  $ rpc show :8080 helloworld.Greeter
//  // The greeting service definition.
//  service Greeter {
//          // Sends a greeting
//          rpc SayHello(HelloRequest) returns (HelloReply) {};
//  }
//
// Show a method:
//
//  $ rpc show :8080 helloworld.Greeter.SayHello
//  // The greeting service definition.
//  service Greeter {
//          // Sends a greeting
//          rpc SayHello(HelloRequest) returns (HelloReply) {};
//  }
//
//  // The request message containing the user's name.
//  message HelloRequest {
//          string name = 1;
//  }
//
//  // The response message containing the greetings
//  message HelloReply {
//          string message = 1;
//  }
//
// Show a type:
//
//  $ rpc show :8080 helloworld.HelloReply
//  // The response message containing the greetings
//  message HelloReply {
//          string message = 1;
//  }
package main
