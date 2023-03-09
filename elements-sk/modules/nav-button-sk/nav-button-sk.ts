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

/**
 * @module elements-sk/nav-button-sk
 * @description <h2><code>nav-button-sk<code></h2>
 *
 * <p>
 *   Allows for the creation of a pop-up menu. The actual menu is contained
 *   in a sibling <nav-links-sk> element.
 * </p>
 *
 * <p>
 *   <code>nav-button-sk</code> is just a convenience that contains the
 *   hamburger menu and toggles the shown property of a sibling
 *   'nav-links-sk'. Other types of popup menus can be created using buttons
 *   and icons directly.  The children of 'nav-links-sk' do not have to be
 *   links, they could be other elements, such as buttons.
 * </p>
 *
 * @example
 *
 * // Using nav-button-sk:
 * <nav-button-sk></nav-button-sk>
 * <nav-links-sk shown>
 *   <a href="">Main</a>
 *   <a href="">Triage</a>
 *   <a href="">Alerts</a>
 * </nav-links-sk>
 *
 * // Using a button instead of nav-button-sk:
 * <button onclick="this.nextElementSibling.shown = true;">
 *   <create-icon-sk></create-icon-sk>
 * </button>
 * <nav-links-sk>
 *   <a href="">New A</a>
 *   <a href="">New B</a>
 *   <a href="">New C</a>
 * </nav-links-sk>
 *
 */

import { define } from '../define';
import { NavLinksSk } from '../nav-links-sk/nav-links-sk';

export class NavButtonSk extends HTMLElement {
  connectedCallback(): void {
    this.addEventListener('click', this);
    this.innerHTML = '<button><menu-icon-sk></menu-icon-sk></button>';
  }

  disconnectedCallback(): void {
    this.removeEventListener('click', this);
  }

  handleEvent(e: Event): void {
    if (this.nextElementSibling?.tagName === 'NAV-LINKS-SK') {
      const navLinksSk = this.nextElementSibling as NavLinksSk;
      navLinksSk.shown = !navLinksSk.shown;
      if (navLinksSk.shown) {
        (navLinksSk.firstElementChild as HTMLElement).focus();
      }
    }
  }
}

define('nav-button-sk', NavButtonSk);
