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
import { SearchCriteriaToHintableObject, SearchCriteriaFromHintableObject } from '../search-controls-sk';

import '../cluster-digests-sk';
import '../digest-details-sk';

const template = (ele) => html`
<div>
  <search-controls-sk .corpora=${ele._corpora}
      .paramSet=${ele._paramset}
      .searchCriteria=${ele._searchCriteria}
      @search-controls-sk-change=${() => ele._stateChanged()}></search-controls-sk>

  <cluster-digests-sk @selection-changed=${ele._selectionChanged}></cluster-digests-sk>

  ${infoPanel(ele)}
</div>
`;

const infoPanel = (ele) => {
  if (!ele._selectedDigests.length) {
    return html`<div>Click on one digest or shift click multiple digests to see more information.`;
  }
  if (ele._selectedDigests.length === 1) {
    const details = {
      digest: ele._selectedDigests[0],
      // FIXME, original page does it with a fetch.
      test: 'foo',
      traces: {},
    };
    return html`<digest-details-sk .details=${details}></digest-details-sk>`;
  }
  if (ele._selectedDigests.length === 2) {
    const details = {
      digest: ele._selectedDigests[0],
      // FIXME, original page does it with a fetch.
      test: 'foo',
      traces: {},
    };
    const right = {
      numDiffPixels: 1689996,
      pixelDiffPercent: 99.99976,
      maxRGBADiffs: [
        255,
        255,
        255,
        0,
      ],
      dimDiffer: true,
      diffs: {
        combined: 9.306038,
        percent: 99.99976,
        pixel: 1689996,
      },
      digest: 'ec3b8f27397d99581e06eaa46d6d5837',
      status: 'negative',
    };
    return html`
<digest-details-sk .details=${details} .right=${right}>
</digest-details-sk>`;
  }
  return html`<div>This should summarize the ${ele._selectedDigests.length} digests</div>`;
};

define('cluster-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._paramset = {};
    this._searchCriteria = {
      leftHandTraceFilter: {}, // search-controls-sk depends on this being an object.
    };

    this._stateChanged = stateReflector(
      /* getState */() => SearchCriteriaToHintableObject(this._searchCriteria),
      /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = SearchCriteriaFromHintableObject(newState);
        this._fetch();
        this._render();
      },
    );

    this._selectedDigests = [];

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

  _selectionChanged(e) {
    this._selectedDigests = e.detail;
    this._render();
  }
});
