/**
 * @module modules/commit-range-sk
 * @description <h2><code>commit-range-sk</code></h2>
 *
 * Displays a link that describes a range of commits in a repo. This element
 * uses the global `window.perf.commit_range_url`, which can be set on Perf via
 * the command line.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { lookupCids } from '../cid/cid';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { ColumnHeader, CommitNumber } from '../json';
import '../window/window';

// Converts CommitNumbers to Git hashes.
type commitNumberToHashes = (commitNumbers: CommitNumber[]) => Promise<string[]>;

/** The default implementation for commitNumberToHashes run the commit numbers
 *  through cid lookup to get the hashes by making a request to the server.
 */
const defaultcommitNumberToHashes = async (cids: CommitNumber[]): Promise<string[]> => {
  const json = await lookupCids(cids);
  return [json.commitSlice![0].hash, json.commitSlice![1].hash];
};

export class CommitRangeSk extends ElementSk {
  private _trace: number[] = [];

  private _commitIndex: number = -1;

  private _header: (ColumnHeader | null)[] | null = null;

  private _htmlTemplate = html``;

  private _commitIds: [CommitNumber, CommitNumber] | null = null;

  private _hashes: string[] | null = null;

  private _autoload: boolean = true;

  // commitNumberToHashes can be replaced to make testing easier.
  private commitNumberToHashes: commitNumberToHashes = async (
    cids: CommitNumber[]
  ): Promise<string[]> => {
    if (this._autoload) {
      const hashes = await defaultcommitNumberToHashes(cids);
      return hashes;
    }

    return [];
  };

  constructor() {
    super(CommitRangeSk.template);
  }

