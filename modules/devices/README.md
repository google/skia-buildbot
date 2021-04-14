# Devices

A list of Android device names and their aliases.

This file is updated from
https://github.com/luci/luci-py/blob/master/appengine/swarming/ui2/modules/alias.js#L33.

## Importing JSON from TypeScript

Currently, the [rules_nodejs](https://github.com/bazelbuild/rules_nodejs) Bazel rules do not allow
importing JSON documents from TypeScript source files. See this
[bug](https://github.com/bazelbuild/rules_nodejs/issues/1109).

When said bug is resolved, we might want to transform `devices.ts` into a JSON document, so that it
can be imported from TypeScript sources via the
[`--resolveJsonModule`](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-2-9.html#new---resolvejsonmodule)
mechanism as follows:

```
import * as DEVICE_ALIASES_ANY from '../../modules/devices/devices.json';

const DEVICE_ALIASES = DEVICE_ALIASES_ANY as Record<string, string>;
```

The following line will need to be added to `//tsconfig.json` for this to work:

```
"resolveJsonModule": true,
```

Reference: https://mariusschulz.com/blog/importing-json-modules-in-typescript.
