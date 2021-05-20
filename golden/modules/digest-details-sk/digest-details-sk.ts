/**
 * @module module/digest-details-sk
 * @description <h2><code>digest-details-sk</code></h2>
 *
 * Displays the details about a digest. These details include comparing it to other digests in the
 * same grouping (e.g. test), if those are available. It provides the affordances to triage the
 * given digest and makes the POST request to triage this given digest.
 *
 * <h2>Events</h2>
 *   This element produces the following events:
 * @evt begin-task/end-task - when a POST request is in flight to handle triaging.
 * @evt triage - Emitted when the user triages the digest. e.detail contains the assigned Label.
 *
 *   Children elements emit the following events of note:
 * @evt show-commits - Event generated when a trace dot is clicked. e.detail contains
 *   the blamelist (an array of commits that could have made up that dot).
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject } from 'common-sk/modules/query';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  truncateWithEllipses, detailHref, diffPageHref, sendBeginTask, sendEndTask, sendFetchError,
} from '../common';

import 'elements-sk/icon/group-work-icon-sk';
import '../dots-sk';
import '../dots-legend-sk';
import '../triage-sk';
import '../image-compare-sk';
import '../blamelist-panel-sk';
import '../../../infra-sk/modules/paramset-sk';
import { SearchCriteriaToHintableObject } from '../search-controls-sk';
import {Commit, Digest, Label, ParamSet, SearchResult, SRDiffDigest, TestName, TraceGroup, TraceID, TriageHistory, TriageRequest} from '../rpc_types';
import {SearchCriteria, SearchCriteriaHintableObject} from '../search-controls-sk/search-controls-sk';
import {HintableObject} from 'common-sk/modules/hintable';
import {DotsSk} from '../dots-sk/dots-sk';
import {BlamelistPanelSk} from '../blamelist-panel-sk/blamelist-panel-sk';
import {LabelOrEmpty, TriageSk} from '../triage-sk/triage-sk';
import {ImageComparisonData} from '../image-compare-sk/image-compare-sk';
import {diffDate} from 'common-sk/modules/human';



function toggleButtonMouseover(canToggle: boolean) {
  if (canToggle) {
    return 'By default, Gold shows the closest image, whether it has been marked positive or '
    + 'negative. This button allows you to explicitly select the closest positive or negative.';
  }
  return 'There are no other reference image types to compare against.';
}

const validRefs = ['pos', 'neg'];

export class DigestDetailsSk extends ElementSk {
  private static template = (ele: DigestDetailsSk) => html`
    <div class=container>
      <div class=top_bar>
        <span class=grouping_name>Test: ${ele.grouping}</span>
        <span class=expand></span>
        <a href=${ele.clusterHref()} target=_blank rel=noopener class=cluster_link>
          <group-work-icon-sk title="Cluster view of this digest and all others for this test.">
          </group-work-icon-sk>
        </a>
      </div>

      <div class=comparison>
        <div class=digest_labels>
          <span class="digest_label left">Left: ${ele.digest}</span>
          <span class=expand></span>
          <span class="digest_label right" ?hidden=${!ele.right}>
            Right: ${ele.right && ele.right.digest}
          </span>
        </div>
        <div class=comparison_data>
          <div>${DigestDetailsSk.detailsAndTriageTemplate(ele)}</div>
          <div>${DigestDetailsSk.imageComparisonTemplate(ele)}</div>
          <div>
            <button
                @click=${ele.toggleRightRef}
                ?disabled=${!ele.canToggle()}
                class=toggle_ref
                ?hidden=${ele._overrideRight || !ele.right}
                title=${toggleButtonMouseover(ele.canToggle())}>
              Toggle Reference
            </button>
            <div ?hidden=${!ele.right || ele.right.status !== 'negative'} class=negative_warning>
              Closest image is negative!
            </div>
            <!-- TODO(kjlubick) Comments would go here -->
          </div>
        </div>
      </div>
    ${DigestDetailsSk.traceInfoTemplate(ele)}
    ${DigestDetailsSk.paramsetTemplate(ele)}
    </div>
    <dialog class=blamelist_dialog>
      <blamelist-panel-sk></blamelist-panel-sk>
      <button class=close_btn @click=${ele.closeBlamelistDialog}>Close</button>
    </dialog>
  `;

  private static detailsAndTriageTemplate = (ele: DigestDetailsSk) => {
    if (!ele.right) {
      return html`
        <div class=metrics_and_triage>
          <triage-sk @change=${ele.triageChangeHandler} .value=${ele.status}></triage-sk>
          ${DigestDetailsSk.triageHistoryTemplate(ele)}
      </div>
      `;
    }

    // TODO(kjlubick) would it be clearer to just tell the user the images differ in size and omit
    //  the (probably useless metrics)? Could we also include the actual dimensions of the two?

    return html`
      <div class=metrics_and_triage>
        <div>
          <a href=${diffPageHref(
                        ele.grouping,
                        ele.digest,
                        ele.right.digest,
                        ele.changeListID,
                        ele.crs)}
             target=_blank rel=noopener class=diffpage_link>
            Diff Details
          </a>
        </div>
        <div class=size_warning ?hidden=${!ele.right.dimDiffer}>Images differ in size!</div>
        <div class=metric>
          <span>Diff metric:</span>
          <span>${ele.right.combinedMetric.toFixed(3)}</span>
        </div>
        <div class=metric>
          <span>Diff %:</span>
          <span>${ele.right.pixelDiffPercent.toFixed(2)}</span>
        </div>
        <div class=metric>
          <span>Pixels:</span>
          <span>${ele.right.numDiffPixels}</span>
        </div>
        <div class=metric>
          <span>Max RGBA:</span>
          <span>[${ele.right.maxRGBADiffs.join(',')}]</span>
        </div>
        <triage-sk @change=${ele.triageChangeHandler} .value=${ele.status}></triage-sk>
        ${DigestDetailsSk.triageHistoryTemplate(ele)}
      </div>
    `;
  };

  private static triageHistoryTemplate = (ele: DigestDetailsSk) => {
    if (ele.triageHistory.length === 0) return '';

    const mostRecent = ele.triageHistory[0];
    return html`
      <div class=triage-history title="Last triaged on ${mostRecent.ts} by ${mostRecent.user}">
        ${diffDate(mostRecent.ts)} ago by
        ${mostRecent.user.includes('@')
            ? mostRecent.user.substring(0, mostRecent.user.indexOf('@') + 1)
            : mostRecent.user}
      </div>
    `;
  }

  private static imageComparisonTemplate = (ele: DigestDetailsSk) => {
    const left: ImageComparisonData = {
      digest: ele.digest,
      title: truncateWithEllipses(ele.digest),
      detail: detailHref(ele.grouping, ele.digest, ele.changeListID, ele.crs),
    };
    if (!ele.right) {
      const hasOtherDigests = (ele.traces?.digests?.length || 0) > 1;
      return html`<image-compare-sk .left=${left}
        .isComputingDiffs=${hasOtherDigests}></image-compare-sk>`;
    }

    const right: ImageComparisonData = {
      digest: ele.right.digest,
      title: ele.right.status === 'positive' ? 'Closest Positive' : 'Closest Negative',
      detail: detailHref(ele.grouping, ele.right.digest, ele.changeListID, ele.crs),
    };
    if (ele._overrideRight) {
      right.title = truncateWithEllipses(ele.right.digest);
    }

    return html`<image-compare-sk .left=${left} .right=${right}></image-compare-sk>`;
  };

  private static traceInfoTemplate = (ele: DigestDetailsSk) => {
    if (!ele.traces || !ele.traces.traces || !ele.traces.traces.length) {
      return '';
    }
    return html`
      <div class=trace_info>
        <dots-sk
            .value=${ele.traces}
            .commits=${ele._commits}
            @hover=${ele.hoverOverTrace}
            @mouseleave=${ele.clearTraceHighlights}
            @showblamelist=${ele.showBlamelist}>
        </dots-sk>
        <dots-legend-sk
            .digests=${ele.traces.digests}
            .changeListID=${ele.changeListID}
            .crs=${ele.crs}
            .test=${ele.grouping}
            .totalDigests=${ele.traces.total_digests || 0}>
        </dots-legend-sk>
      </div>
    `;
  };

  private static paramsetTemplate = (ele: DigestDetailsSk) => {
    if (!ele.digest || !ele.params) {
      return ''; // details might not be loaded yet.
    }

    const titles = [truncateWithEllipses(ele.digest)];
    const paramsets = [ele.params];

    if (ele.right && ele.right.paramset) {
      titles.push(truncateWithEllipses(ele.right.digest));
      paramsets.push(ele.right.paramset);
    }

    return html`
      <paramset-sk .titles=${titles}
                   .paramsets=${paramsets}
                   .highlight=${ele._highlightedParams}>
      </paramset-sk>
    `;
  };

  private grouping: TestName = '';
  private digest: Digest = '';
  private status: Label = 'untriaged';
  private triageHistory: TriageHistory[] = [];
  private params: ParamSet | null = null;
  private traces: TraceGroup | null = null;
  private refDiffs: { [key: string]: SRDiffDigest | null } = {};
  private _changeListID = '';
  private _crs = '';

  private _commits: Commit[] = [];
  // This tracks which ref we are showing on the right. It will default to the closest one, but
  // can be changed with the toggle.
  private _rightRef = '';
  private _overrideRight: SRDiffDigest | null= null;

  private _highlightedParams: { [key: string]: string } = {};

  constructor() {
    super(DigestDetailsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    dialogPolyfill.registerDialog(this.querySelector('dialog.blamelist_dialog')!);
  }

  /**
   * An array of the commits in the tile. Used to compute the blamelist for representing traces.
   */
  get commits(): Commit[] { return this._commits; }

  set commits(arr: Commit[]) {
    this._commits = arr;
    this._render();
  }

  /** SearchResult from which to pull the digest details to show. */
  set details(obj: SearchResult) {
    this.grouping = obj.test || '';
    this.digest = obj.digest || '';
    this.traces = obj.traces || {};
    this.params = obj.paramset;
    this.refDiffs = obj.refDiffs || {};
    this._rightRef = obj.closestRef || '';
    this.status = obj.status || '';
    this.triageHistory = obj.triage_history || [];
    this._render();
  }

  /** The changelist id (or empty string if this is the master branch). */
  get changeListID(): string { return this._changeListID; }

  set changeListID(id: string) {
    this._changeListID = id;
    this._render();
  }

  /** The Code Review System (e.g. "gerrit") if changeListID is set. */
  get crs(): string { return this._crs; }

  set crs(c: string) {
    this._crs = c;
    this._render();
  }

  /**
   * @prop right {Object} Forces the left image to be compared to the given ref.
   */
  get right(): SRDiffDigest | null {
    if (this._overrideRight) {
      return this._overrideRight;
    }
    return this.refDiffs[this._rightRef] || null;
  }

  set right(override: SRDiffDigest | null) {
    this._overrideRight = override;
    this._render();
  }

  private canToggle(): boolean {
    let totalRefs = 0;
    for (const ref of validRefs) {
      if (this.refDiffs[ref]) {
        totalRefs++;
      }
    }
    return totalRefs > 1;
  }

  private clearTraceHighlights() {
    this._highlightedParams = {};
    this._render();
  }

  private closeBlamelistDialog() {
    this.querySelector<HTMLDialogElement>('dialog.blamelist_dialog')?.close();
  }

  private clusterHref() {
    if (!this.grouping || !this.params || !this.params['source_type'] ||
        this.params['source_type'].length === 0) {
      return '';
    }

    const searchCriteria: Partial<SearchCriteria> = {
      corpus: this.params['source_type'][0],
      includePositiveDigests: true,
      includeNegativeDigests: true,
      includeUntriagedDigests: true,
      includeDigestsNotAtHead: true,
    };
    const clusterState: SearchCriteriaHintableObject & {grouping?: TestName} =
        SearchCriteriaToHintableObject(searchCriteria);
    clusterState.grouping = this.grouping;
    return `/cluster?${fromObject(clusterState as HintableObject)}`;
  }

  private hoverOverTrace(e: CustomEvent<TraceID>) {
    // Find the matching trace in details.traces.
    const trace = this.traces?.traces?.find((trace) => trace.label === e.detail);
    this._highlightedParams = trace?.params || {}
    this._render();
  }

  protected _render() {
    super._render();
    // By default, the browser will show this long trace scrolled all the way to the left. This
    // is the oldest traces and typically not helpful, so after we load, we ask the traces to
    // scroll itself to the left, which it will do once (and not repeatedly on each render).
    this.querySelector<DotsSk>('dots-sk')?.autoscroll();
  }

  private showBlamelist(e: CustomEvent<Commit[]>) {
    e.stopPropagation();
    const dialog = this.querySelector<HTMLDialogElement>('dialog.blamelist_dialog')!;
    const blamelist = dialog.querySelector<BlamelistPanelSk>('blamelist-panel-sk')!;
    blamelist.commits = e.detail;
    dialog.showModal();
  }

  private toggleRightRef() {
    if (!this.canToggle()) {
      return;
    }
    let idx = validRefs.indexOf(this._rightRef);
    let newRight = '';
    while (!this.refDiffs[newRight]) {
      idx = (idx + 1) % validRefs.length;
      newRight = validRefs[idx];
    }
    this._rightRef = newRight;
    this._render();
  }

  private triageChangeHandler(e: CustomEvent<LabelOrEmpty>) {
    e.stopPropagation();
    const newStatus = e.detail as Label;
    this.dispatchEvent(
        new CustomEvent<LabelOrEmpty>('triage', { bubbles: true, detail: newStatus }));

    const triageRequest: TriageRequest = {
      testDigestStatus: {
        [this.grouping]: {
          [this.digest]: newStatus
        }
      },
      changelist_id: this.changeListID,
      crs: this.crs,
    };

    sendBeginTask(this);

    fetch('/json/v1/triage', {
      method: 'POST',
      body: JSON.stringify(triageRequest),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then((resp) => {
      if (resp.ok) {
        // Triaging was successful.
        this.status = newStatus;
        this.triageHistory.unshift({
          user: 'me',
          ts: Date.now().toString(),
        });
        this._render();
        sendEndTask(this);
      } else {
        // Triaging did not work (possibly because the user was not logged in). We want to set
        // the status of the triage-sk back to what it was to give a visual indication it did not
        // go through. Additionally, toast error message should catch the user's attention.
        console.error(resp);
        errorMessage(
            `Unexpected error triaging: ${resp.status} ${resp.statusText} ` +
            '(Are you logged in with the right account?)', 8000);
        this.querySelector<TriageSk>('triage-sk')!.value = this.status;
        this._render();
        sendEndTask(this);
      }
    }).catch((e) => {
      sendFetchError(this, e, 'triaging')
    });
  }
}

define('digest-details-sk', DigestDetailsSk);
