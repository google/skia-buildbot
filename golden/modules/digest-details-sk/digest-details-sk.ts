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
import { HintableObject } from 'common-sk/modules/hintable';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { truncate } from '../../../infra-sk/modules/string';
import {
  detailHref, diffPageHref, sendBeginTask, sendEndTask, sendFetchError,
} from '../common';

import 'elements-sk/icon/group-work-icon-sk';
import '../dots-sk';
import '../dots-legend-sk';
import '../triage-sk';
import '../image-compare-sk';
import '../blamelist-panel-sk';
import '../../../infra-sk/modules/paramset-sk';
import { SearchCriteriaToHintableObject } from '../search-controls-sk';
import {
  Commit, GroupingsResponse, Label, Params, RefClosest, SearchResult, SRDiffDigest, TestName, TraceID, TriageRequestV3, TriageResponse, TriageResponseStatus,
} from '../rpc_types';
import { SearchCriteria, SearchCriteriaHintableObject } from '../search-controls-sk/search-controls-sk';
import { DotsSk } from '../dots-sk/dots-sk';
import { BlamelistPanelSk } from '../blamelist-panel-sk/blamelist-panel-sk';
import { TriageSk } from '../triage-sk/triage-sk';
import { ImageCompareSk, ImageComparisonData } from '../image-compare-sk/image-compare-sk';

function toggleButtonMouseover(canToggle: boolean) {
  if (canToggle) {
    return 'By default, Gold shows the closest image, whether it has been marked positive or '
    + 'negative. This button allows you to explicitly select the closest positive or negative.';
  }
  return 'There are no other reference image types to compare against.';
}

const validRefs: RefClosest[] = ['pos', 'neg'];

