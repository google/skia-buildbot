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
import { fromParamSet, fromObject, ParamSet } from 'common-sk/modules/query';
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
      @search-controls-sk-change=${ele._searchControlsChanged}></search-controls-sk>

  <cluster-digests-sk @selection-changed=${ele._selectionChanged}></cluster-digests-sk>

  ${infoPanel(ele)}
</div>
`;
};

const infoPanel = (ele) => {
  if (!ele._selectedDigests.length) {
    return html`
<div>Click on one digest or shift click multiple digests to see more specific information.</div>

<paramset-sk clickable .paramsets=${[ele._paramsetOfAllDigests]}
  @paramset-key-click=${ele._paramKeyClicked} @paramset-key-value-click=${ele._paramValueClicked}>
</paramset-sk>`;
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

<paramset-sk clickable .paramsets=${[selectedDigestParamset]}
  @paramset-key-click=${ele._paramKeyClicked} @paramset-key-value-click=${ele._paramValueClicked}>
</paramset-sk>
`;
};

function mergeParamsets(base, extra) {
  for (const key in extra) {
    const values = base[key] || [];
    for (const value of extra[key]) {
      if (!values.includes(value)) {
        values.push(value);
      }
    }
    base[key] = values;
  }
}

function sortParamset(ps) {
  for (const key in ps) {
    ps[key].sort();
  }
}

define('cluster-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._paramset = {};
    this._searchCriteria = {
      corpus: '',
      leftHandTraceFilter: {},
      rightHandTraceFilter: {},
      includePositiveDigests: false,
      includeNegativeDigests: false,
      includeUntriagedDigests: false,
      includeDigestsNotAtHead: false,
      includeIgnoredDigests: false,
      minRGBADelta: 0,
      maxRGBADelta: 0,
      mustHaveReferenceImage: false,
      sortOrder: 'descending',
    };
    this._grouping = '';
    this._changeListID = '';
    this._crs = '';

    this._stateChanged = stateReflector(
      /* getState */() => {
        const state = SearchCriteriaToHintableObject(this._searchCriteria);
        state.grouping = this._grouping;
        state.changeListID = this._changeListID;
        state.crs = this._crs;
        return state;
      },
      /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = SearchCriteriaFromHintableObject(newState);
        this._grouping = newState.grouping;
        this._changeListID = newState.changeListID;
        this._crs = newState.crs;
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

    // These are the nodes and links that are drawn in the cluster-digests-sk. Holding onto them
    // lets us update them (e.g. their labels) and easily re-layout the diagram.
    this._renderedNodes = [];
    this._renderedLinks = [];

    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * Creates the RPC URL for fetching the data about clustering within this test (aka grouping).
   * @return {string}
   */
  _clusterURL() {
    if (!this._grouping) {
      return '';
    }
    const sc = this._searchCriteria;

    const query = { ...sc.leftHandTraceFilter };
    query.name = [this._grouping];

    const queryObj = {
      source_type: sc.corpus,
      query: fromParamSet(query),
      pos: sc.includePositiveDigests,
      neg: sc.includeNegativeDigests,
      unt: sc.includeUntriagedDigests,
      head: !sc.includeDigestsNotAtHead,
      include: sc.includeIgnoredDigests,
    };
    return `/json/clusterdiff?${fromObject(queryObj)}`;
  }

  _fetchClusterData() {
    const url = this._clusterURL();
    if (!url) {
      console.warn('no grouping/test was specified.');
      return;
    }

    const extra = this._prefetch();
    sendBeginTask(this);
    sendBeginTask(this);

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._renderedNodes = json.nodes;
        this._renderedLinks = json.links;
        this._layoutCluster();
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

    const urlObj = {
      corpus: [this._searchCriteria.corpus],
      test: [this._grouping],
      digest: [digest],
    };
    if (this._changeListID) {
      urlObj.changelist_id = [this._changeListID];
      urlObj.crs = [this._crs];
    }
    const url = `/json/details?${fromObject(urlObj)}`;

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

    const urlObj = {
      corpus: [this._searchCriteria.corpus],
      test: [this._grouping],
      left: [leftDigest],
      right: [rightDigest],
    };
    if (this._changeListID) {
      urlObj.changelist_id = [this._changeListID];
      urlObj.crs = [this._crs];
    }
    const url = `/json/diff?${fromObject(urlObj)}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._diffDetails = json;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'diff details'));
  }

  _layoutCluster() {
    $$('cluster-digests-sk', this).setData(this._renderedNodes, this._renderedLinks);
  }

  _paramKeyClicked(e) {
    const keyClicked = e.detail.key;
    for (const node of this._renderedNodes) {
      const ps = this._paramsetsByDigest[node.name];
      node.label = ps[keyClicked] || '';
    }
    this._layoutCluster();
  }

  _paramValueClicked(e) {
    const keyClicked = e.detail.key;
    const valueClicked = e.detail.value;
    for (const node of this._renderedNodes) {
      const ps = this._paramsetsByDigest[node.name];
      if (ps[keyClicked].includes(valueClicked)) {
        node.label = ps[keyClicked];
      } else {
        node.label = '';
      }
    }
    this._layoutCluster();
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

  _searchControlsChanged(e) {
    this._searchCriteria = e.detail;
    this._stateChanged();
    this._fetchClusterData();

    // Reset selection
    this._digestDetails = null;
    this._diffDetails = null;
    this._selectedDigests = [];
    this._render();
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
