import './index.js'

// Can't use import fetch-mock because the library isn't quite set up
// correctly for it, and we get strange errors about "this" not being defined.
const fetchMock = require('fetch-mock')

var data = {
  list: [
    {
      host_id: 'jumphost-rpi-01',
      bot_id: 'skia-rpi-039',
      dimensions: [{'value': ['1'], 'key': 'android_devices'}, {'value': ['N', 'NMF26Q'], 'key': 'device_os'}, {'value': ['sailfish'], 'key': 'device_type'}, {'value': ['skia-rpi-046'], 'key': 'id'}, {'value': ['Android'], 'key': 'os'}, {'value': ['Skia'], 'key': 'pool'}, {'value': ['Device Missing'], 'key': 'quarantined'}],
      status: 'Device Missing',
      since: new Date(new Date().getTime() - 16*60*1000),
      silenced: false,
    },
    {
      host_id: 'jumphost-rpi-01',
      bot_id: 'skia-rpi-002',
      dimensions: [{'value': ['1'], 'key': 'android_devices'}, {'value': ['N', 'NMF26Q'], 'key': 'device_os'}, {'value': ['dragon'], 'key': 'device_type'}, {'value': ['skia-rpi-002'], 'key': 'id'}, {'value': ['Android'], 'key': 'os'}, {'value': ['Skia'], 'key': 'pool'}],
      status: 'Host Missing',
      since: new Date(new Date().getTime() - 25*60*1000),
      silenced: false,
    },
    {
      host_id: 'jumphost-rpi-02',
      bot_id: 'skia-rpi-202',
      dimensions: [{'value': ['1'], 'key': 'android_devices'}, {'value': ['N', 'NMF26Q'], 'key': 'device_os'}, {'value': ['dragon'], 'key': 'device_type'}, {'value': ['skia-rpi-002'], 'key': 'id'}, {'value': ['Android'], 'key': 'os'}, {'value': ['Skia'], 'key': 'pool'}],
      status: 'Host Missing',
      since: new Date(new Date().getTime() - 95*60*1000),
      silenced: false,
    },
    {
      host_id: 'jumphost-win-01',
      bot_id: 'skia-e-win-032',
      dimensions: [{'value': ['4'], 'key': 'cores'}, {'value': ['x86', 'x86-64'], 'key': 'cpu'}, {'value': ['8086', '8086:1926'], 'key': 'gpu'}, {'value': ['skia-e-win-032'], 'key': 'id'}, {'value': ['n1-standard-4'], 'key': 'machine_type'}, {'value': ['Windows', 'Windows-10', 'Windows-10-14393'], 'key': 'os'}, {'value': ['Skia'], 'key': 'pool'}],
      status: 'Host Missing',
      since: new Date(new Date().getTime() - 68*60*1000),
      silenced: true,
    },
  ],
};
fetchMock.get('/down_bots', JSON.stringify(data));
