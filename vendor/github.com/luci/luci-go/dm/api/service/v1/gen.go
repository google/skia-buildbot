// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

//go:generate cproto
//go:generate proto-gae -type Quest_Desc -type Quest_TemplateSpec -type AbnormalFinish -type Execution_Auth -type JsonResult -type Result
//go:generate svcdec -type DepsServer
