// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gaemiddleware provides a standard set of middleware tools for luci
// appengine apps. It's built around "github.com/julienschmidt/httprouter".
//
// Usage Example
//
// Middleware is pretty basic to use. You pick one of the 'Base' functions,
// then layer middlewares, making the innermost middleware your actual handler
// function.
//
// BaseProd and BaseTest ensure that the context has a full compliment of
// luci/gae services, as well as a luci-go/common/logging service.
//
//   import (
//     "log"
//
//     "github.com/julienschmidt/httprouter"
//     "github.com/luci/gae/service/datastore"
//     "github.com/luci/luci-go/appengine/gaemiddleware"
//     "github.com/luci/luci-go/common/logging"
//   )
//
//   // Thing is just a silly datastore model for the example.
//   type Thing struct{
//     ID string `gae:"$id"`
//   }
//
//   func myHandler(c context.Context, rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
//     if err := datastore.Get(c).Put(&Thing{ID: p.ByName("thing_id")}); err != nil {
//       logging.Errorf(c, "failed to put thing: %s", err)
//       fmt.Fprintf("error: %s", err)
//       rw.WriteHeader(500)
//       return
//     }
//     rw.Write([]byte("ok"))
//   }
//
//   func init() {
//     router := httprouter.New()
//     router.GET("/internal/make_thing/:obj_id",
//       gaemiddleware.BaseProd(gaemiddleware.RequireCron(myHandler)))
//
//     log.Fatal(http.ListenAndServe(":8080", router))
//   }
package gaemiddleware
