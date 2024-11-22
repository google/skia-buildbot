/**
 * @module module/diff-page-sk
 * @description <h2><code>diff-page-sk</code></h2>
 *
 * Page to view a specific diff between two digests. This does not include trace data.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../digest-details-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import {
  DiffRequest,
  DigestComparison,
  GroupingsResponse,
  LeftDiffInfo,
  Params,
  SRDiffDigest,
} from '../rpc_types';

export class DiffPageSk extends ElementSk {
  private static template = (ele: DiffPageSk) => {
    if (!ele.didInitialLoad) {
      return html`<p>Loading...</p>`;
    }
    if (!ele.leftDetails) {
      return html`<p>Could not load diff.</p>`;
    }
    return html`
      <digest-details-sk
        class="overview"
        .details=${ele.leftDetails}
        .right=${ele.rightDetails}
        .groupings=${ele.groupings}
        .changeListID=${ele.changeListID}
        .crs=${ele.crs}
        @image_compare_size_toggled=${ele.enableFullWidthComparison}>
      </digest-details-sk>
    `;
  };

  private groupings: GroupingsResponse | null = null;

  private grouping: Params = {};

  private leftDigest = '';

  private rightDigest = '';

  private crs = '';

  private changeListID = '';

  private leftDetails: LeftDiffInfo | null = null;

  private rightDetails: SRDiffDigest | null = null;

  private didInitialLoad = false;

  private readonly _stateChanged: () => void;

  // Allows us to abort fetches if we fetch again.
  private _fetchController?: AbortController;

  constructor() {
    super(DiffPageSk.template);

    this._stateChanged = stateReflector(
      /* getState */ () => ({
        // provide empty values
        grouping: this.grouping,
        left: this.leftDigest,
        right: this.rightDigest,
        changelist_id: this.changeListID,
        crs: this.crs,
      }),
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this.grouping = (newState.grouping as Params) || {};
        this.leftDigest = (newState.left as string) || '';
        this.rightDigest = (newState.right as string) || '';
        this.changeListID = (newState.changelist_id as string) || '';
        this.crs = (newState.crs as string) || '';
        this.fetchGroupingsOnce();
        this.fetchDigestComparison();
        this._render();
      }
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private async fetchGroupingsOnce() {
    // Only fetch once. We assume this doesn't change during the page's lifetime.
    if (this.groupings) return;

    try {
      sendBeginTask(this);
      this.groupings = await fetch('/json/v1/groupings', {
        method: 'GET',
      }).then(jsonOrThrow);
      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching groupings');
    }
  }

  private fetchDigestComparison() {
    // Kill any outstanding requests
    this._fetchController?.abort();

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    const extra = {
      signal: this._fetchController.signal,
    };
    sendBeginTask(this);

    const request: DiffRequest = {
      grouping: this.grouping,
      left_digest: this.leftDigest,
      right_digest: this.rightDigest,
    };
    if (this.changeListID && this.crs) {
      request.changelist_id = this.changeListID;
      request.crs = this.crs;
    }

    fetch('/json/v2/diff', {
      method: 'POST',
      body: JSON.stringify(request),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((obj: DigestComparison) => {
        this.leftDetails = obj.left;
        this.rightDetails = obj.right;
        this.didInitialLoad = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => {
        this.leftDetails = null;
        this.rightDetails = null;
        this.didInitialLoad = false;
        this._render();
        sendFetchError(this, e, 'diff-details');
      });
  }

  private enableFullWidthComparison(e: CustomEvent) {
    e.stopPropagation();
    const digestDetail = this.querySelector('digest-details-sk');

    if (digestDetail && digestDetail.classList) {
      digestDetail.classList.remove('overview');
    }
  }
}

define('diff-page-sk', DiffPageSk);
