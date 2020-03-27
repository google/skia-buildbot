/**
 * @module infra-sk/modules/theme-chooser-sk
 * @description <h2><code>theme-chooser-sk</code></h2>
 *
 * <p>
 * The <theme-chooser-sk> custom element. Imports a default and darkmode color
 * palette and provides a toggle to swap between them.  To use the themes,
 * elements in the page should set colors via the variable provided in
 * infra-sk/modules/theme-chooser-sk/theme-chooser-sk.scss
 * Such as --primary, --on-primary, --background, --on-background, etc.  Some
 * additional classes are provided for convenience (primary-container-theme-sk,
 * secondary-container-theme-sk, surface, etc)
 * </p>
 * <p>
 * To change the color themes override the css variables in a ':root' selector.
 * </p>
 */
import { define } from 'elements-sk/define'
import { ElementSk } from '../ElementSk'
import { html } from 'lit-html'
import 'elements-sk/icon/invert-colors-icon-sk'

// Class applied to <body> to enable darkmode, and the key in localstorage to
// persist it.
const kDarkmodeClass = 'darkmode';

const template = (ele) => html`<invert-colors-icon-sk></invert-colors-icon-sk>`;
// TODO(weston): Add logic to optionally automatically compute the --on-*
// colors to white or black based on color brightness for accessibility.
define('theme-chooser-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.addEventListener('click', this._toggleTheme);
    document.body.classList.toggle(kDarkmodeClass, window.localStorage.getItem(kDarkmodeClass) === 'true');
  }

  _toggleTheme() {
    const classlist = document.body.classList;
    classlist.toggle(kDarkmodeClass);
    window.localStorage.setItem(kDarkmodeClass, classlist.contains(kDarkmodeClass).toString())
  }
});
