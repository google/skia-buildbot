/**
 * @module modules/point-links-sk
 * @description <h2><code>point-links-sk</code></h2>
 * This module provides the ability to display links which are specific to data points.
 * The original data source for the links come from the ingestion file and the caller
 * provides a list of keys to extract from the links and format those as anchor elements
 * to be displayed.
 *
 * This module also generates commit range links for incoming links that are commits. This
 * is done by getting the links for the current commit (the point that is selected) and the
 * previous commit, and then generating a git log url to show the list of commits between
 * both of these.
 *
 * @example
 *
 * Link in ingestion file (commit n): {'V8 Git Hash': 'https://chromium.googlesource.com/v8/v8/+/47f420e89ec1b33dacc048d93e0317ab7fec43dd'}
 * Link in ingestion file (commit n-1): {'V8 Git Hash': 'https://chromium.googlesource.com/v8/v8/+/f052b8c4db1f08d1f8275351c047854e6ff1805f'}
 *
 * Since both the commit links are different, this module will generate a new link like below.
 *
 * V8 Git Hash Range: <a href='https://chromium.googlesource.com/v8/v8/+log/f052b8c4..47f420e>f052b8c4 - 47f420e</a>
 *
 * @example
 * Link in ingestion file (commit n): {'V8 Git Hash': 'https://chromium.googlesource.com/v8/v8/+/47f420e89ec1b33dacc048d93e0317ab7fec43dd'}
 * Link in ingestion file (commit n-1): {'V8 Git Hash': 'https://chromium.googlesource.com/v8/v8/+/47f420e89ec1b33dacc048d93e0317ab7fec43dd'}
 *
 * Since both the commit links are the same, this module will use the link that points to the commit.
 * It will not provide a link to the list view since it'll just be empty.
 *
 * V8 Git Hash Range: <a href='https://chromium.googlesource.com/v8/v8/+/47f420e89ec1b33dacc048d93e0317ab7fec43dd>47f420e - 47f420e</a>
 */
import { TemplateResult, html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CommitDetailsRequest, CommitNumber, ingest } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';

export class PointLinksSk extends ElementSk {
  constructor() {
    super(PointLinksSk.template);
  }

  commitPosition: CommitNumber | null = null;

  // Contains the urls to be displayed.
  displayUrls: { [key: string]: string } = {};

  // Contains the texts for the urls to be displayed.
  displayTexts: { [key: string]: string } = {};

  private static template = (ele: PointLinksSk) =>
    html`<div class="point-links" ?hidden=${Object.keys(ele.displayUrls || {}).length === 0}>
      <ul class="table">
        ${ele.renderLinks()}
        <a href="/u/?rev=${ele.commitPosition}" target="_blank">Regressions</a>
      </ul>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  renderLinks(): TemplateResult[] {
    const keys = Object.keys(this.displayUrls);
    const getHtml = (key: string): TemplateResult => {
      const link = this.displayUrls![key];
      return html` <a href="${link}" target="_blank">${key}</a>`;
    };
    return keys.map(getHtml);
  }

  // load and display the links for the given commit and trace.
  public async load(
    cid: CommitNumber,
    prev_cid: CommitNumber,
    traceid: string,
    keysForCommitRange: string[],
    keysForUsefulLinks: string[]
  ): Promise<void> {
    // Clear any existing links first.
    this.displayUrls = {};
    this.displayTexts = {};
    const currentLinks = await this.getLinksForPoint(cid, traceid);
    if (keysForCommitRange !== null) {
      const prevLinks = await this.getLinksForPoint(prev_cid, traceid);
      keysForCommitRange.forEach((key) => {
        const currentCommitUrl = currentLinks[key];
        if (
          currentCommitUrl !== undefined &&
          currentCommitUrl !== null &&
          currentCommitUrl !== ''
        ) {
          const prevCommitUrl = prevLinks[key];
          const currentCommitId = this.getCommitIdFromCommitUrl(currentCommitUrl).substring(0, 7);
          const prevCommitId = this.getCommitIdFromCommitUrl(prevCommitUrl).substring(0, 7);

          // The links should be different depending on whether the
          // commits are the same. If the commits are the same, simply point to
          // the commit. If they're not, point to the log list.
          if (currentCommitId === prevCommitId) {
            const displayKey = `${key} Commit`;
            this.displayTexts[displayKey] = `${currentCommitId} (No Change)`;
            this.displayUrls[displayKey] = currentCommitUrl;
          } else {
            const displayKey = `${key} Range`;
            this.displayTexts[displayKey] = this.getFormattedCommitRangeText(
              prevCommitId,
              currentCommitId
            );

            const repoUrl = this.getRepoUrlFromCommitUrl(currentCommitUrl);
            const commitRangeUrl = `${repoUrl}+log/${prevCommitId}..${currentCommitId}`;
            this.displayUrls[displayKey] = commitRangeUrl;
          }
        }
      });
    }
    if (keysForUsefulLinks !== null) {
      for (const key in currentLinks) {
        if (keysForUsefulLinks.includes(key)) {
          this.displayTexts[key] = 'Link';
          this.displayUrls[key] = currentLinks[key];
        }
      }
    }
    this._render();
  }

  /** Clear Point Links */
  reset(): void {
    this.displayUrls = {};
    this.displayTexts = {};
    this._render();
  }

  /**
   * Get the commit range text.
   * @param start Start Commit.
   * @param end End Commit.
   * @returns Formatted text.
   */
  private getFormattedCommitRangeText(start: string, end: string): string {
    return `${start} - ${end}`;
  }

  /**
   * Get the repository name from the given commit url.
   * @param commitUrl Full commit url.
   * @returns Repository name.
   */
  private getRepoUrlFromCommitUrl(commitUrl: string): string {
    const idx = commitUrl.indexOf('+');
    return commitUrl.substring(0, idx);
  }

  /**
   * Get the commit id from the given commit url.
   * @param commitUrl Full commit url.
   * @returns Commit id.
   */
  private getCommitIdFromCommitUrl(commitUrl: string): string {
    const idx = commitUrl.lastIndexOf('/');
    return commitUrl.substring(idx + 1);
  }

  /**
   * Get the links for the given commit.
   * @param cid Commit id.
   * @param traceId Trace id.
   * @returns Links relevant to the commit id and trace id.
   */
  private async getLinksForPoint(
    cid: CommitNumber,
    traceId: string
  ): Promise<{ [key: string]: string }> {
    const body: CommitDetailsRequest = {
      cid: cid,
      traceid: traceId,
    };
    const url = '/_/details/?results=false';
    try {
      const resp = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      });
      const json = await jsonOrThrow(resp);
      const format = json as ingest.Format;
      return format.links!;
    } catch (error) {
      await errorMessage(error as string);
    }

    return {};
  }
}

define('point-links-sk', PointLinksSk);
