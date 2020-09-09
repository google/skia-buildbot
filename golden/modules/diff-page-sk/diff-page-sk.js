/**
 * @module module/diff-page-sk
 * @description <h2><code>diff-page-sk</code></h2>
 *
 * Page to view a specific diff between two digests. This does not include trace data.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../digest-details-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';

const template = (ele) => {
  if (!ele._didInitialLoad) {
    return html`<h1>Loading...</h1>`;
  }
  return html`
<digest-details-sk .details=${ele._leftDetails} .right=${ele._rightDetails}
                   .changeListID=${ele._changeListID} .crs=${ele._crs}>
</digest-details-sk>`;
};

define('diff-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._grouping = '';
    this._leftDigest = '';
    this._rightDigest = '';
    this._crs = '';
    this._changeListID = '';
    this._leftDetails = {};
    this._rightDetails = {};
    this._didInitialLoad = false;

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        test: this._grouping, // TODO(kjlubick) rename test -> grouping
        left: this._leftDigest,
        right: this._rightDigest,
        changelist_id: this._changeListID,
        crs: this._crs,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this._grouping = newState.test || '';
        this._leftDigest = newState.left || '';
        this._rightDigest = newState.right || '';
        this._changeListID = newState.changelist_id || '';
        this._crs = newState.crs || '';
        this._fetch();
        this._render();
      },
    );

    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _fetch() {
    if (this._fetchController) {
      // Kill any outstanding requests
      this._fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    const extra = {
      signal: this._fetchController.signal,
    };
    sendBeginTask(this);

    const url = `/json/v1/diff?test=${encodeURIComponent(this._grouping)}`
      + `&left=${encodeURIComponent(this._leftDigest)}`
      + `&right=${encodeURIComponent(this._rightDigest)}`
      + `&changelist_id=${this._changeListID}&crs=${this._crs}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((obj) => {
        this._leftDetails = obj.left || {};
        this._rightDetails = obj.right || {};
        this._didInitialLoad = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => {
        this._leftDetails = {};
        this._rightDetails = {};
        this._didInitialLoad = false;
        this._render();
        sendFetchError(this, e, 'diff-details');
      });
  }
});
