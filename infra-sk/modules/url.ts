/**
 * Function that returns the root domain of a sub-domain.
 *
 * I.e. it will return "skia.org" if the current location is "perf.skia.org".
 *
 * In addition it will fallback to "skia.org" is case we are on corp.goog.
 */
export function rootDomain(): string {
  let ret = window.location.host.split('.').slice(-2).join('.');
  if (ret === 'corp.goog') {
    ret = 'skia.org';
  }
  return ret;
}
