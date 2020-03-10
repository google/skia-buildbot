/**
 * @module module/triage-history-sk
 * @description <h2><code>triage-history-sk</code></h2> shows the triage history for a given
 *   Gold entry.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { diffDate } from '../../../common-sk/modules/human';

const template = (ele) => {
  if (!ele.history.length) {
    return '';
  }
  const mostRecent = ele.history[0];
  return html`
<div class=message title="Last triaged on ${mostRecent.ts} by ${mostRecent.user}">
  ${diffDate(mostRecent.ts)} ago by ${shortenEmail(mostRecent.user)}
</div>`;
};

function shortenEmail(s) {
  const idx = s.indexOf('@');
  if (idx >= 0) {
    return s.substring(0, idx + 1);
  }
  return s;
}

define('triage-history-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._history = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop history {Array} An array of objects that have a "user" and "ts". If the "ts" provided
   *       is a string, it will be converted into a Date object before use.
   */
  get history() { return this._history; }

  set history(history) {
    this._history = history;
    this._history.forEach((h) => {
      if (typeof h.ts === 'string') {
        h.ts = new Date(h.ts);
      }
    });
    this._render();
  }
});
