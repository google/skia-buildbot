// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package infrask

// Use go:generate to generate the tokens.scss file. The two values passed in are the primary
// and secondary colors in hex, but w/o the leading hash.

//go:generate bazelisk run --config=mayberemote //infra-sk/modules/gentheme/cmd:gentheme -- 005db7 006e1c ${PWD}/tokens.scss
