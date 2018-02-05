// Capture the value from the unupgraded instance and delete the property so
// it does not shadow the custom element's own property setter.
//
// See the following for more details:
// https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties
export function upgradeProperty(ele, prop) {
  if (ele.hasOwnProperty(prop)) {
    let value = ele[prop];
    delete ele[prop];
    ele[prop] = value;
  }
}
