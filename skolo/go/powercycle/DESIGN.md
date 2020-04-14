Powercycle and power.skia.org design
====================================

See also `//power/README.md` and <http://go/skolo-powercycle>.

```
+-------------------+
|                   |------------> Swarming APIs
|                   |
|  power.skia.org   |   A
|                   +<--------+
|                   |         |
+----+--------------+         |
     ^                        |
     |E                       |       User
     |                        |         |
     |                        |         v
 +---+---------------+    +---+---------+--+
 |                   |  D |                |
 | powercycle-daemon +<---+ powercycle-cli |
 |                   |    |                |
 +-------------------+    ++--+------------+
                           |  |
       +-------------------+  |  /etc/powercycle.json5
       | B                    |
       V                      |
 +-----+--+  +-------------+  | C
 |        |  |             |  |
 | mPower |  | edge switch +<-+
 |        |  |             |
 +--------+  +-------------+
```

Current Design
--------------

When a user interacts with the powercycle-cli, it loads the configuration in from disk
(`/etc/powercycle.json5`). Production copies are checked in at `//skolo/sys/`, one per host.

power.skia.org also has a copy of these configs and polls swarming for down and quarantined
machines. When it notices a machine or attached device that has powercycling enabled, it shows it
on the web UI. For the purposes of this doc and the code in this package *device* can refer to
either a host machine or an attached mobile device.

powercycle-cli can poll power.skia.org for a list of down devices (connection A in the diagram).

When given one or more device ids by the user, powercycle-cli connects to the associated *Client*.
There are two supported flavors of Client at the moment: mPower switch and a Ubiquiti POE switch.

Both connections B and C are via SSH, but it could be via different means.

Once connected, the Client is programmed to be able to turn off ports and then turn them back on.

The CLI currently sends a web request to the daemon containing the devices it powercycled
(connection D) and this is forwarded to power.skia.org (connection E).
TODO(kjlubick@) Is anything is done with this? Does it still work?

(Potential) Future Design
-------------------------

In the original design, the powercycle-daemon is intended to be the way to talk to the Clients.
By doing so that way, a user could interact with power.skia.org and the next time the daemon
polls the web server (connection E), it could trigger powercycles and report back to the web UI
that it did so.

If a user needed to interact with powercycle-cli, the CLI would only talk to the daemon, not
directly to the clients.
