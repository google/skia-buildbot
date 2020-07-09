/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 * The cluster-page-sk shows many digests and clusters them based on how similar they are. This
 * can help identify incorrectly triaged images or other interesting patterns.
 *
 * It is a top level element.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { fromParamSet, fromObject } from 'common-sk/modules/query';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SearchCriteriaToHintableObject, SearchCriteriaFromHintableObject } from '../search-controls-sk';

import '../cluster-digests-sk';
import '../digest-details-sk';
import '../../../infra-sk/modules/paramset-sk';

const template = (ele) => {
  if (!ele._grouping) {
    return html`<h1>Need a test to cluster by</h1>`;
  }
  return html`
<div>
  <search-controls-sk .corpora=${ele._corpora}
      .paramSet=${ele._paramset}
      .searchCriteria=${ele._searchCriteria}
      @search-controls-sk-change=${() => ele._stateChanged()}></search-controls-sk>

  <cluster-digests-sk @selection-changed=${ele._selectionChanged}></cluster-digests-sk>

  ${infoPanel(ele)}
</div>
`;
}

const infoPanel = (ele) => {
  if (!ele._selectedDigests.length) {
    return html`
<div>Click on one digest or shift click multiple digests to see more specific information.</div>

<!-- TODO(kjlubick) clicking on this paramset should do something. In the old version, it
caused text labels to spawn on the appropriate digests.-->
<paramset-sk .paramsets=${[ele._paramsetOfAllDigests]}></paramset-sk>`;
  }
  if (ele._selectedDigests.length === 1) {
    if (ele._digestDetails) {
      return html`<digest-details-sk .details=${ele._digestDetails}></digest-details-sk>`;
    }
    return html`<h2>Loading digest details</h2>`;
  }
  if (ele._selectedDigests.length === 2) {
    if (ele._diffDetails) {
      return html`<digest-details-sk .details=${ele._diffDetails.left}
                                     .right=${ele._diffDetails.right}></digest-details-sk>`;
    }
    return html`<h2>Loading diff details</h2>`;
  }

  const selectedDigestParamset = {};
  for (const digest of ele._selectedDigests) {
    mergeParamsets(selectedDigestParamset, ele._paramsetsByDigest[digest]);
  }
  sortParamset(selectedDigestParamset);

  return html`
<div>Summary of ${ele._selectedDigests.length} digests</div>

<!-- TODO(kjlubick) clicking on this paramset should do something. In the old version, it
caused text labels to spawn on the appropriate digests.-->
<paramset-sk .paramsets=${[selectedDigestParamset]}></paramset-sk>
`;
};

function mergeParamsets(base, extra) {
  for (const key in extra) {
    const existing = base[key] || [];
    for (const value of extra[key]) {
      if (existing.indexOf(value) >= 0) {
        continue;
      }
      existing.push(value);
    }
    base[key] = existing;
  }
}

function sortParamset(ps) {
  for (const key in ps) {
    ps[key].sort();
  }
}

function makeClusterURL(searchCriteria) {
  const queryObj = {
    source_type: searchCriteria.corpus,
    query: fromParamSet(searchCriteria.leftHandTraceFilter),
    rquery: fromParamSet(searchCriteria.rightHandTraceFilter),
    pos: searchCriteria.includePositiveDigests,
    neg: searchCriteria.includeNegativeDigests,
    unt: searchCriteria.includeUntriagedDigests,
    head: !searchCriteria.includeDigestsNotAtHead,
    include: searchCriteria.includeIgnoredDigests,
    frgbamin: searchCriteria.minRGBADelta,
    frgbamax: searchCriteria.maxRGBADelta,
  }
  return `/json/clusterdiff?${fromObject(queryObj)}`;
}

define('cluster-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._paramset = {};
    this._searchCriteria = {
      leftHandTraceFilter: {}, // search-controls-sk depends on this being an object.
    };
    FIXME(kjlubick): Grouping is a bit awkward - do we put it in paramset or monkey patch it
    on as we send it to the server? I lean towards the latter as it means the users cant change
    things we don't want them to'
    this._grouping = '';
    this._changeListID = '';

    this._stateChanged = stateReflector(
      /* getState */() => {
        const state = SearchCriteriaToHintableObject(this._searchCriteria);
        state.grouping = this._grouping;
        state.changelistID = this._changeListID;
        return state;
      },
      /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = SearchCriteriaFromHintableObject(newState);
        this._grouping = newState.grouping;
        this._changelistID = newState.changelistID;
        this._fetchClusterData();
        this._render();
      },
    );

    // Keeps track of the digests the user has selected.
    this._selectedDigests = [];

    // The combined paramset of all digests we loaded and displayed.
    this._paramsetOfAllDigests = {};

    // A map of digest -> paramset. Useful for showing the params of the selected digests.
    this._paramsetsByDigest = {};

    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _fetchClusterData() {
    const extra = this._prefetch();
    sendBeginTask(this);
    sendBeginTask(this);

    const url = makeClusterURL(this._searchCriteria);

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((json) => {
        const digestNodes = json.nodes;
        const links = json.links;
        $$('cluster-digests-sk', this).setData(digestNodes, links);
        // TODO(kjlubick) remove json.test from the RPC value ( we have it in this._grouping)
        this._paramsetOfAllDigests = json.paramsetsUnion;
        this._paramsetsByDigest = json.paramsetByDigest;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'clusterdiff'));

    fetch('/json/paramset', extra)
      .then(jsonOrThrow)
      .then((paramset) => {
        // We split the paramset into a list of corpora...
        this._corpora = paramset.source_type || [];
        // ...and the rest of the keys. This is to make it so the layout is
        // consistent with other pages (e.g. the search page, the by blame page, etc).
        delete paramset.source_type;

        // This cluster page is locked into the specific grouping (aka test name); We shouldn't
        // support clustering across tests unless we absolutely need to. Doing so would probably
        // require some backend changes.
        delete paramset.name;
        this._paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  _fetchDetails(digest) {
    const extra = this._prefetch();
    sendBeginTask(this);

    const url = `/json/details?test=${encodeURIComponent(this._grouping)}`
      + `&issue=${this._changeListID}&digest=${digest}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._digestDetails = json;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'digest details'));
  }

  _fetchDiff(leftDigest, rightDigest) {
    const extra = this._prefetch();
    sendBeginTask(this);

    const url = `/json/diff?test=${encodeURIComponent(this._grouping)}`
      + `&issue=${this._changeListID}&left=${leftDigest}&right=${rightDigest}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._diffDetails = json;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'diff details'));
  }

  _prefetch() {
    if (this._fetchController) {
      // Kill any outstanding requests
      this._fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    return {
      signal: this._fetchController.signal,
    };
  }

  _selectionChanged(e) {
    this._selectedDigests = e.detail;
    const numDigests = this._selectedDigests.length;
    this._digestDetails = null;
    this._diffDetails = null;
    if (numDigests === 1) {
      this._fetchDetails(this._selectedDigests[0]);
    } else if (numDigests === 2) {
      this._fetchDiff(this._selectedDigests[0], this._selectedDigests[1]);
    }
    this._render();
  }
});
