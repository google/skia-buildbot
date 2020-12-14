# Devices

A list of Android device names and their aliases.

Note that this JSON file can be directly imported in TypeScript:

        import * as DEVICE_ALIASES_ANY from '../../modules/devices/devices.json';

        const DEVICE_ALIASES = DEVICE_ALIASES_ANY as Record<string, string>;

Just be sure to add the following to your `tsconfig.json` file:

        "resolveJsonModule": true,

See https://mariusschulz.com/blog/importing-json-modules-in-typescript

This file is updated from https://github.com/luci/luci-py/blob/master/appengine/swarming/ui2/modules/alias.js#L33
