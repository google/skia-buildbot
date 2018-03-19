/**
 * @module common/plot-sk
 * @description <h2><code>plot-sk</code></h2>
 *
 * @attr {boolean} width - Width of element in px.
 * @attr {boolean} height- Height of element in px.
 * @attr {boolean} specialevents - True if special lines are to produce events.
 *
 * @evt trace_selected - Event produced when the user clicks on a line.
 *     The e.detail contains the id of the line and the index of the point in the
 *     line closest to the mouse, and the [x, y] value of the point in 'pt'.
 *
 * <pre>
 * detail = {
 *   id: "id of trace",
 *   index: 3,
 *   pt: [2, 34.5],
 * }
 * </pre>
 *
 * @evt trace_focused - Event produced when the user moves the mouse close
 *     to a line. The e.detail contains the id of the line and the index of the
 *     point in the line closest to the mouse.
 *
 * <pre>
 * detail = {
 *   id: "id of trace",
 *   index: 3,
 *   pt: [2, 34.5],
 * }
 * </pre>
 *
 * @evt zoom - Event produced when the user has zoomed into a region
 *      by dragging.
 *
 * @example
 */
import { upgradeProperty } from 'skia-elements/upgrade-property'

window.customElements.define('plot-sk', class extends HTMLElement {
  static get observedAttributes() {
    return [''];
  }

  connectedCallback() {
    upgradeProperty(this, '');
    this.addEventListener('mousemove', this);
  }

  disconnectedCallback() {
    this.removeEventListener('mousemove', this);
  }

  /** @prop {boolean}  Mirrors the  attribute. */
  get () { return this.hasAttribute(''); }
  set (val) {
    if (val) {
      this.setAttribute('', '');
    } else {
      this.removeAttribute('');
    }
  }

  handleEvent(e) {
    switch (e.type) {
      case 'mousemove':
        break;
      default:
    }
  }

  attributeChangedCallback(name, oldValue, newValue) {
    let isTrue = newValue !== null;
    switch (name) {
      case '':
        break;
    }
  }
});