export class DigestDetailsSk extends ElementSk {
  private static template = (ele: DigestDetailsSk) => html`
    <div class=container>
      <div class=top_bar>
        <span class=grouping_name>Test: ${ele._details.test}</span>
        <span class=expand></span>
        <a href=${ele.clusterHref()} target=_blank rel=noopener class=cluster_link>
          <group-work-icon-sk title="Cluster view of this digest and all others for this test.">
          </group-work-icon-sk>
        </a>
      </div>

      <div class=comparison>
        <div class=digest_labels>
          <span class="digest_label left">Left: ${ele._details.digest}</span>
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
                ?hidden=${ele.overrideRight || !ele.right}
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
          <triage-sk @change=${ele.triageChangeHandler} .value=${ele._details.status}></triage-sk>
          ${DigestDetailsSk.triageHistoryTemplate(ele)}
      </div>
      `;
    }

    // TODO(kjlubick) would it be clearer to just tell the user the images differ in size and omit
    //  the (probably useless metrics)? Could we also include the actual dimensions of the two?

    // We need the digest's grouping to compute a link to the diff page. If the digest has no
    // params, we'll get an exception when computing the grouping. If we don't catch the exception,
    // this whole element will fail to render.
    let maybeGrouping: Params | null = null;
    try {
      maybeGrouping = ele.getGrouping();
    } catch {
      // Nothing to do.
    }

    // Only show a link to the diff page if we successfully computed the digest's grouping.
    const diffPageLinkTemplate = maybeGrouping
      ? html`
        <div>
          <a href=${diffPageHref(
        ele.getGrouping(),
        ele._details.digest,
        ele.right.digest,
        ele._changeListID,
        ele._crs,
      )}
            target=_blank rel=noopener class=diffpage_link>
            Diff Details
          </a>
        </div>
      `
      : '';

    return html`
      <div class=metrics_and_triage>
        ${diffPageLinkTemplate}
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
        <triage-sk @change=${ele.triageChangeHandler} .value=${ele._details.status}></triage-sk>
        ${DigestDetailsSk.triageHistoryTemplate(ele)}
      </div>
    `;
  };

  private static triageHistoryTemplate = (ele: DigestDetailsSk) => {
    if (!ele._details.triage_history || ele._details.triage_history.length === 0) return '';

    const mostRecent = ele._details.triage_history![0];
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
    // We need the digest's grouping to compute a link to the details page. If the digest has no
    // params, we'll get an exception when computing the grouping. If we don't catch the exception,
    // this whole element will fail to render.
    let maybeGrouping: Params | null = null;
    try {
      maybeGrouping = ele.getGrouping();
    } catch {
      // Nothing to do.
    }

    const left: ImageComparisonData = {
      digest: ele._details.digest,
      title: truncate(ele._details.digest, 15),
      detail: maybeGrouping
        ? detailHref(maybeGrouping, ele._details.digest, ele._changeListID, ele._crs)
        : '',
    };
    if (!ele.right) {
      const hasOtherDigests = (ele._details.traces?.digests?.length || 0) > 1;
      return html`
        <image-compare-sk .left=${left}
                          .isComputingDiffs=${hasOtherDigests}
                          .fullSizeImages=${ele._fullSizeImages}>
        </image-compare-sk>
      `;
    }

    const right: ImageComparisonData = {
      digest: ele.right.digest,
      title: ele.right.status === 'positive' ? 'Closest Positive' : 'Closest Negative',
      detail: maybeGrouping
        ? detailHref(maybeGrouping, ele.right.digest, ele._changeListID, ele._crs)
        : '',
    };
    if (ele.overrideRight) {
      right.title = truncate(ele.right.digest, 15);
    }

    return html`
      <image-compare-sk .left=${left}
                        .right=${right}
                        .fullSizeImages=${ele._fullSizeImages}>
      </image-compare-sk>
    `;
  };

  private static traceInfoTemplate = (ele: DigestDetailsSk) => {
    if (!ele._details.traces || !ele._details.traces.traces || !ele._details.traces.traces.length) {
      return '';
    }
    return html`
      <div class=trace_info>
        <dots-sk
            .value=${ele._details.traces}
            .commits=${ele._commits}
            @hover=${ele.hoverOverTrace}
            @mouseleave=${ele.clearTraceHighlights}
            @showblamelist=${ele.showBlamelist}>
        </dots-sk>
        <dots-legend-sk
            .grouping=${ele.getGrouping()}
            .digests=${ele._details.traces.digests}
            .changeListID=${ele._changeListID}
            .crs=${ele._crs}
            .totalDigests=${ele._details.traces.total_digests || 0}>
        </dots-legend-sk>
      </div>
    `;
  };

  private static paramsetTemplate = (ele: DigestDetailsSk) => {
    if (!ele._details.digest || !ele._details.paramset) {
      return ''; // details might not be loaded yet.
    }

    const titles = [truncate(ele._details.digest, 15)];
    const paramsets = [ele._details.paramset];

    if (ele.right && ele.right.paramset) {
      titles.push(truncate(ele.right.digest, 15));
      paramsets.push(ele.right.paramset);
    }

    return html`
      <paramset-sk .titles=${titles}
                   .paramsets=${paramsets}
                   .highlight=${ele.highlightedParams}>
      </paramset-sk>
    `;
  };

  private _details: SearchResult = {
    digest: '',
    test: '',
    status: 'untriaged',
    triage_history: null,
    paramset: {},
    traces: {
      traces: null,
      digests: null,
      total_digests: 0,
    },
    refDiffs: null,
    closestRef: '',
  };

  private _changeListID = '';

  private _crs = '';

  private _groupings: GroupingsResponse = {
    grouping_param_keys_by_corpus: {},
  };

  private _commits: Commit[] = [];

  // This tracks which ref we are showing on the right. It will default to the closest one, but
  // can be changed with the toggle.
  private rightRef: RefClosest = '';

  private overrideRight: SRDiffDigest | null= null;

  private highlightedParams: { [key: string]: string } = {};

  private _fullSizeImages = false;

  constructor() {
    super(DigestDetailsSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    dialogPolyfill.registerDialog(this.querySelector('dialog.blamelist_dialog')!);
  }

  /** GroupingsResponse used to derive the correct grouping to use when triaging. */
  set groupings(groupings: GroupingsResponse) {
    this._groupings = groupings;
  }

  /**
   * An array of the commits in the tile. Used to compute the blamelist for representing traces.
   */
  set commits(arr: Commit[]) {
    this._commits = arr;
    this._render();
  }

  /** SearchResult from which to pull the digest details to show. */
  set details(details: SearchResult) {
    this._details = details;
    this.rightRef = details.closestRef;
    this._render();
  }

  /** The changelist id (or empty string if this is the master branch). */
  set changeListID(id: string) {
    this._changeListID = id;
    this._render();
  }

  /** The Code Review System (e.g. "gerrit") if changeListID is set. */
  set crs(c: string) {
    this._crs = c;
    this._render();
  }

  /** Forces the left image to be compared to the given ref. */
  get right(): SRDiffDigest | null {
    if (this.overrideRight) {
      return this.overrideRight;
    }
    return this._details.refDiffs ? this._details.refDiffs[this.rightRef] : null;
  }

  set right(override: SRDiffDigest | null) {
    this.overrideRight = override;
    this._render();
  }

  /** Whether to show thumbnails or full size images. */
  set fullSizeImages(val: boolean) {
    this._fullSizeImages = val;
    this._render();
  }

  private canToggle(): boolean {
    let totalRefs = 0;
    for (const ref of validRefs) {
      if (this._details.refDiffs && this._details.refDiffs[ref]) {
        totalRefs++;
      }
    }
    return totalRefs > 1;
  }

  private clearTraceHighlights() {
    this.highlightedParams = {};
    this._render();
  }

  private closeBlamelistDialog() {
    this.querySelector<HTMLDialogElement>('dialog.blamelist_dialog')?.close();
  }

  private clusterHref() {
    if (!this._details.test || !this._details.paramset || !this._details.paramset.source_type
        || this._details.paramset.source_type.length === 0) {
      return '';
    }

    const searchCriteria: Partial<SearchCriteria> = {
      corpus: this._details.paramset.source_type[0],
      includePositiveDigests: true,
      includeNegativeDigests: true,
      includeUntriagedDigests: true,
      includeDigestsNotAtHead: true,
    };
    const clusterState: SearchCriteriaHintableObject & {grouping?: TestName} = SearchCriteriaToHintableObject(searchCriteria);
    clusterState.grouping = this._details.test;
    return `/cluster?${fromObject(clusterState as HintableObject)}`;
  }

  private hoverOverTrace(e: CustomEvent<TraceID>) {
    // Find the matching trace in details.traces.
    const trace = this._details.traces?.traces?.find((trace) => trace.label === e.detail);
    this.highlightedParams = trace?.params || {};
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
    let idx = validRefs.indexOf(this.rightRef);
    let newRight: RefClosest = '';
    while (!this._details.refDiffs![newRight]) {
      idx = (idx + 1) % validRefs.length;
      newRight = validRefs[idx];
    }
    this.rightRef = newRight;
    this._render();
  }

  private triageChangeHandler(e: CustomEvent<Label>) {
    e.stopPropagation();
    const newLabel = e.detail;
    this.setTriaged(newLabel);
  }

  private getGrouping(): Params {
    // Extract corpus.
    const corpusKey = 'source_type';
    if (!this._details.paramset[corpusKey]) {
      throw new Error(`Digest is missing key "${corpusKey}".`);
    }
    if (this._details.paramset[corpusKey].length !== 1) {
      throw new Error(
        `Digest key "${corpusKey}" must have exactly one value;`
        + `${this._details.paramset[corpusKey].length} values found.`,
      );
    }
    const corpus = this._details.paramset[corpusKey][0];

    // Build grouping.
    const grouping: Params = {};
    const groupingKeys = this._groupings.grouping_param_keys_by_corpus![corpus];
    groupingKeys?.forEach((key) => {
      if (!this._details.paramset[key]) {
        throw new Error(`Digest is missing key "${key}"`);
      }
      if (this._details.paramset[key].length !== 1) {
        throw new Error(
          `Digest key ${key} must have exactly one value;`
          + `${this._details.paramset[key].length} values found.`,
        );
      }
      grouping[key] = this._details.paramset[key][0];
    });

    return grouping;
  }

  setTriaged(label: Label): void {
    // We save the label before the triage action because the search page might change the label
    // when it handles the "triage" event, see
    // https://skia.googlesource.com/buildbot/+/6cfe69ae17a74c87224196b6e170dad01bad558a/golden/modules/search-page-sk/search-page-sk.ts#512.
    const labelBefore = this._details.status;

    this.dispatchEvent(
      new CustomEvent<Label>('triage', { bubbles: true, detail: label }),
    );

    let grouping: Params;
    try {
      grouping = this.getGrouping();
    } catch (e) {
      if (e instanceof Error) {
        errorMessage(e.message);
        return;
      }
      throw e;
    }

    const triageRequest: TriageRequestV3 = {
      deltas: [
        {
          grouping: grouping,
          digest: this._details.digest,
          label_before: labelBefore,
          label_after: label,
        },
      ],
    };
    if (this._changeListID && this._crs) {
      triageRequest.changelist_id = this._changeListID;
      triageRequest.crs = this._crs;
    }

    const restorePreviousStatusInUI = () => {
      this.querySelector<TriageSk>('triage-sk')!.value = labelBefore;
      this._render();
    };

    sendBeginTask(this);
    const url = '/json/v3/triage';
    fetch(url, {
      method: 'POST',
      body: JSON.stringify(triageRequest),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(async (resp: Response) => {
      if (resp.ok) {
        const triageResponse = await resp.json() as TriageResponse;
        if (triageResponse.status === 'ok') {
          // Triaging was successful.
          this._details.status = label;
          this._details.triage_history ||= [];
          this._details.triage_history!.unshift({
            user: 'me',
            ts: new Date(Date.now()).toISOString(),
          });
          this._render();
          sendEndTask(this);
        } else if (triageResponse.status === 'conflict') {
          // Triage conflict. We want to set the status of the triage-sk back to what it was to
          // give a visual indication it did not go through. Additionally, toast error message
          // should catch the user's attention.
          console.error('TriageResponse indicates triage conflict:', triageResponse);
          errorMessage(
            'Triage conflict: Attempted to triage from '
            + `${triageResponse.conflict?.actual_label_before} to ${label}, `
            + 'but the digest\'s current label is '
            + `${triageResponse.conflict?.expected_label_before}. `
            + 'It is possible that another user triaged this digest. Try refreshing the page.',
          );
          restorePreviousStatusInUI();
          sendEndTask(this);
        } else {
          // Unknown triage status. We want to set the status of the triage-sk back to what it was
          // to give a visual indication it did not go through. Additionally, toast error message
          // should catch the user's attention.
          console.error('Unknown TriageResponse status:', triageResponse);
          errorMessage(`Unexpected TriageResponse status: ${triageResponse.status}.`, 8000);
          restorePreviousStatusInUI();
          sendEndTask(this);
        }
      } else {
        // Triaging did not work (possibly because the user was not logged in). We want to set
        // the status of the triage-sk back to what it was to give a visual indication it did not
        // go through. Additionally, toast error message should catch the user's attention.
        console.error(resp);
        errorMessage(
          `Unexpected error triaging: ${resp.status} ${resp.statusText} `
            + '(Are you logged in with the right account?)', 8000,
        );
        restorePreviousStatusInUI();
        sendEndTask(this);
      }
    }).catch((e) => {
      sendFetchError(this, e, 'triaging');
    });
  }

  openZoom(): void {
    const compare = this.querySelector<ImageCompareSk>('image-compare-sk');
    if (!compare) {
      return;
    }
    compare.openZoomWindow();
  }
}

define('digest-details-sk', DigestDetailsSk);
