/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { toHintableObject, fromURL } from '../search-controls-sk';

import '../cluster-digests-sk';
import '../digest-details-sk';


const template = (ele) => html`
<div>
  <search-controls-sk .corpora=${ele._corpora}
      .paramSet=${ele._paramset}
      .searchCriteria=${ele._searchCriteria}
      @search-controls-sk-change=${() => ele._stateChanged()}></search-controls-sk>

  <cluster-digests-sk></cluster-digests-sk>

  <digest-details-sk></digest-details-sk>
</div>
`;

define('cluster-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._paramset = {};
    this._searchCriteria = {
      leftHandTraceFilter: {}, // search-controls-sk depends on this being an object.
    };

    this._stateChanged = stateReflector(
      /* getState */() => toHintableObject(this._searchCriteria),
      /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = fromURL(newState);
        this._fetch();
        this._render();
      }
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
    sendBeginTask(this);

    fetch('/json/clusterdiff?TODO=kjlubick', extra)
      .then(jsonOrThrow)
      .then((json) => {
        const digestNodes = json.nodes;
        const links = json.links;
        $$('cluster-digests-sk', this).setData(digestNodes, links);
        // TODO(kjlubick) make use of json.test, json.paramsetByDigest, json.paramsetsUnion or
        //   remove them from the RPC return value.
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'list'));

    fetch('/json/paramset', extra)
      .then(jsonOrThrow)
      .then((paramset) => {
        // We split the paramset into a list of corpora...
        this._corpora = paramset.source_type || [];
        // ...and the rest of the keys. This is to make it so the layout is
        // consistent with other pages (e.g. the search page, the by blame page, etc).
        delete paramset.source_type;
        this._paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }
});
