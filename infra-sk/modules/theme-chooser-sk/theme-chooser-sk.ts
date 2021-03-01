/**
 * @module infra-sk/modules/theme-chooser-sk
 * @description <h2><code>theme-chooser-sk</code></h2>
 *
 * <p>
 * The <theme-chooser-sk> custom element. Imports a default and darkmode color
 * palette and provides a toggle to swap between them.  To use the themes,
 * elements in the page should set colors via the variable provided in
 * infra-sk/modules/theme-chooser-sk/theme-chooser-sk.scss Such as --primary,
 * --on-primary, --background, --on-background, etc.  Some additional classes
 * are provided for convenience (primary-container-theme-sk,
 * secondary-container-theme-sk, surface, etc)
 * </p>
 *
 * <p>
 * To change the color themes override the css variables in a ':root' selector.
 * </p>
 *
 * @attr dark - If this attribute is set during connectedCallback then the
 *   default mode will be dark.
 *
 * @evt theme-chooser-toggle Sent when the theme has changed. The detail
 *   contains the darkmode value:
 *
 *   <pre>
 *     detail {
 *       darkmode: true,
 *     }
 *   </pre>
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import 'elements-sk/icon/invert-colors-icon-sk';

const DARKMODE_DEFAULT_ATTRIBUTE = 'dark';

/** Class applied to <body> to enable darkmode, and the key in localstorage to persist it. */
export const DARKMODE_CLASS = 'darkmode';

/** Describes the "theme-chooser-toggle" event detail. */
export interface ThemeChooserSkEventDetail {
  readonly darkmode: boolean;
}

// TODO(weston): Add logic to optionally automatically compute the --on-* colors to white or black
//               based on color brightness for accessibility.
export class ThemeChooserSk extends ElementSk {
  constructor() {
    super(ThemeChooserSk.template);
  }

  private static template = () => html`<invert-colors-icon-sk></invert-colors-icon-sk>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.addEventListener('click', this._toggleTheme);

    // Set the class correctly based on window.localStorage.
    this.darkmode = this.darkmode; // eslint-disable-line no-self-assign
  }

  private _toggleTheme() {
    this.darkmode = !this.darkmode;
  }

  get darkmode(): boolean {
    // If localstore has never been set then return the default, which depends
    // on the 'dark' attribute.
    if (window.localStorage.getItem(DARKMODE_CLASS) === null) {
      return this.hasAttribute(DARKMODE_DEFAULT_ATTRIBUTE);
    }
    return window.localStorage.getItem(DARKMODE_CLASS) === 'true';
  }

  set darkmode(val: boolean) {
    // Force to be a bool.
    val = !!val;
    window.localStorage.setItem(DARKMODE_CLASS, val.toString());
    document.body.classList.toggle(DARKMODE_CLASS, this.darkmode);
    this.dispatchEvent(
      new CustomEvent<ThemeChooserSkEventDetail>('theme-chooser-toggle', {
        detail: { darkmode: val },
        bubbles: true,
      }),
    );
  }
}

// isDarkMode returns true if the application is currently set to darkmode.
//
// Note that this function is only valid after the theme-chooser-sk element has
// finished connectedCallback().
export const isDarkMode = (): boolean => window.localStorage.getItem(DARKMODE_CLASS) === 'true';

define('theme-chooser-sk', ThemeChooserSk);
