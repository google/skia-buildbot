// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// A Go GAE module requires some .go files to be present, even if it is
// a pure static module, and the gae.py tool does not support different
// runtimes in the same deployment.
// You can symlink dummy.go to your module like helloworld app does.

package static
