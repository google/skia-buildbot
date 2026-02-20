/**
 * @module modules/commit-range-sk
 * @description <h2><code>commit-range-sk</code></h2>
 *
 * Displays a link that describes a range of commits in a repo. This element
 * uses the global `window.perf.commit_range_url`, which can be set on Perf via
 * the command line.
 */
import { html, LitElement, PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
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

@customElement('commit-range-sk')
export class CommitRangeSk extends LitElement {
  @property({ attribute: false })
  trace: number[] = [];

  @property({ type: Number, attribute: 'commit-index' })
  commitIndex: number = -1;

  @property({ attribute: false })
  header: (ColumnHeader | null)[] | null = null;

  @property({ attribute: false })
  hashes: string[] | null = null;

  @state()
  private _text: string = '';

  @state()
  private _url: string = '';

  private _autoload: boolean = true;

  private _commitIds: [CommitNumber, CommitNumber] | null = null;

  private currentRequestId: number = 0;

  private hashCache: Map<string, string[]> = new Map();

  // commitNumberToHashes can be replaced to make testing easier.
  private commitNumberToHashes: commitNumberToHashes = async (
    cids: CommitNumber[]
  ): Promise<string[]> => {
    const cacheKey = cids.join(',');
    if (this.hashCache.has(cacheKey)) {
      return this.hashCache.get(cacheKey)!;
    }

    let hashes: string[] = [];
    if (this._autoload) {
      hashes = await defaultcommitNumberToHashes(cids);
      this.hashCache.set(cacheKey, hashes);
      return hashes;
    }

    return [];
  };

  createRenderRoot() {
    return this;
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (
      changedProperties.has('trace') ||
      changedProperties.has('commitIndex') ||
      changedProperties.has('header') ||
      changedProperties.has('hashes')
    ) {
      this.recalcLink(changedProperties);
    }
  }

  reset(): void {
    this.commitIndex = -1;
    this.trace = [];
    this.header = null;
    this.hashes = null;
    this._text = '';
    this._url = '';
    this._commitIds = null;
  }

  clear(): void {
    this._text = '';
    this._url = '';
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
   * Sets the range explicitly.
   * @param start The previous commit (exclusive).
   * @param end The current commit (inclusive).
   */
  async setRange(start: CommitNumber, end: CommitNumber): Promise<void> {
    this._commitIds = [start, end];
    this.hashes = null;
    this.updateText();
    await this.recalcLink();
  }

  /**
   * Recalculates the link based on the current state of the object.
   * If there is not enough information to build the link, it clears the
   * current link.
   */
  async recalcLink(changedProperties?: PropertyValues): Promise<void> {
    this.currentRequestId++;
    const requestId = this.currentRequestId;

    if (window.perf.commit_range_url === '') {
      this.clear();
      return;
    }

    if (this.commitIndex !== -1) {
      const newCommitIds = this.setCommitIds(this.commitIndex);
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
        if (!changedProperties?.has('hashes')) {
          this.hashes = null;
        }
      }
      this._commitIds = newCommitIds;
    } else if (!this._commitIds) {
      // If commitIndex is -1 and no manual range set, clear.
      this.clear();
      return;
    }

    try {
      if (!this._commitIds || this._commitIds.length !== 2) {
        this.clear();
        return;
      }

      this.updateText();
      // Clear URL while fetching new hashes or if irrelevant
      this._url = '';

      if (!this.hashes) {
        // Run the commit numbers through cid lookup to get the hashes.
        const hashes = await this.commitNumberToHashes(this._commitIds);
        if (requestId !== this.currentRequestId) {
          return;
        }
        this.hashes = hashes;
      }

      // If we have the hashes, then we can build the link.
      if (this.hashes && this.hashes.length > 1) {
        let url = window.perf.commit_range_url;

        // Always replace {end} with the second hash.
        if (url.includes('{end}')) {
          url = url.replace('{end}', this.hashes[1]);
        }
        const isRange = this.isRange();
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
              this._text = this.hashes[1].substring(0, 7);
            }
          }
        }

        if (requestId !== this.currentRequestId) {
          return;
        }

        this._url = url;

        // Ensure element is connected to tooltip before dispatching event.
        if (this.isConnected) {
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

  private updateText(): void {
    if (!this._commitIds || this._commitIds.length !== 2) {
      return;
    }
    let text = `${this._commitIds[1]}`;
    // Check if there are no points between start and end.
    const isRange = this.isRange();

    if (isRange) {
      // Add +1 to the previous commit to only show commits after previous.
      text = `${this._commitIds[0] + 1} - ${this._commitIds[1]}`;
    }
    this._text = text;
  }

  set autoload(val: boolean) {
    this._autoload = val;
  }

  setCommitIds(commitIndex: number): [CommitNumber, CommitNumber] | null {
    if (this.trace.length === 0 || this.header === null) {
      this.clear();
      return null;
    }
    // First the previous commit that has data.
    let prevCommit = commitIndex - 1;

    while (prevCommit > 0 && this.trace[prevCommit] === MISSING_DATA_SENTINEL) {
      prevCommit -= 1;
    }

    // If we don't find a second commit then we can't present the information.
    if (prevCommit < 0) {
      this.clear();
      return null;
    }

    const startOffset = this.header[prevCommit]?.offset ?? null;
    const endOffset = this.header[commitIndex]?.offset ?? null;
    if (startOffset === null || endOffset === null) {
      this.clear();
      return null;
    }
    return [startOffset, endOffset];
  }

  render() {
    if (this._url) {
      return html`<a href="${this._url}" target="_blank">${this._text}</a>`;
    }
    return html`<span style="cursor: default">${this._text}</span>`;
  }
}
