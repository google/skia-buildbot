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
 * <a href='https://chromium.googlesource.com/v8/v8/+log/f052b8c4..47f420e>V8 Git Hash Range</a>
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

  // Contains the urls to be displayed.
  displayUrls: { [key: string]: string } = {};

  // Contains the texts for the urls to be displayed.
  displayTexts: { [key: string]: string } = {};

  private static template = (ele: PointLinksSk) =>
    html` <div ?hidden=${Object.keys(ele.displayUrls || {}).length === 0}>
      <ul>
        ${ele.renderLinks()}
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
      const linkText = this.displayTexts[key];
      return html`<li>${key}: <a href="${link}"> ${linkText}</a></li>`;
    };
    return keys.map(getHtml);
  }

  // load and display the links for the given commit and trace.
  public async load(
    cid: CommitNumber,
    prev_cid: CommitNumber,
    traceid: string,
    keysForCommitRange: string[]
  ): Promise<void> {
    // Clear any existing links first.
    this.displayUrls = {};
    this.displayTexts = {};
    if (keysForCommitRange !== null) {
      const currentLinks = await this.getLinksForPoint(cid, traceid);
      const prevLinks = await this.getLinksForPoint(prev_cid, traceid);
      keysForCommitRange.forEach((key) => {
        const currentCommitUrl = currentLinks[key];
        if (
          currentCommitUrl !== undefined &&
          currentCommitUrl !== null &&
          currentCommitUrl !== ''
        ) {
          const prevCommitUrl = prevLinks[key];
          const currentCommitId = this.getCommitIdFromCommitUrl(
            currentCommitUrl
          ).substring(0, 7);
          const prevCommitId = this.getCommitIdFromCommitUrl(
            prevCommitUrl
          ).substring(0, 7);
          if (currentCommitId === prevCommitId) {
            this.displayUrls[key] = currentCommitUrl;
            this.displayTexts[key] = currentCommitId;
          } else {
            const repoUrl = this.getRepoUrlFromCommitUrl(currentCommitUrl);
            const commitRangeUrl = `${repoUrl}+log/${prevCommitId}..${currentCommitId}`;
            const displayKey = `${key} Range`;
            this.displayUrls[displayKey] = commitRangeUrl;
            this.displayTexts[displayKey] = this.getFormattedCommitRangeText(
              prevCommitId,
              currentCommitId
            );
          }
        }
      });
      this._render();
    }
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
