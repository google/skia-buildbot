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
import { $$ } from 'common-sk/modules/dom';
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
import '../triage-history-sk';
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
        <span class=grouping_name>Test: ${ele._grouping}</span>
        <span class=expand></span>
        <a href=${ele._clusterHref()} target=_blank rel=noopener class=cluster_link>
          <group-work-icon-sk title="Cluster view of this digest and all others for this test.">
          </group-work-icon-sk>
        </a>
      </div>
    
      <div class=comparison>
        <div class=digest_labels>
          <span class=digest_label>Left: ${ele._digest}</span>
          <span class=expand></span>
          <span class=digest_label ?hidden=${!ele.right}>Right: ${ele.right && ele.right.digest}</span>
        </div>
        <div class=comparison_data>
          <div>${DigestDetailsSk.detailsAndTriage(ele)}</div>
          <div>${DigestDetailsSk.imageComparison(ele)}</div>
          <div>
            <button @click=${ele._toggleRightRef} ?disabled=${!ele._canToggle()} class=toggle_ref
               ?hidden=${ele._overrideRight || !ele.right} title=${toggleButtonMouseover(ele._canToggle())}>
               Toggle Reference
            </button>
            <div ?hidden=${!ele.right || ele.right.status !== 'negative'} class=negative_warning>
              Closest image is negative!
            </div>
            <!-- TODO(kjlubick) Comments would go here -->
          </div>
        </div>
      </div>
    ${DigestDetailsSk.traceInfo(ele)}
    ${DigestDetailsSk.paramset(ele)}
    </div>
    <dialog class=blamelist_dialog>
      <blamelist-panel-sk></blamelist-panel-sk>
      <button class=close_btn @click=${ele._closeBlamelistDialog}>Close</button>
    </dialog>
  `;

  private static detailsAndTriage = (ele: DigestDetailsSk) => {
    if (!ele.right) {
      return html`
        <div class=metrics_and_triage>
          <triage-sk @change=${ele._triageChangeHandler} .value=${ele._status}></triage-sk>
          <triage-history-sk .history=${ele._triageHistory}></triage-history-sk>
        </div>
      `;
    }

    // TODO(kjlubick) would it be clearer to just tell the user the images differ in size and omit
    //  the (probably useless metrics)? Could we also include the actual dimensions of the two?

    return html`
      <div class=metrics_and_triage>
        <div>
          <a href=${diffPageHref(ele._grouping, ele._digest, ele.right.digest, ele.changeListID, ele.crs)}
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
        <triage-sk @change=${ele._triageChangeHandler} .value=${ele._status}></triage-sk>
        <triage-history-sk .history=${ele._triageHistory}></triage-history-sk>
      </div>
    `;
  };

  private static imageComparison = (ele: DigestDetailsSk) => {
    const left: ImageComparisonData = {
      digest: ele._digest,
      title: truncateWithEllipses(ele._digest),
      detail: detailHref(ele._grouping, ele._digest, ele.changeListID, ele.crs),
    };
    if (!ele.right) {
      const hasOtherDigests = (ele._traces?.digests?.length || 0) > 1;
      return html`<image-compare-sk .left=${left}
        .isComputingDiffs=${hasOtherDigests}></image-compare-sk>`;
    }

    const right: ImageComparisonData = {
      digest: ele.right.digest,
      title: ele.right.status === 'positive' ? 'Closest Positive' : 'Closest Negative',
      detail: detailHref(ele._grouping, ele.right.digest, ele.changeListID, ele.crs),
    };
    if (ele._overrideRight) {
      right.title = truncateWithEllipses(ele.right.digest);
    }

    return html`<image-compare-sk .left=${left} .right=${right}></image-compare-sk>`;
  };

  private static traceInfo = (ele: DigestDetailsSk) => {
    if (!ele._traces || !ele._traces.traces || !ele._traces.traces.length) {
      return '';
    }
    return html`
      <div class=trace_info>
        <dots-sk .value=${ele._traces} .commits=${ele._commits} @hover=${ele._hoverOverTrace}
            @mouseleave=${ele._clearTraceHighlights} @showblamelist=${ele._showBlamelist}></dots-sk>
        <dots-legend-sk .digests=${ele._traces.digests} .changeListID=${ele.changeListID} .crs=${ele.crs}
           .test=${ele._grouping} .totalDigests=${ele._traces.total_digests || 0}></dots-legend-sk>
      </div>
    `;
  };

  private static paramset = (ele: DigestDetailsSk) => {
    if (!ele._digest || !ele._params) {
      return ''; // details might not be loaded yet.
    }

    const titles = [truncateWithEllipses(ele._digest)];
    const paramsets = [ele._params];

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

  private _grouping: TestName = '';
  private _digest: Digest = '';
  private _status: Label = 'untriaged';
  private _triageHistory: TriageHistory[] = [];
  private _params: ParamSet | null = null;
  private _traces: TraceGroup | null = null;
  private _refDiffs: { [key: string]: SRDiffDigest | null } = {};
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
    this._grouping = obj.test || '';
    this._digest = obj.digest || '';
    this._traces = obj.traces || {};
    this._params = obj.paramset;
    this._refDiffs = obj.refDiffs || {};
    this._rightRef = obj.closestRef || '';
    this._status = obj.status || '';
    this._triageHistory = obj.triage_history || [];
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
    return this._refDiffs[this._rightRef] || null;
  }

  set right(override: SRDiffDigest | null) {
    this._overrideRight = override;
    this._render();
  }

  private _canToggle(): boolean {
    let totalRefs = 0;
    for (const ref of validRefs) {
      if (this._refDiffs[ref]) {
        totalRefs++;
      }
    }
    return totalRefs > 1;
  }

  private _clearTraceHighlights() {
    this._highlightedParams = {};
    this._render();
  }

  private _closeBlamelistDialog() {
    this.querySelector<HTMLDialogElement>('dialog.blamelist_dialog')?.close();
  }

  private _clusterHref() {
    if (!this._grouping || !this._params || !this._params['source_type'] ||
        this._params['source_type'].length === 0) {
      return '';
    }

    const searchCriteria: Partial<SearchCriteria> = {
      corpus: this._params['source_type'][0],
      includePositiveDigests: true,
      includeNegativeDigests: true,
      includeUntriagedDigests: true,
      includeDigestsNotAtHead: true,
    };
    const clusterState: SearchCriteriaHintableObject & {grouping?: TestName} = SearchCriteriaToHintableObject(searchCriteria);
    clusterState.grouping = this._grouping;
    return `/cluster?${fromObject(clusterState as HintableObject)}`;
  }

  private _hoverOverTrace(e: CustomEvent<TraceID>) {
    // Find the matching trace in details.traces.
    const trace = this._traces?.traces?.find((trace) => trace.label === e.detail);
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

  private _showBlamelist(e: CustomEvent<Commit[]>) {
    e.stopPropagation();
    const dialog = this.querySelector<HTMLDialogElement>('dialog.blamelist_dialog')!;
    const blamelist = dialog.querySelector<BlamelistPanelSk>('blamelist-panel-sk')!;
    blamelist.commits = e.detail;
    dialog.showModal();
  }

  private _toggleRightRef() {
    if (!this._canToggle()) {
      return;
    }
    let idx = validRefs.indexOf(this._rightRef);
    let newRight = '';
    while (!this._refDiffs[newRight]) {
      idx = (idx + 1) % validRefs.length;
      newRight = validRefs[idx];
    }
    this._rightRef = newRight;
    this._render();
  }

  private _triageChangeHandler(e: CustomEvent<LabelOrEmpty>) {
    e.stopPropagation();
    const label = e.detail;
    this.dispatchEvent(new CustomEvent<LabelOrEmpty>('triage', { bubbles: true, detail: label }));
    this.triggerTriage(label as Label);
  }

  /** Triages the given digest with the new status. */
  triggerTriage(newStatus: Label) {
    const triageRequest: TriageRequest = {
      testDigestStatus: {
        [this._grouping]: {
          [this._digest]: newStatus
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
        this._status = newStatus;
        this._triageHistory.unshift({
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
        this.querySelector<TriageSk>('triage-sk')!.value = this._status;
        this._render();
        sendEndTask(this);
      }
    }).catch((e) => {
      sendFetchError(this, e, 'triaging')
    });
  }
}

define('digest-details-sk', DigestDetailsSk);
