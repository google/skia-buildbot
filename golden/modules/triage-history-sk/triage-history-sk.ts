/**
 * @module module/triage-history-sk
 * @description <h2><code>triage-history-sk</code></h2> shows the triage history for a given
 *   Gold entry.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {TriageHistory} from '../rpc_types';

function shortenEmail(s: string): string {
  const idx = s.indexOf('@');
  if (idx >= 0) {
    return s.substring(0, idx + 1);
  }
  return s;
}

export class TriageHistorySk extends ElementSk {
  private static template = (ele: TriageHistorySk) => {
    if (!ele.history.length) {
      return '';
    }
    const mostRecent = ele.history[0];
    return html`
      <div class=message title="Last triaged on ${mostRecent.ts} by ${mostRecent.user}">
        ${diffDate(mostRecent.ts)} ago by ${shortenEmail(mostRecent.user)}
      </div>
    `;
  };

  private _history: TriageHistory[] = [];

  constructor() {
    super(TriageHistorySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop history {Array} An array of objects that have a "user" and "ts". If the "ts" provided
   *       is a string, it will be converted into a Date object before use.
   */
  get history(): TriageHistory[] { return this._history; }

  set history(history: TriageHistory[]) {
    this._history = history;
    this._history.forEach((h) => {
      if (typeof h.ts === 'string') {
        h.ts = new Date(h.ts);
      }
    });
    this._render();
  }
}

define('triage-history-sk', TriageHistorySk);
