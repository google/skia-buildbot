// Functions used by more than one element.
import { errorMessage } from '../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../infra-sk/modules/jsonOrThrow';
import { DEVICE_ALIASES } from '../../modules/devices/devices';

const UNKNOWN = 'unknown';

const GPU_ALIASES: Record<string, string> = {
  1002: 'AMD',
  '1002:6613': 'AMD Radeon R7 240',
  '1002:6646': 'AMD Radeon R9 M280X',
  '1002:6779': 'AMD Radeon HD 6450/7450/8450',
  '1002:679e': 'AMD Radeon HD 7800',
  '1002:6821': 'AMD Radeon HD 8870M',
  '1002:683d': 'AMD Radeon HD 7770/8760',
  '1002:9830': 'AMD Radeon HD 8400',
  '1002:9874': 'AMD Carrizo',
  '102b': 'Matrox',
  '102b:0522': 'Matrox MGA G200e',
  '102b:0532': 'Matrox MGA G200eW',
  '102b:0534': 'Matrox G200eR2',
  '10de': 'NVIDIA',
  '10de:08a4': 'NVIDIA GeForce 320M',
  '10de:08aa': 'NVIDIA GeForce 320M',
  '10de:0a65': 'NVIDIA GeForce 210',
  '10de:0fe9': 'NVIDIA GeForce GT 750M Mac Edition',
  '10de:0ffa': 'NVIDIA Quadro K600',
  '10de:104a': 'NVIDIA GeForce GT 610',
  '10de:11c0': 'NVIDIA GeForce GTX 660',
  '10de:1244': 'NVIDIA GeForce GTX 550 Ti',
  '10de:1401': 'NVIDIA GeForce GTX 960',
  '10de:1ba1': 'NVIDIA GeForce GTX 1070',
  '10de:1cb3': 'NVIDIA Quadro P400',
  8086: 'Intel',
  '8086:0046': 'Intel Ironlake HD Graphics',
  '8086:0102': 'Intel Sandy Bridge HD Graphics 2000',
  '8086:0116': 'Intel Sandy Bridge HD Graphics 3000',
  '8086:0166': 'Intel Ivy Bridge HD Graphics 4000',
  '8086:0412': 'Intel Haswell HD Graphics 4600',
  '8086:041a': 'Intel Haswell HD Graphics',
  '8086:0a16': 'Intel Haswell HD Graphics 4400',
  '8086:0a26': 'Intel Haswell HD Graphics 5000',
  '8086:0a2e': 'Intel Haswell Iris Graphics 5100',
  '8086:0d26': 'Intel Haswell Iris Pro Graphics 5200',
  '8086:0f31': 'Intel Bay Trail HD Graphics',
  '8086:1616': 'Intel Broadwell HD Graphics 5500',
  '8086:161e': 'Intel Broadwell HD Graphics 5300',
  '8086:1626': 'Intel Broadwell HD Graphics 6000',
  '8086:162b': 'Intel Broadwell Iris Graphics 6100',
  '8086:1912': 'Intel Skylake HD Graphics 530',
  '8086:1926': 'Intel Skylake Iris 540/550',
  '8086:193b': 'Intel Skylake Iris Pro 580',
  '8086:22b1': 'Intel Braswell HD Graphics',
  '8086:591e': 'Intel Kaby Lake HD Graphics 615',
  '8086:5926': 'Intel Kaby Lake Iris Plus Graphics 640',
};

/**
 * Returns the device alias for the specified device type. Eg: 'walleye'
 * returns 'Pixel 2'.
 */
export function device(dt: string): string {
  return DEVICE_ALIASES[dt] || UNKNOWN;
}

/**
 * Returns the GPU name for the specified GPU. Eg: '10de' returns 'NVIDIA'.
 */
export function gpu(g: string): string {
  if (!g || !g.split) {
    return UNKNOWN;
  }
  g = g.split('-')[0];
  return GPU_ALIASES[g] || UNKNOWN;
}

/**
 * Returns a suitable for display aka string.
 */
export function getAKAStr(aka: string): string {
  return ` (${aka})`;
}

/**
 * Does a POST to the specified URL with the specified content.
 *
 * @param {string} url - The URL to make the POST call to.
 * @param {Object} detail - Will be converted to JSON and specified as body of
                            the POST call.
 * @param {Function} action - The response of the POST call will be converted
 *                            to JSON and will be passed to the action function.
 */
export function doImpl(url: string, detail: any, action: (json: any) => any): void {
  fetch(url, {
    body: JSON.stringify(detail),
    headers: {
      'content-type': 'application/json',
    },
    credentials: 'include',
    method: 'POST',
  })
    .then(jsonOrThrow)
    .then((json) => {
      action(json);
    })
    .catch((msg) => {
      console.error(msg); // eslint-disable-line no-console
      msg.resp.then(errorMessage);
    });
}
