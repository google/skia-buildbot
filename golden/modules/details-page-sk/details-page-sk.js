/**
 * @module module/details-page-sk
 * @description <h2><code>details-page-sk</code></h2>
 *
 * Page to view details about a digest. This includes other digests similar to it and trace history.
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
  if (!ele._details.digest) {
    return html`
<div>
  Could not load details for digest ${ele._digest} and test "${ele._grouping}".
  <br>
  It might not exist or be too new so as not to be indexed yet.
</div>`;
  }
  return html`
<digest-details-sk .commits=${ele._commits} .changeListID=${ele._changeListID} .crs=${ele._crs}
                   .details=${ele._details}>
</digest-details-sk>
  `;
};

define('details-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._grouping = '';
    this._digest = '';
    this._crs = '';
    this._changeListID = '';
    this._commits = [];
    this._details = {};
    this._didInitialLoad = false;

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        test: this._grouping, // TODO(kjlubick) rename test -> grouping
        digest: this._digest,
        changelist_id: this._changeListID,
        crs: this._crs,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this._grouping = newState.test || '';
        this._digest = newState.digest || '';
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

    const url = `/json/v1/details?test=${encodeURIComponent(this._grouping)}`
      + `&digest=${encodeURIComponent(this._digest)}&changelist_id=${this._changeListID}`
      + `&crs=${this._crs}`;
    fetch(url, extra)
      .then(jsonOrThrow)
      .then((obj) => {
        this._commits = obj.commits || [];
        this._details = obj.digest || {};
        this._didInitialLoad = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => {
        this._commits = [];
        this._details = {};
        this._didInitialLoad = true;
        this._render();
        sendFetchError(this, e, 'digest-details');
      });
  }
});
