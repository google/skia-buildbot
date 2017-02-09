var botMapping = {
  locations: {
    "skia-rpi-0(0[1-9]|1[0-6])": "Skolo Shelf 3a",
    "skia-rpi-0(1[7-9]|2[0-9]|3[0-2])": "Skolo Shelf 3b",
    "skia-rpi-0(3[3-9]|4[0-8])": "Skolo Shelf 2a",
    "skia-rpi-0(8[1-9]|9[0-6])": "Skolo Shelf 2b",
    "skia-rpi-0(49|5[0-9]|6[0-4])": "Skolo Shelf 1a",
    "skia-rpi-0(6[5-9]|7[0-6]|80)": "Skolo Shelf 1b",

    "skia-e-linux-00[1-4]": "Skolo Door Bottom Shelf",
    "skia-e-linux-020": "Skolo Door C",
    "skia-e-linux-021": "Skolo Door E",
    "skia-e-linux-022": "Skolo Door G",

    "skia-e-win.+": "Skia Office Shelf",

    "skia-(vm|ct-vm).*": "GCE",
    "ct-vm.*": "GCE",

    ".+(a3|a4|m3)": "Chrome Golo",
    ".+m5": "Chrome Golo (bare metal)",
  },
  ifBroken: {
    "skia-(vm|ct-vm).*": "Reboot in Cloud Console (see above)",
    "ct-vm.*": "Reboot in Cloud Console (see above)",

    "skia.*": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",

    "skia-e-win.+": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",

    ".+(m3|m5|a3)": "<a href='https://bugs.chromium.org/p/chromium/issues/entry?summary=[Device%20Restart]%20for%20_id_&description=Please%20Reboot%20_id_&cc=rmistry@google.com&components=Infra%3ELabs&labels=Pri-2,Infra-Troopers,Restrict-View-Google'> File a bug</a>",
  },
  useJumphost: {
    "skia-rpi-.+": true,
    "skia-e-linux.+": true,
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