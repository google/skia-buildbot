# bot state

The https://bots.skia.org server, aka bot state server, is a centralized
management application for mobile device testing.

The goal of this application is to remove all bot state from the RPis and move
it to a centralized server, that is, today bot state is stored across a set of
files in on the RPi itself, for example if the bot is quarantined the state is
written into a specially named file in the $HOME directory. This requires
SSH'ing into each machine to delete that file when the device has recovered,
which isn't scalable.

## Background

The
[skia_mobile.py](https://chrome-internal.googlesource.com/infradata/config/+/refs/heads/master/configs/chromium-swarm/scripts/skia_mobile.py)
script, used by Swarming for our RPi
[bot_config](https://chrome-internal.googlesource.com/infradata/config/+/refs/heads/master/configs/chromium-swarm/bots.cfg#8341),
makes a bunch of assumptions that aren't true when running in a docker
container, for example, that the $HOME directory is r/w and persistent across
restarts. It is also 1,300 lines of Python code without any unit tests.
Similarly the Skia recipes make similar assumptions.

This design document lays out a plan on how to fix that, presuming that all RPi
based bots will eventually migrate over to be kubernetes based racks.

First let's record the state of where we are today. Files and assumptions that skia_mobile.py makes:

| File                                                  | Notes                                     |
| ------------------------------------------------------|-------------------------------------------|
| $HOME/chromecast.txt                                  | IP address and port of Chromecast device. |
| $HOME/android_device_dimensions.json                  | Written to remember device state.         |
| $HOME/ios_device_dimensions.json                      | IOS remembered state.                     |
| $HOME/ssh_machine.json                                | IP address of remote ssh device (CrOS)    |
| $HOME/.android/chrome_infrastructure_adbkey           | Only used/provided on Chrome bots.        |
| $HOME/.android/adbkey                                 | Required to make py-adb work.             |
| $HOME/.android/{bot-name}__android_device_status.json | Written by bot_config. Only used on CrOS. |
| $HOME/.boto                                           | Required by swarming, can be empty.       |
| $HOME/%s.bot_died_warning                             | State - Bot died.                         |
| $HOME/%s.bot_died_quarantined                         | State - Bot quarantined.                  |
| $HOME/%s.force_quarantine                             | State - Bot quarantined.                  |

Where $HOME isn't actually the home directory, but is hard-coded to
`/home/chrome-bot` for all but the ".boto" file.

Bots deployed via [sk8s](../sk8s/README.md) are supplied with `.boto` and
`abdkey` files via Docker, so these don't need to be managed by the bot state
server. All of the rest of the files functionality needs to be handled by the
bot state server.

This list doesn't include files that are read that make sense on any linux
system, such as reading files under /proc, or access to the usb device at
/dev/bus/usb or /dev/ttyAMA0.

## Flow

The communication between the bot state server and each RPi is broken into two
parts. Data going to the RPi comes from Firestore realtime updates. Each RPi
communicates back to the bot state server by writing data into Firestore.

<pre>
Firestore OnSnapshot()
  |
  V
 RPi
  |
  V
Firestore write.
</pre>

The reads and writes will go to different documents in different collections.

<pre>
Collection("bot-state").Doc("skolo-rack4").Collection("from-bot")
                                          .Collection("state")
</pre>

The `"from-bot"` collection will contain one document for each bot. The bot will
write information into that document such as information from running adb to
interrogate a local device.

The `"state"` collection will also have one document per bot which is the state
that should be returned to swarming.

Example:

<pre>
Collection("bot-state").Doc("skolo-rack4").Collection("from-bot").Doc("skolo-rack4-shelf1-002")
                                          .Collection("state").Doc("skolo-rack4-shelf1-002")
</pre>

The bot state server will run onSnapshot() for each collection and as the bot
information changes the server will make decisions about the desired state of
the bot, e.g. to quarantine the bot. There will also be a web UI for bot server
which allows the user to over-ride such decisions, such as manually forcing a
bot to be quarantined for maintenance.

## Interface with swarming

Each RPi runs a simple `skia_mobile2.py` script that simply defers all work to a
locally installed [`bot_config`](../sk8s/go/bot_config) Go executable.

### get_state

On get_state the bot_config application will return the state from Firestore.

### get_dimensions

On get_dimensions the bot_config application will return the dimensions from
Firestore.

### on_idle

On idle the bot_config application will 

## Open questions

1. Does the bot state server also provide the
   [`bot_config`](../sk8s/go/bot_config) Go executable that is run on each RPi?