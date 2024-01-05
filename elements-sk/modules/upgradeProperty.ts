// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/** @module elements-sk/upgradeProperty */

/**
 * Capture the value from the unupgraded instance and delete the property so
 * it does not shadow the custom element's own property setter.
 *
 * See this
 * [Google Developers article]{@link https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties}
 * for more details.
 *
 * @param ele -The element.
 * @param prop - The name of the property to upgrade.
 *
 * @example
 *
 * // Upgrade the 'duration' property if it was already set.
 * window.customElements.define('my-element', class extends HTMLElement {
 *   connectedCallback() {
 *     upgradeProperty(this, 'duration');
 *   }
 *
 *   get duration() { return +this.getAttribute('duration'); }
 *   set duration(val) { this.setAttribute('duration', val); }
 * });
 *
 */
export function upgradeProperty(ele: Element, prop: string): void {
  // eslint-disable-next-line no-prototype-builtins
  if (ele.hasOwnProperty(prop)) {
    const value = (ele as any)[prop];
    delete (ele as any)[prop];
    (ele as any)[prop] = value;
  }
}
