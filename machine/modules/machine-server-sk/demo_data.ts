import {Description} from '../json';

export const fakeNow = Date.parse('2021-06-03T18:20:30.00000Z')

// Based on a production response on 2021-06-03.
export const descriptions: Description[] = [{
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:24.97453Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATB3"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-001"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-jg6kz",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:24.974527Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 24.2,
    "TYPE_CPU": 29.1,
    "TYPE_SKIN": 26.3,
    "TYPE_USB_PORT": 23.2,
    "dumpsys_battery": 24.3
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:09.386312Z",
  "DeviceUptime": 167
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-qdgf2\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:18.710419Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["H", "HUAWEIELE-L29", "HUAWEIELE-L29_9.1.0.241C605"],
    "device_os_flavor": ["huawei"],
    "device_os_type": ["user"],
    "device_type": ["HWELE", "ELE"],
    "id": ["skia-rpi2-rack4-shelf1-002"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-qdgf2",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-qdgf2",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:18.710416Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 266
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-5hqvb\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:18.967714Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["H", "HUAWEIELE-L29", "HUAWEIELE-L29_9.1.0.241C605"],
    "device_os_flavor": ["huawei"],
    "device_os_type": ["user"],
    "device_type": ["HWELE", "ELE"],
    "id": ["skia-rpi2-rack4-shelf1-003"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-pnp6w",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:20.87764Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 22
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 183
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-q2vpj\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:15:13.910199Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["H", "HUAWEIELE-L29", "HUAWEIELE-L29_9.1.0.241C605"],
    "device_os_flavor": ["huawei"],
    "device_os_type": ["user"],
    "device_type": ["HWELE", "ELE"],
    "id": ["skia-rpi2-rack4-shelf1-004"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-b6brg",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:02.034149Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 22
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 167
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-k8fdn\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:56.440311Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["H", "HUAWEIELE-L29", "HUAWEIELE-L29_9.1.0.241C605"],
    "device_os_flavor": ["huawei"],
    "device_os_type": ["user"],
    "device_type": ["HWELE", "ELE"],
    "id": ["skia-rpi2-rack4-shelf1-005"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-mzg6v",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:21.471856Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 234
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-j9lzl\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:13.827511Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATBM"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-006"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-b5zk5",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:33.121421Z",
  "Battery": 94,
  "Temperature": {
    "TYPE_BATTERY": 24.9,
    "TYPE_CPU": 36.2,
    "TYPE_SKIN": 28.8,
    "TYPE_USB_PORT": 23.8,
    "dumpsys_battery": 24.9
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:19:19.268204Z",
  "DeviceUptime": 343
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-d86nk\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:49.07976Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PQ1A.190105.004", "PQ1A.190105.004_5148680"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["blueline"],
    "id": ["skia-rpi2-rack4-shelf1-007"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-92k5w",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:09.386348Z",
  "Battery": 99,
  "Temperature": {
    "dumpsys_battery": 24.9
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-01-12T18:33:24.063867Z",
  "DeviceUptime": 657
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:14:53.393161Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATB3"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-008"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-cgzxt",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.74389Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 24.2,
    "TYPE_CPU": 25.2,
    "TYPE_SKIN": 25.5,
    "TYPE_USB_PORT": 24,
    "dumpsys_battery": 24.2
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:12:59.603173Z",
  "DeviceUptime": 660
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:05.427786Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATB3"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-009"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-7jx68",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.282608Z",
  "Battery": 91,
  "Temperature": {
    "TYPE_BATTERY": 28.8,
    "TYPE_CPU": 41.4,
    "TYPE_SKIN": 32.3,
    "TYPE_USB_PORT": 26.4,
    "dumpsys_battery": 28.8
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:05.427785Z",
  "DeviceUptime": 891
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-ghncz\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:55.480856Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATBM"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-010"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-fptzl",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:23.312632Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 25.1,
    "TYPE_CPU": 26.2,
    "TYPE_SKIN": 26.5,
    "TYPE_USB_PORT": 25,
    "dumpsys_battery": 25
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:15:16.668578Z",
  "DeviceUptime": 899
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:09:51.501239Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATBM"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-011"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-82kjc",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:07.114313Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 23.7,
    "TYPE_CPU": 25,
    "TYPE_SKIN": 25.2,
    "TYPE_USB_PORT": 22.4,
    "dumpsys_battery": 23.7
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:09:29.775541Z",
  "DeviceUptime": 209
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:09.276658Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATBM"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-012"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-2gbk7",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:21.765318Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:18:51.688084Z",
  "DeviceUptime": 123
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:11.21949Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RPB2.200611.009", "RPB2.200611.009_6625208"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["flame"],
    "id": ["skia-rpi2-rack4-shelf1-013"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-m544q",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-m544q",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:11.219483Z",
  "Battery": 71,
  "Temperature": {
    "battery": 22.300001,
    "cpu-0-0-usr": 37.600002,
    "cpu-0-1-usr": 37.600002,
    "cpu-0-2-usr": 37.600002,
    "cpu-0-3-usr": 35.7,
    "cpu-1-0-usr": 36.100002,
    "cpu-1-1-usr": 38,
    "cpu-1-2-usr": 43.4,
    "cpu-1-3-usr": 36.9,
    "dumpsys_battery": 22.3,
    "gpuss-0-usr": 30.7,
    "gpuss-1-usr": 30.600002,
    "pa-therm": 24.088001,
    "quiet-therm": 27.137001,
    "s2mpg01_tz": 28.2,
    "sdm-therm-monitor": 26.215002,
    "usbc-therm-monitor": 22.599
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:20:11.21949Z",
  "DeviceUptime": 19964
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:35.409491Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RPB2.200611.009", "RPB2.200611.009_6625208"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["flame"],
    "id": ["skia-rpi2-rack4-shelf1-014"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-n27xx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:20.318775Z",
  "Battery": 69,
  "Temperature": {
    "battery": 20.800001,
    "cpu-0-0-usr": 23.000002,
    "cpu-0-1-usr": 23.800001,
    "cpu-0-2-usr": 23.800001,
    "cpu-0-3-usr": 23.800001,
    "cpu-1-0-usr": 22.6,
    "cpu-1-1-usr": 23.000002,
    "cpu-1-2-usr": 23.800001,
    "cpu-1-3-usr": 23.800001,
    "dumpsys_battery": 20.7,
    "gpuss-0-usr": 23.000002,
    "gpuss-1-usr": 22.800001,
    "pa-therm": 21.559002,
    "quiet-therm": 22.165,
    "s2mpg01_tz": 27.033,
    "sdm-therm-monitor": 22.043001,
    "usbc-therm-monitor": 21.226002
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:19:14.265492Z",
  "DeviceUptime": 2731
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-f2wzf\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:50.550762Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf1-015"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-wtlzl",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:00.288354Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 7398
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-gsmj4\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T18:25:04.525067Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.009", "PPR1.180610.009_4898911"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["taimen"],
    "id": ["skia-rpi2-rack4-shelf1-016"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-59kxh",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:16.689225Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 21.4
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 133
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:08.97329Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G960FXXU7DTAA"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["starlte", "exynos9810"],
    "id": ["skia-rpi2-rack4-shelf1-017"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-j67cn",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:08.973288Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 24,
    "TYPE_CPU": 29.9,
    "TYPE_POWER_AMPLIFIER": 27.2,
    "TYPE_USB_PORT": 23.1,
    "dumpsys_battery": 23.9
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:01.721247Z",
  "DeviceUptime": 688
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-r9xj6\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T21:48:16.505879Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PQ1A.190105.004", "PQ1A.190105.004_5148680"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["blueline"],
    "id": ["skia-rpi2-rack4-shelf1-018"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-brg9s",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:25.194782Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 24.7
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 692
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-vw7rp\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:02.527525Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "R16NW", "R16NW_G930FXXS2ERH6"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["herolte", "universal8890"],
    "id": ["skia-rpi2-rack4-shelf1-019"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-lhwmt",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:20.74479Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-02-17T18:01:19.710792Z",
  "DeviceUptime": 1128
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-m6kfh\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T19:28:15.769434Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "R16NW", "R16NW_G930FXXS2ERH6"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["herolte", "universal8890"],
    "id": ["skia-rpi2-rack4-shelf1-020"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-mm27w",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:21.86836Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23.8
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-05-04T18:50:49.256887Z",
  "DeviceUptime": 798
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:18:40.526174Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RPB2.200611.009", "RPB2.200611.009_6625208"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["flame"],
    "id": ["skia-rpi2-rack4-shelf1-025"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-2sq4d",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:10.461761Z",
  "Battery": 100,
  "Temperature": {
    "battery": 25.1,
    "cpu-0-0-usr": 41.7,
    "cpu-0-1-usr": 42.000004,
    "cpu-0-2-usr": 41.7,
    "cpu-0-3-usr": 40.500004,
    "cpu-1-0-usr": 40.100002,
    "cpu-1-1-usr": 42.000004,
    "cpu-1-2-usr": 45.500004,
    "cpu-1-3-usr": 54.4,
    "dumpsys_battery": 25.1,
    "gpuss-0-usr": 36.600002,
    "gpuss-1-usr": 37,
    "pa-therm": 27.944002,
    "quiet-therm": 30.834002,
    "s2mpg01_tz": 29.367,
    "sdm-therm-monitor": 29.598001,
    "usbc-therm-monitor": 25.268002
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:18:40.526174Z",
  "DeviceUptime": 284
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-g95pf\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:18:40.728482Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["N", "NRD90M", "NRD90M_G920VVRU4DRE1"],
    "device_os_flavor": ["verizon"],
    "device_os_type": ["user"],
    "device_type": ["zerofltevzw", "universal7420"],
    "id": ["skia-rpi2-rack4-shelf1-026"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-wzwnj",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:09.061003Z",
  "Battery": 56,
  "Temperature": {
    "dumpsys_battery": 24.1
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-01-29T18:25:36.675007Z",
  "DeviceUptime": 680
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:40.822586Z"
  },
  "Note": {
    "Message": "",
    "User": "jcgregorio@google.com",
    "Timestamp": "2021-04-29T20:52:12.509349Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-027"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-vcngx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:10.58401Z",
  "Battery": 100,
  "Temperature": {
    "battery": 23.800001,
    "cpu-0-0-usr": 28.300001,
    "cpu-0-1-usr": 28.300001,
    "cpu-0-2-usr": 28.300001,
    "cpu-0-3-usr": 27.1,
    "cpu-1-0-usr": 27.500002,
    "cpu-1-1-usr": 27.900002,
    "cpu-1-2-usr": 28.300001,
    "cpu-1-3-usr": 28.300001,
    "dumpsys_battery": 23.8,
    "gpuss-0-usr": 27.500002,
    "gpuss-1-usr": 26.900002,
    "pa-therm": 25.831001,
    "quiet-therm": 25.218,
    "s2mpg01_tz": 28.2,
    "sdm-therm-monitor": 25.361002,
    "usbc-therm-monitor": 24.546001
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:19:40.494972Z",
  "DeviceUptime": 76
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:58.689239Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-028"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-rsrhl",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:22.625848Z",
  "Battery": 99,
  "Temperature": {
    "battery": 24.1,
    "cpu-0-0-usr": 31.100002,
    "cpu-0-1-usr": 30.7,
    "cpu-0-2-usr": 31.100002,
    "cpu-0-3-usr": 31.100002,
    "cpu-1-0-usr": 30.300001,
    "cpu-1-1-usr": 31.100002,
    "cpu-1-2-usr": 31.500002,
    "cpu-1-3-usr": 31.500002,
    "dumpsys_battery": 24.1,
    "gpuss-0-usr": 29.2,
    "gpuss-1-usr": 29.300001,
    "pa-therm": 26.503002,
    "quiet-therm": 26.45,
    "s2mpg01_tz": 28.2,
    "sdm-therm-monitor": 26.521002,
    "usbc-therm-monitor": 24.818
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:19:52.55499Z",
  "DeviceUptime": 60
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:35.713473Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-029"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-w26kt",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.713469Z",
  "Battery": 100,
  "Temperature": {
    "battery": 20.900002,
    "cpu-0-0-usr": 24.800001,
    "cpu-0-1-usr": 24.000002,
    "cpu-0-2-usr": 24.400002,
    "cpu-0-3-usr": 24.000002,
    "cpu-1-0-usr": 23.6,
    "cpu-1-1-usr": 24.400002,
    "cpu-1-2-usr": 24.800001,
    "cpu-1-3-usr": 25.6,
    "dumpsys_battery": 20.9,
    "gpuss-0-usr": 22.800001,
    "gpuss-1-usr": 23.500002,
    "pa-therm": 21.843,
    "quiet-therm": 21.478,
    "s2mpg01_tz": 27.033,
    "sdm-therm-monitor": 21.681002,
    "usbc-therm-monitor": 21.328001
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:19:38.718002Z",
  "DeviceUptime": 2156
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-8qs8j\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:03.135873Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-030"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-rprx7",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:27.777969Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:18:57.898007Z",
  "DeviceUptime": 134
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-99bs5\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:48.330289Z"
  },
  "Note": {
    "Message": "",
    "User": "jcgregorio@google.com",
    "Timestamp": "2021-04-23T15:02:46.938235Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RP1A.200720.009", "RP1A.200720.009_6720564"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["flame"],
    "id": ["skia-rpi2-rack4-shelf1-031"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-dmcjq",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:16.765635Z",
  "Battery": 92,
  "Temperature": {
    "battery": 22.800001,
    "cpu-0-0-usr": 28.1,
    "cpu-0-1-usr": 28.500002,
    "cpu-0-2-usr": 28.500002,
    "cpu-0-3-usr": 27.800001,
    "cpu-1-0-usr": 27.800001,
    "cpu-1-1-usr": 28.1,
    "cpu-1-2-usr": 28.500002,
    "cpu-1-3-usr": 28.500002,
    "dumpsys_battery": 22.8,
    "gpuss-0-usr": 26.6,
    "gpuss-1-usr": 26.7,
    "pa-therm": 24.042002,
    "quiet-therm": 24.485,
    "s2mpg01_tz": 25.866001,
    "sdm-therm-monitor": 24.244001,
    "usbc-therm-monitor": 23.856
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-05-28T18:33:57.212467Z",
  "DeviceUptime": 3715424
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:10:35.285337Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-032"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"],
    "quarantined": ["Device [\"coral\"] has gone missing"]
  },
  "PodName": "rpi-swarming-vfcxx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.564896Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:10:23.712587Z",
  "DeviceUptime": 0
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:20.648192Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf1-033"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-qd2gn",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:20.64819Z",
  "Battery": 100,
  "Temperature": {
    "battery": 23.900002,
    "cpu-0-0-usr": 41.500004,
    "cpu-0-1-usr": 41.100002,
    "cpu-0-2-usr": 40.7,
    "cpu-0-3-usr": 39.600002,
    "cpu-1-0-usr": 39.9,
    "cpu-1-1-usr": 41.9,
    "cpu-1-2-usr": 44.2,
    "cpu-1-3-usr": 50.7,
    "dumpsys_battery": 23.9,
    "gpuss-0-usr": 35.300003,
    "gpuss-1-usr": 36.300003,
    "pa-therm": 25.980001,
    "quiet-therm": 26.929,
    "s2mpg01_tz": 30.534002,
    "sdm-therm-monitor": 27.143002,
    "usbc-therm-monitor": 24.633001
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:20.648192Z",
  "DeviceUptime": 293
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-vfqks\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:14:50.9783Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATB3"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["x1s", "exynos990"],
    "id": ["skia-rpi2-rack4-shelf1-034"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"],
    "quarantined": ["Device [\"x1s\" \"exynos990\"] has gone missing"]
  },
  "PodName": "rpi-swarming-njcqv",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:18.773179Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-05-26T04:59:52.939138Z",
  "DeviceUptime": 0
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-m7tpx\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:19.204707Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_b_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf1-035"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-m7tpx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-m7tpx",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:19.204704Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 1263
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:11.97806Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RD1A.200810.022.A4", "RD1A.200810.022.A4_6835977"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["redfin"],
    "id": ["skia-rpi2-rack4-shelf1-036"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-9mkg7",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:11.978058Z",
  "Battery": 100,
  "Temperature": {
    "battery": 23.6,
    "cellular-emergency": 24.627,
    "cpu-0-0-usr": 34.300003,
    "cpu-0-1-usr": 34.300003,
    "cpu-0-2-usr": 33.9,
    "cpu-0-3-usr": 34.300003,
    "cpu-0-4-usr": 35.300003,
    "cpu-0-5-usr": 35.600002,
    "cpu-1-0-usr": 35.9,
    "cpu-1-1-usr": 39.600002,
    "cpu-1-2-usr": 36.600002,
    "cpu-1-3-usr": 42.600002,
    "dumpsys_battery": 23.6,
    "gpuss-0-usr": 31.900002,
    "gpuss-1-usr": 31.600002,
    "skin-therm-monitor": 24.627
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:11.97806Z",
  "DeviceUptime": 896
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:25.687064Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RD1A.200810.022.A4", "RD1A.200810.022.A4_6835977"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["redfin"],
    "id": ["skia-rpi2-rack4-shelf1-037"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-bs6fw",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:25.762195Z",
  "Battery": 100,
  "Temperature": {
    "battery": 23.6,
    "cellular-emergency": 23.972002,
    "cpu-0-0-usr": 34.4,
    "cpu-0-1-usr": 34.7,
    "cpu-0-2-usr": 34.100002,
    "cpu-0-3-usr": 34.7,
    "cpu-0-4-usr": 35.4,
    "cpu-0-5-usr": 36.4,
    "cpu-1-0-usr": 36.4,
    "cpu-1-1-usr": 39.4,
    "cpu-1-2-usr": 37.100002,
    "cpu-1-3-usr": 41.4,
    "dumpsys_battery": 23.6,
    "gpuss-0-usr": 31.7,
    "gpuss-1-usr": 31.7,
    "skin-therm-monitor": 23.969002
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:19:25.687064Z",
  "DeviceUptime": 725
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-q8l4q\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T20:27:16.765869Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PQ1A.190105.004", "PQ1A.190105.004_5148680"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["blueline"],
    "id": ["skia-rpi2-rack4-shelf1-038"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-w6ls9",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:25.414924Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 22.2
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-03-24T15:06:45.125835Z",
  "DeviceUptime": 2954
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-ttlhr\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:12.324193Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PQ1A.190105.004", "PQ1A.190105.004_5148680"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["blueline"],
    "id": ["skia-rpi2-rack4-shelf1-039"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-2lnwx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.425398Z",
  "Battery": 96,
  "Temperature": {
    "dumpsys_battery": 22.6
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2020-11-10T19:12:54.417129Z",
  "DeviceUptime": 1262
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-knpdf\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:51.535344Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PQ1A.190105.004", "PQ1A.190105.004_5148680"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["blueline"],
    "id": ["skia-rpi2-rack4-shelf1-040"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-scnlj",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:20.281943Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 24.7
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 1008
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:34.631184Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RPB2.200611.009", "RPB2.200611.009_6625208"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["flame"],
    "id": ["skia-rpi2-rack4-shelf2-001"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-vj6sl",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:34.631182Z",
  "Battery": 70,
  "Temperature": {
    "battery": 23.300001,
    "cpu-0-0-usr": 41.100002,
    "cpu-0-1-usr": 41.100002,
    "cpu-0-2-usr": 40.300003,
    "cpu-0-3-usr": 39.2,
    "cpu-1-0-usr": 39.9,
    "cpu-1-1-usr": 41.9,
    "cpu-1-2-usr": 44.2,
    "cpu-1-3-usr": 48.800003,
    "dumpsys_battery": 23.2,
    "gpuss-0-usr": 35.7,
    "gpuss-1-usr": 35.9,
    "pa-therm": 25.393002,
    "quiet-therm": 29.060001,
    "s2mpg01_tz": 29.367,
    "sdm-therm-monitor": 27.902,
    "usbc-therm-monitor": 23.822
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:34.631184Z",
  "DeviceUptime": 384
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-jc57v\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:45.825701Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf2-002"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-twd65",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:11.984103Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 610
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-cl8xz\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:59.472746Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["L", "LMY47V", "LMY47V_1836172"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["grouper"],
    "id": ["skia-rpi2-rack4-shelf2-003"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-2v4rd",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:26.868364Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 2774
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-md6cp\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:06.622596Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_5800535"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["sargo"],
    "id": ["skia-rpi2-rack4-shelf2-004"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-md6cp",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-md6cp",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:21.28541Z",
  "Battery": 100,
  "Temperature": {
    "battery": 21.500002,
    "cpu0-gold-usr": 25.000002,
    "cpu0-silver-usr": 25.300001,
    "cpu1-gold-usr": 24.400002,
    "cpu1-silver-usr": 25.000002,
    "cpu2-silver-usr": 25.000002,
    "cpu3-silver-usr": 24.400002,
    "cpu4-silver-usr": 25.300001,
    "cpu5-silver-usr": 25.300001,
    "dumpsys_battery": 21.5,
    "gpu0-usr": 24.1,
    "gpu1-usr": 24.1,
    "mb-therm-monitor": 22.410002,
    "usbc-therm-monitor": 20.982
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:19:30.910927Z",
  "DeviceUptime": 1450
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Leaving recovery mode.",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:12:06.198658Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_5800535"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["sargo"],
    "id": ["skia-rpi2-rack4-shelf2-005"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-vz8sm",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:07.891434Z",
  "Battery": 100,
  "Temperature": {
    "battery": 22.000002,
    "cpu0-gold-usr": 26.900002,
    "cpu0-silver-usr": 26.900002,
    "cpu1-gold-usr": 25.900002,
    "cpu1-silver-usr": 27.2,
    "cpu2-silver-usr": 27.500002,
    "cpu3-silver-usr": 27.2,
    "cpu4-silver-usr": 27.2,
    "cpu5-silver-usr": 26.900002,
    "dumpsys_battery": 22,
    "gpu0-usr": 25.900002,
    "gpu1-usr": 26.900002,
    "mb-therm-monitor": 22.857,
    "usbc-therm-monitor": 21.607
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:12:03.485144Z",
  "DeviceUptime": 1815
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Battery low. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:15:21.236194Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.009", "PPR1.180610.009_4898911"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["sailfish"],
    "id": ["skia-rpi2-rack4-shelf2-006"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-mhhtm",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:21.225662Z",
  "Battery": 10,
  "Temperature": {
    "dumpsys_battery": 30.5
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:15:21.236194Z",
  "DeviceUptime": 1355
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-j7d9z\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:50.284336Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "R16NW", "R16NW_G930FXXS2ERH6"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["herolte", "universal8890"],
    "id": ["skia-rpi2-rack4-shelf2-007"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-hw9nt",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:25.710636Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23.1
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2020-11-24T01:53:36.659971Z",
  "DeviceUptime": 690
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-m2v84\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:17.447354Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf2-008"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-lmjss",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:30.030417Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 5122
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-j2w4r\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T18:11:17.979369Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["L", "LMY47V", "LMY47V_1836172"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["grouper"],
    "id": ["skia-rpi2-rack4-shelf2-009"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-cr9dl",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:24.404158Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 2266
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-tzfhs\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:15.122291Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "M4B30Z", "M4B30Z_3437181"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["hammerhead"],
    "id": ["skia-rpi2-rack4-shelf2-010"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"],
    "quarantined": ["Device [\"hammerhead\"] has gone missing"]
  },
  "PodName": "rpi-swarming-tzfhs",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-tzfhs",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:15.122286Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 0
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-c52cc\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:18:35.477444Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.009", "PPR1.180610.009_4898911"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["taimen"],
    "id": ["skia-rpi2-rack4-shelf2-011"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-ftz9d",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:36.158113Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 22.2
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 1601
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-952mn\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:55.718479Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf2-012"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-fqrng",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:23.619701Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 3072
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-wb98m\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:18:44.31198Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RD1A.200810.022.A4", "RD1A.200810.022.A4_6835977"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["redfin"],
    "id": ["skia-rpi2-rack4-shelf2-013"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"],
    "quarantined": ["Device [\"redfin\"] has gone missing"]
  },
  "PodName": "rpi-swarming-xbxnx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:10.972034Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T16:29:41.091837Z",
  "DeviceUptime": 0
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-6plvp\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:09.638464Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "RD1A.200810.022.A4", "RD1A.200810.022.A4_6835977"],
    "device_os_flavor": ["google"],
    "device_os_type": ["user"],
    "device_type": ["redfin"],
    "id": ["skia-rpi2-rack4-shelf2-014"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-jn5sk",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:25.311188Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:19:25.352922Z",
  "DeviceUptime": 6
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-flcl8\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:03.837707Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.009", "PPR1.180610.009_4898911"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["sailfish"],
    "id": ["skia-rpi2-rack4-shelf2-015"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"],
    "quarantined": ["Device [\"sailfish\"] has gone missing"]
  },
  "PodName": "rpi-swarming-gvhfv",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:23.865269Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-05-30T01:04:48.196939Z",
  "DeviceUptime": 0
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-g9tg7\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:10.173881Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.009", "PPR1.180610.009_4898911"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["taimen"],
    "id": ["skia-rpi2-rack4-shelf2-016"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-g9tg7",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-g9tg7",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:35.983212Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 21.4
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 650
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-x8wdn\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:59.97818Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["P", "PPR1.180610.011", "PPR1.180610.011_BNPQ-190218V100"],
    "device_os_flavor": ["tecno"],
    "device_os_type": ["user"],
    "device_type": ["TECNO-KB8", "kb8_h624"],
    "id": ["skia-rpi2-rack4-shelf2-017"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-6hdwx",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:27.632201Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 23
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 924
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-jd9kj\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-02T18:19:32.508477Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "R16NW", "R16NW_G930FXXS2ERH6"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["herolte", "universal8890"],
    "id": ["skia-rpi2-rack4-shelf2-018"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-7rs78",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:12.002694Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 21.7
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 3213
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:28.850021Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_5800535"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["sargo"],
    "id": ["skia-rpi2-rack4-shelf2-019"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-l684w",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:28.850019Z",
  "Battery": 100,
  "Temperature": {
    "battery": 24.300001,
    "cpu0-gold-usr": 37.600002,
    "cpu0-silver-usr": 33.100002,
    "cpu1-gold-usr": 34,
    "cpu1-silver-usr": 32.7,
    "cpu2-silver-usr": 32.7,
    "cpu3-silver-usr": 32.7,
    "cpu4-silver-usr": 35,
    "cpu5-silver-usr": 35.600002,
    "dumpsys_battery": 24.3,
    "gpu0-usr": 29.2,
    "gpu1-usr": 28.900002,
    "mb-therm-monitor": 26.009,
    "usbc-therm-monitor": 22.901001
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:20:28.850021Z",
  "DeviceUptime": 557
}, {
  "Mode": "recovery",
  "Annotation": {
    "Message": "Too hot. ",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:17:28.624525Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QP1A.190711.020", "QP1A.190711.020_G960FXXU7DTAA"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["starlte", "exynos9810"],
    "id": ["skia-rpi2-rack4-shelf2-020"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-4tmf8",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:28.693673Z",
  "Battery": 100,
  "Temperature": {
    "TYPE_BATTERY": 26.4,
    "TYPE_CPU": 36.1,
    "TYPE_POWER_AMPLIFIER": 31.2,
    "TYPE_USB_PORT": 25.5,
    "dumpsys_battery": 26.4
  },
  "RunningSwarmingTask": true,
  "RecoveryStart": "2021-06-03T18:17:28.624525Z",
  "DeviceUptime": 679
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-572l6\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:14.091721Z"
  },
  "Note": {
    "Message": "",
    "User": "jcgregorio@google.com",
    "Timestamp": "2021-05-27T15:40:11.297794Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["Q", "QD1A.190821.011.C4", "QD1A.190821.011.C4_5917693"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["coral"],
    "id": ["skia-rpi2-rack4-shelf2-021"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-572l6",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-572l6",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:18.439271Z",
  "Battery": 100,
  "Temperature": {
    "battery": 20.7,
    "cpu-0-0-usr": 24.000002,
    "cpu-0-1-usr": 23.2,
    "cpu-0-2-usr": 23.2,
    "cpu-0-3-usr": 22.800001,
    "cpu-1-0-usr": 24.000002,
    "cpu-1-1-usr": 24.000002,
    "cpu-1-2-usr": 25.2,
    "cpu-1-3-usr": 24.800001,
    "dumpsys_battery": 20.7,
    "gpuss-0-usr": 22.400002,
    "gpuss-1-usr": 21.500002,
    "pa-therm": 21.898,
    "quiet-therm": 21.574001,
    "s2mpg01_tz": 27.033,
    "sdm-therm-monitor": 21.542002,
    "usbc-therm-monitor": 21.426
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "2021-06-03T18:18:41.606578Z",
  "DeviceUptime": 3037
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-vg8cn\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:47.0502Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["R", "R16NW", "R16NW_G930FXXS2ERH6"],
    "device_os_flavor": ["samsung"],
    "device_os_type": ["user"],
    "device_type": ["herolte", "universal8890"],
    "id": ["skia-rpi2-rack4-shelf2-022"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-7tzxd",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:15.066272Z",
  "Battery": 100,
  "Temperature": {
    "dumpsys_battery": 22.7
  },
  "RunningSwarmingTask": false,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 2471
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-btjmh\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:20:36.243029Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "M4B30Z", "M4B30Z_3437181"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["hammerhead"],
    "id": ["skia-rpi2-rack4-shelf2-023"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-btjmh",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "rpi-swarming-btjmh",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:36.243026Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 2282
}, {
  "Mode": "available",
  "Annotation": {
    "Message": "Pod too old, requested update for \"rpi-swarming-jwbzh\"",
    "User": "machines.skia.org",
    "Timestamp": "2021-06-03T18:19:58.435653Z"
  },
  "Note": {
    "Message": "",
    "User": "",
    "Timestamp": "0001-01-01T00:00:00Z"
  },
  "Dimensions": {
    "android_devices": ["1"],
    "device_os": ["M", "MOB30Q", "MOB30Q_2975880"],
    "device_os_flavor": ["google"],
    "device_os_type": ["userdebug"],
    "device_type": ["4560MMX_sprout", "sprout"],
    "id": ["skia-rpi2-rack4-shelf2-024"],
    "inside_docker": ["1", "containerd"],
    "os": ["Android"]
  },
  "PodName": "rpi-swarming-758kw",
  "KubernetesImage": "gcr.io/skia-public/rpi-swarming-client:2020-08-18T17_53_11Z-jcgregorio-06c2067-clean",
  "ScheduledForDeletion": "",
  "PowerCycle": false,
  "LastUpdated": "2021-06-03T18:20:16.543189Z",
  "Battery": -99,
  "Temperature": null,
  "RunningSwarmingTask": true,
  "RecoveryStart": "0001-01-01T00:00:00Z",
  "DeviceUptime": 4964
}];
