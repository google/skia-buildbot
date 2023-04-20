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

/** @module elements-sk/define */

/**
 * Define a custom element definition. It will only define a tag name once
 * and will log an error if there's an attempt to define a tag name a second
 * time. This is useful for tests since you can't undefine a custom element.
 *
 * See also https://github.com/karma-runner/karma/issues/412
 *
 * @param tagName - The name of the tag.
 * @param cl - The class for the given tag.
 *
 * @example
 *
 * Instead of:
 *
 *     window.customElements.define('my-element', class extends HTMLElement {...});
 *
 * Use:
 *
 *     import { define } from 'elements-sk/define'
 *     define('my-element', class extends HTMLElement {...});
 *
 */
export function define(tagName: string, cl: CustomElementConstructor): void {
  if (window.customElements.get(tagName) === undefined) {
    window.customElements.define(tagName, cl);
  } else {
    console.log(
      `Multiple registration attempts for ${tagName}. ` +
        'This should only happen during testing, ' +
        "it's probably an error outside of testing."
    );
  }
}