  private static template = (ele: CommitRangeSk) => html`${ele.htmlTemplate}`;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('trace');
    this._upgradeProperty('commitIndex');
    this._upgradeProperty('header');
    this._render();
  }

  reset(): void {
    this._commitIndex = -1;
    this._trace = [];
    this._header = null;
    this._htmlTemplate = html``;
    this._commitIds = null;
    this._hashes = null;
    this._render();
  }

  clear(): void {
    this._htmlTemplate = html``;
    this._render();
  }

  /** Check start and end commits and determines if delta is more than 1.
   * If so, it is a range and returns true.
   * If not, it is a single commit and returns false.
   * Returns false if no commits are set.
   * @returns boolean
   */
  isRange(): boolean | null {
    if (!this._commitIds) {
      return null;
    }
    if (this._commitIds[0] >= this._commitIds[1]) {
      return false;
    }

    if (this._commitIds[0] + 1 === this._commitIds[1]) {
      return false;
    }
    return true;
  }

  /**
   * Recalculates the link based on the current state of the object.
   * If there is not enough information to build the link, it clears the
   * current link.
   *
   * Supported URL formats for commit_range_url in k8s-config yaml files:
   * https://<googlesource_repo>/+log/{begin}..{end}
   * https://<github_repo>/commits/{end}
   *
   * GitHub does not support ranges in the URL.
   * When only one commit is being referenced, the {begin}.. is removed
   * from the URL.
   */
  async recalcLink(): Promise<void> {
    if (window.perf.commit_range_url === '' || this._commitIndex === -1) {
      this.clear();
      return;
    }

    const newCommitIds = this.setCommitIds(this._commitIndex);
    if (!newCommitIds || newCommitIds.length !== 2) {
      this.clear();
      return;
    }

    // If the commit IDs have changed then the hashes are no longer valid.
    if (
      !this._commitIds ||
      this._commitIds[0] !== newCommitIds[0] ||
      this._commitIds[1] !== newCommitIds[1]
    ) {
      this._hashes = null;
    }
    this._commitIds = newCommitIds;

    try {
      let text = `${this._commitIds[1]}`;
      // Check if there are no points between start and end.
      const isRange = this.isRange();

      if (isRange) {
        // Add +1 to the previous commit to only show commits after previous.
        text = `${this._commitIds[0] + 1} - ${this._commitIds[1]}`;
      }
      this.htmlTemplate = html`${text}`;

      if (!this.hashes) {
        // Run the commit numbers through cid lookup to get the hashes.
        this.hashes = await this.commitNumberToHashes(this._commitIds);
      }

      // If we have the hashes, then we can build the link.
      if (this.hashes && this.hashes.length > 1) {
        let url = window.perf.commit_range_url;

        // Always replace {end} with the second hash.
        if (url.includes('{end}')) {
          url = url.replace('{end}', this.hashes[1]);
        }
        if (isRange) {
          // Handle range URLs (Googlesource)
          if (url.includes('{begin}')) {
            url = url.replace('{begin}', this.hashes[0]);
          }
        } else {
          // Handle single commit scenarios
          if (url.includes('+log/{begin}..')) {
            // Googlesource style: transform to single commit view
            url = url.replace('+log/{begin}..', '+/');
          } else {
            // Fallback for any other template, remove {begin} if it exists
            if (url.includes('{begin}')) {
              url = url.replace('{begin}', '');
            }
            // If GitHub, show short hash instead of commit number.
            if (url.includes('github')) {
              text = this.hashes[1].substring(0, 7);
            }
          }
        }
        this.htmlTemplate = html`<a href="${url}" target="_blank">${text}</a>`;
        // Ensure element is connected to tooltip before dispatching event.
        if (this._connected) {
          this.dispatchEvent(
            new CustomEvent('commit-range-changed', {
              bubbles: true, // Allows parent elements to catch the event.
              composed: true, // Allows event to cross shadow DOM boundaries.
            })
          );
        }
      }
    } catch (error) {
      console.log(error);
      this.clear();
    }
  }

  /** A single trace. */
  get trace(): number[] {
    return this._trace;
  }

  set trace(val: number[]) {
    this._trace = val;
    this.recalcLink();
  }

  /** An index into trace, the location of the commit being referenced. */
  get commitIndex(): number {
    return this._commitIndex;
  }

  set commitIndex(val: number) {
    this._commitIndex = val;
    this.recalcLink();
  }

  /** The ColumnHeader of the DataFrame that contains the trace. */
  get header(): (ColumnHeader | null)[] | null {
    return this._header;
  }

  set header(val: (ColumnHeader | null)[] | null) {
    this._header = val;
    this.recalcLink();
  }

  /** The hashes of the commits. */
  get hashes(): string[] | null {
    return this._hashes;
  }

  set hashes(val: string[] | null) {
    if (val !== null && val.length > 1) {
      // Only set the hashes if there are two hashes.
      this._hashes = val;
      this.recalcLink();
    }
  }

  set htmlTemplate(val: any) {
    // If the template is the same, don't re-render.
    if (this._htmlTemplate !== val) {
      this._htmlTemplate = val;
      this._render();
    }
  }

  get htmlTemplate() {
    return this._htmlTemplate;
  }

  set autoload(val: boolean) {
    this._autoload = val;
  }

  setCommitIds(commitIndex: number): [CommitNumber, CommitNumber] | null {
    if (this._trace.length === 0 || this._header === null) {
      this.clear();
      return null;
    }
    // First the previous commit that has data.
    let prevCommit = commitIndex - 1;

    while (prevCommit > 0 && this._trace[prevCommit] === MISSING_DATA_SENTINEL) {
      prevCommit -= 1;
    }

    // If we don't find a second commit then we can't present the information.
    if (prevCommit < 0) {
      this.clear();
      return null;
    }

    const startOffset = this._header[prevCommit]?.offset ?? null;
    const endOffset = this._header[commitIndex]?.offset ?? null;
    if (startOffset === null || endOffset === null) {
      this.clear();
      return null;
    }
    return [startOffset, endOffset];
  }
}

define('commit-range-sk', CommitRangeSk);
