/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 * The cluster-page-sk shows many digests and clusters them based on how similar they are. This
 * can help identify incorrectly triaged images or other interesting patterns.
 *
 * It is a top level element.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { fromParamSet, fromObject, ParamSet } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SearchCriteriaToHintableObject, SearchCriteriaFromHintableObject } from '../search-controls-sk';

import '../cluster-digests-sk';
import '../digest-details-sk';
import '../../../infra-sk/modules/paramset-sk';
import { SearchCriteria } from '../search-controls-sk/search-controls-sk';
import {
  ClusterDiffLink, ClusterDiffResult, Digest, DigestComparison, DigestDetails, TestName,
} from '../rpc_types';
import { ClusterDiffNodeWithLabel, ClusterDigestsSk } from '../cluster-digests-sk/cluster-digests-sk';
import { ParamSetSkClickEventDetail } from '../../../infra-sk/modules/paramset-sk/paramset-sk';

function mergeParamsets(base: ParamSet, extra: ParamSet) {
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

function sortParamset(ps: ParamSet) {
  for (const key in ps) {
    ps[key].sort();
  }
}

export class ClusterPageSk extends ElementSk {
  private static template = (ele: ClusterPageSk) => {
    if (!ele.grouping) {
      return html`<h1>Need a test to cluster by</h1>`;
    }
    return html`
      <div class=page-container>
        <search-controls-sk .corpora=${ele.corpora}
                            .paramSet=${ele.paramset}
                            .searchCriteria=${ele.searchCriteria}
                            @search-controls-sk-change=${ele.searchControlsChanged}>
        </search-controls-sk>

        <cluster-digests-sk @selection-changed=${ele.selectionChanged}></cluster-digests-sk>

        ${ClusterPageSk.infoPanel(ele)}
      </div>
    `;
  };

  private static infoPanel = (ele: ClusterPageSk) => {
    if (!ele.selectedDigests.length) {
      return html`
        <div>
          Click on one digest or shift click multiple digests to see more specific information.
          Use A/Z to Zoom In/Out and S/X to increase/decrease node distance.
        </div>

        <paramset-sk clickable
                     .paramsets=${[ele.paramsetOfAllDigests]}
                     @paramset-key-click=${ele.paramKeyClicked}
                     @paramset-key-value-click=${ele.paramValueClicked}>
        </paramset-sk>
      `;
    }
    if (ele.selectedDigests.length === 1) {
      if (ele.digestDetails) {
        return html`
          <digest-details-sk .details=${ele.digestDetails.digest}
                             .commits=${ele.digestDetails.commits}
                             .useNewAPI=${ele.useNewAPI}>
          </digest-details-sk>
        `;
      }
      return html`<h2>Loading digest details</h2>`;
    }
    if (ele.selectedDigests.length === 2) {
      if (ele.diffDetails) {
        return html`
          <digest-details-sk .details=${ele.diffDetails.left}
                             .right=${ele.diffDetails.right}
                             .useNewAPI=${ele.useNewAPI}>
          </digest-details-sk>`;
      }
      return html`<h2>Loading diff details</h2>`;
    }

    const selectedDigestParamset = {};
    for (const digest of ele.selectedDigests) {
      mergeParamsets(selectedDigestParamset, ele.paramsetsByDigest[digest]);
    }
    sortParamset(selectedDigestParamset);

    return html`
      <div>Summary of ${ele.selectedDigests.length} digests</div>

      <paramset-sk clickable
                   .paramsets=${[selectedDigestParamset]}
                   @paramset-key-click=${ele.paramKeyClicked}
                   @paramset-key-value-click=${ele.paramValueClicked}>
      </paramset-sk>
    `;
  };

  private corpora: string[] = [];

  private paramset: ParamSet = {};

  // TODO(kjlubick): Add a specific type for cluster requests.
  private searchCriteria: SearchCriteria = {
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

  private grouping: TestName = '';

  private changeListID: string = '';

  private crs: string = '';

  private useNewAPI: boolean = false;

  // Keeps track of the digests the user has selected.
  private selectedDigests: Digest[] = [];

  // The combined paramset of all digests we loaded and displayed.
  private paramsetOfAllDigests: ParamSet = {};

  // A map of digest -> paramset. Useful for showing the params of the selected digests.
  private paramsetsByDigest: { [key: string]: ParamSet } = {};

  // These are the nodes and links that are drawn in the cluster-digests-sk. Holding onto them
  // lets us update them (e.g. their labels) and easily re-layout the diagram.
  private renderedNodes: ClusterDiffNodeWithLabel[] = [];

  private renderedLinks: ClusterDiffLink[] = [];

  private digestDetails: DigestDetails | null = null;

  private diffDetails: DigestComparison | null = null;

  // Allows us to abort fetches if we fetch again.
  private fetchController?: AbortController;

  private readonly stateChanged: ()=> void;

  private readonly keyEventHandler: (e: KeyboardEvent)=> void;

  constructor() {
    super(ClusterPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */() => {
        const state = SearchCriteriaToHintableObject(this.searchCriteria) as any;
        state.grouping = this.grouping;
        state.changeListID = this.changeListID;
        state.crs = this.crs;
        state.use_new_api = this.useNewAPI || '';
        return state;
      },
      /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        this.searchCriteria = SearchCriteriaFromHintableObject(newState);
        this.grouping = newState.grouping as string;
        this.changeListID = newState.changeListID as string;
        this.crs = newState.crs as string;
        this.useNewAPI = (newState.use_new_api as boolean) || false;
        this.fetchClusterData();
        this._render();
      },
    );

    this.keyEventHandler = (e: KeyboardEvent) => this.keyPressed(e);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // This assumes that there is only one multi-zoom-sk rendered on the page at a time (if there
    // are multiple, they may all respond to keypresses at once).
    document.addEventListener('keydown', this.keyEventHandler);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this.keyEventHandler);
  }

  /**
   * Creates the RPC URL for fetching the data about clustering within this test (aka grouping).
   */
  private clusterURL(): string {
    if (!this.grouping) {
      return '';
    }
    const sc = this.searchCriteria;

    const query: ParamSet = { ...sc.leftHandTraceFilter };
    query.name = [this.grouping];

    const queryObj: HintableObject = {
      source_type: sc.corpus,
      query: fromParamSet(query),
      pos: sc.includePositiveDigests,
      neg: sc.includeNegativeDigests,
      unt: sc.includeUntriagedDigests,
      head: !sc.includeDigestsNotAtHead,
      include: sc.includeIgnoredDigests,
    };
    const url = this.useNewAPI ? '/json/v2/clusterdiff' : '/json/v1/clusterdiff';
    return `${url}?${fromObject(queryObj)}`;
  }

  private fetchClusterData() {
    const url = this.clusterURL();
    if (!url) {
      console.warn('no grouping/test was specified.');
      return;
    }

    const extra = this.prefetch();
    sendBeginTask(this);
    sendBeginTask(this);

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((clusterDiffResult: ClusterDiffResult) => {
        this.renderedNodes = clusterDiffResult.nodes || [];
        this.renderedLinks = clusterDiffResult.links || [];
        this.layoutCluster();
        // TODO(kjlubick) remove json.test from the RPC value ( we have it in this.grouping)
        this.paramsetOfAllDigests = clusterDiffResult.paramsetsUnion;
        this.paramsetsByDigest = clusterDiffResult.paramsetByDigest;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'clusterdiff'));

    const paramsetURL = this.useNewAPI ? '/json/v2/paramset' : '/json/v1/paramset';
    fetch(paramsetURL, extra)
      .then(jsonOrThrow)
      .then((paramset: ParamSet) => {
        // We split the paramset into a list of corpora...
        this.corpora = paramset.source_type || [];
        // ...and the rest of the keys. This is to make it so the layout is
        // consistent with other pages (e.g. the search page, the by blame page, etc).
        delete paramset.source_type;

        // This cluster page is locked into the specific grouping (aka test name); We shouldn't
        // support clustering across tests unless we absolutely need to. Doing so would probably
        // require some backend changes.
        delete paramset.name;
        this.paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  private fetchDetails(digest: Digest) {
    const extra = this.prefetch();
    sendBeginTask(this);

    const urlObj: HintableObject = {
      corpus: [this.searchCriteria.corpus],
      test: [this.grouping],
      digest: [digest],
    };
    if (this.changeListID) {
      urlObj.changelist_id = [this.changeListID];
      urlObj.crs = [this.crs];
    }
    const base = this.useNewAPI ? '/json/v2/details' : '/json/v1/details';
    const url = `${base}?${fromObject(urlObj)}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((digestDetails: DigestDetails) => {
        this.digestDetails = digestDetails;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'digest details'));
  }

  private fetchDiff(leftDigest: Digest, rightDigest: Digest) {
    const extra = this.prefetch();
    sendBeginTask(this);

    const urlObj: HintableObject = {
      corpus: [this.searchCriteria.corpus],
      test: [this.grouping],
      left: [leftDigest],
      right: [rightDigest],
    };
    if (this.changeListID) {
      urlObj.changelist_id = [this.changeListID];
      urlObj.crs = [this.crs];
    }
    const base = this.useNewAPI ? '/json/v2/diff' : '/json/v1/diff';
    const url = `${base}?${fromObject(urlObj)}`;

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((digestComparison: DigestComparison) => {
        this.diffDetails = digestComparison;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'diff details'));
  }

  private keyPressed(e: KeyboardEvent) {
    // Advice taken from https://medium.com/@uistephen/keyboardevent-key-for-cross-browser-key-press-check-61dbad0a067a
    const cluster = this.querySelector<ClusterDigestsSk>('cluster-digests-sk');
    if (!cluster) {
      return;
    }
    const key = e.key || e.keyCode;
    switch (key) {
      case 'z': case 90: // Zoom in (loosen links)
        cluster.changeLinkTightness(false);
        break;
      case 'a': case 65: // Zoom out (tighten links)
        cluster.changeLinkTightness(true);
        break;
      case 's': case 83: // Increase distance between nodes
        cluster.changeNodeRepulsion(true);
        break;
      case 'x': case 88: // Decrease distance between nodes
        cluster.changeNodeRepulsion(false);
        break;
      default:
        return;
    }
    // If we captured the key event, stop it from propagating.
    e.stopPropagation();
  }

  private layoutCluster() {
    this.querySelector<ClusterDigestsSk>('cluster-digests-sk')
      ?.setData(this.renderedNodes, this.renderedLinks);
  }

  private paramKeyClicked(e: CustomEvent<ParamSetSkClickEventDetail>) {
    const keyClicked = e.detail.key;
    for (const node of this.renderedNodes) {
      const ps = this.paramsetsByDigest[node.name];
      node.label = (ps[keyClicked] || '').toString();
    }
    this.layoutCluster();
  }

  private paramValueClicked(e: CustomEvent<ParamSetSkClickEventDetail>) {
    const keyClicked = e.detail.key;
    const valueClicked = e.detail.value!;
    for (const node of this.renderedNodes) {
      const ps = this.paramsetsByDigest[node.name];
      if (ps[keyClicked].includes(valueClicked)) {
        node.label = ps[keyClicked].toString();
      } else {
        node.label = '';
      }
    }
    this.layoutCluster();
  }

  private prefetch() {
    if (this.fetchController) {
      // Kill any outstanding requests
      this.fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this.fetchController = new AbortController();
    return {
      signal: this.fetchController.signal,
    };
  }

  protected _render() {
    super._render();
    // Make the cluster draw to the full width.
    const cluster = this.querySelector<ClusterDigestsSk>('cluster-digests-sk');
    if (cluster) {
      cluster.setWidth(cluster.offsetWidth);
    }
  }

  private searchControlsChanged(e: CustomEvent<SearchCriteria>) {
    this.searchCriteria = e.detail;
    this.stateChanged();
    this.fetchClusterData();

    // Reset selection
    this.digestDetails = null;
    this.diffDetails = null;
    this.selectedDigests = [];
    this._render();
  }

  private selectionChanged(e: CustomEvent<Digest[]>) {
    this.selectedDigests = e.detail;
    const numDigests = this.selectedDigests.length;
    this.digestDetails = null;
    this.diffDetails = null;
    if (numDigests === 1) {
      this.fetchDetails(this.selectedDigests[0]);
    } else if (numDigests === 2) {
      this.fetchDiff(this.selectedDigests[0], this.selectedDigests[1]);
    }
    this._render();
  }
}

define('cluster-page-sk', ClusterPageSk);
