/**
 * @module infra-sk/modules/colors-sk
 * @description <h2><code>colors-sk</code></h2>
 *
 * <p>
 * The <colors-sk> custom element. Imports a default and darkmode color palette
 * and provides a toggle to swap between them.  To use the themes, elements in
 * the page should set colors via the variable provided in
 * infra-sk/modules/colors-sk/colors-sk.scss
 * Such as --primary, --on-primary, --background, --on-background, etc.  Some
 * additional classes are provided for convenience (primary-container,
 * secondary-container, surface, etc)
 * </p>
 * <p>
 * To change the color themes override the css variables in a ':root' selector.
 * </p>
 */
import { define } from 'elements-sk/define'
import 'elements-sk/icon/invert-colors-icon-sk'

// TODO(weston): Add logic to optionally automatically compute the --on-*
// colors to white or black based on color brightness for accessibility.
define('colors-sk', class extends HTMLElement {
  constructor() {
    super();
  }
  connectedCallback() {
    let icon = document.createElement('invert-colors-icon-sk');
    icon.addEventListener('click', (e) => document.body.classList.toggle('darkmode'));
    this.appendChild(icon);
    return;
  }
});
