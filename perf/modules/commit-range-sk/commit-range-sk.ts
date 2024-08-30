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
type commitNumberToHashes = (
  commitNumbers: CommitNumber[]
) => Promise<string[]>;

/** The default implementation for commitNumberToHashes run the commit numbers
 *  through cid lookup to get the hashes by making a request to the server.
 */
const defaultcommitNumberToHashes = async (
  cids: CommitNumber[]
): Promise<string[]> => {
  const json = await lookupCids(cids);
  return [json.commitSlice![0].hash, json.commitSlice![1].hash];
};

export class CommitRangeSk extends ElementSk {
  private _trace: number[] = [];

  private _commitIndex: number = -1;

  private _header: (ColumnHeader | null)[] | null = null;

  /** The calculated URL. */
  private _url: string = '';

  /** The link text to display. */
  private _text: string = '';

  // commitNumberToHashes can be replaced to make testing easier.
  private commitNumberToHashes: commitNumberToHashes =
    defaultcommitNumberToHashes;

  constructor() {
    super(CommitRangeSk.template);
  }

  private static template = (ele: CommitRangeSk) =>
    html`<a href="${ele._url}" target="_blank">${ele._text}</a>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('trace');
    this._upgradeProperty('commitIndex');
    this._upgradeProperty('header');
    this._render();
  }

  clear(): void {
    this._url = '';
    this._text = '';
    this._render();
  }

  async recalcLink(): Promise<void> {
    if (
      window.perf.commit_range_url === '' ||
      this._commitIndex === -1 ||
      this._trace.length === 0 ||
      this._header === null
    ) {
      this.clear();
      return;
    }
    // First the previous commit that has data.
    let prevCommit = this._commitIndex - 1;

    while (
      prevCommit > 0 &&
      this._trace[prevCommit] === MISSING_DATA_SENTINEL
    ) {
      prevCommit -= 1;
    }

    // If we don't find a second commit then we can't present the information.
    if (prevCommit < 0) {
      this.clear();
      return;
    }
    const cids: CommitNumber[] = [
      this._header![prevCommit]!.offset,
      this._header![this._commitIndex]!.offset,
    ];

    try {
      // Run the commit numbers through cid lookup to get the hashes.
      const hashes = await this.commitNumberToHashes(cids);
      // Create the URL.
      let url = window.perf.commit_range_url;
      url = url.replace('{begin}', hashes[0]);
      url = url.replace('{end}', hashes[1]);

      // Now populate link, including text and url.
      this._url = url;
      this._text = `Commits At Step (${cids[0]} - ${cids[1]})`;
      this._render();
    } catch (error) {
      console.log(error);
      this.clear();
    }
  }

  /** A single trace. */
  set trace(val: number[]) {
    this._trace = val;
    this.recalcLink();
  }

  /** An index into trace, the location of the commit being referenced. */
  set commitIndex(val: number) {
    this._commitIndex = val;
    this.recalcLink();
  }

  /** The ColumnHeader of the DataFrame that contains the trace. */
  set header(val: (ColumnHeader | null)[] | null) {
    this._header = val;
    this.recalcLink();
  }
}

define('commit-range-sk', CommitRangeSk);
