var botMapping = {
  locations: {
    "skia-rpi-0(0[1-9]|1[0-6])": "Skolo Shelf 3a",
    "skia-rpi-0(1[7-9]|2[0-9]|3[0-2])": "Skolo Shelf 3b",
    "skia-rpi-0(3[3-9]|4[0-8])": "Skolo Shelf 2a",
    "skia-rpi-0(8[1-9]|9[0-6])": "Skolo Shelf 2b",
    "skia-rpi-0(49|5[0-9]|6[0-4])": "Skolo Shelf 1a",
    "skia-rpi-0(6[5-9]|7[0-6]|80)": "Skolo Shelf 1b",

    "skiabot-win-001": "Skolo Office A",
    "win8-hd7770-001": "Skolo Office B",
    "win8-hd7770-000": "Skolo Office C",
    "skiabot-shuttle-ubuntu12-gtx660-001": "Skolo Office D",
    "skiabot-shuttle-ubuntu12-gtx550ti-001": "Skolo Office E",
    "skiabot-shuttle-ubuntu12-gtx660-002": "Skolo Office F",

    "skiabot-mac-10_10-ios": "Skolo Office 2",
    "win-i75557u-000": "Skolo Office 5",
    "skiabot-macmini-10_8-002": "Skolo Office 6",
    "win-i75557u-001": "Skolo Office 7",
    "skiabot-macmini-10_8-001": "Skolo Office 8",

    "win8-4790k-001": "Skolo Door 1",
    "win8-gtx960-002": "Skolo Door 2",
    "win8-4790k-002": "Skolo Door 3",
    "win-ihd530-000": "Skolo Door 4",
    "win8-4790k-003": "Skolo Door 7",
    "win-gtx960-003": "Skolo Door A",
    "win-gtx960-004": "Skolo Door B",

    "skia-(vm|ct-vm).*": "GCE",
    "ct-vm.*": "GCE",

    ".+(a3|a4|m3)": "Chrome Golo",
    ".+m5": "Chrome Golo (bare metal)",
  },
  ifBroken: {
    "skia-(vm|ct-vm).*": "Reboot in Cloud Console (see above)",
    "ct-vm.*": "Reboot in Cloud Console (see above)",

    "skia.*": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",

    "win.+": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",

    ".+(m3|m5|a3)": "<a href='https://bugs.chromium.org/p/chromium/issues/entry?summary=[Device%20Restart]%20for%20_id_&description=Please%20Reboot%20_id_&cc=rmistry@google.com&components=Infra%3ELabs&labels=Pri-2,Infra-Troopers,Restrict-View-Google'> File a bug</a>",
  },
  useJumphost: {
    "skia-rpi-.+": true,
    "skiabot-.+ubuntu.+": true,
  },
  golo: {
    ".+(m3|m5)": true,
  },
  chrome: {
    ".+(a3|a4)": true,
  },
  cloudConsole: {
    "skia-(vm|ct-vm).*": true,
    "ct-vm.*": true,
  }
}