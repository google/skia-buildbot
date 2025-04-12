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

import '@material/web/icon/icon.js';
import '@material/web/iconbutton/outlined-icon-button.js';

export interface CommitLinks {
  traceid: string;
  cid: number;
  displayUrls: { [key: string]: string } | null;
  displayTexts: { [key: string]: string } | null;
}

export class PointLinksSk extends ElementSk {
  constructor() {
    super(PointLinksSk.template);
  }

  // The point links for the current commit.
  commitPosition: CommitNumber | null = null;

  // Contains the urls to be displayed.
  displayUrls: { [key: string]: string } = {};

  // Contains texts to be displayed.
  // TODO(sunxiaodi@): remove display texts
  displayTexts: { [key: string]: string } = {};

  private buildLogText = 'Build Log';

  private fuchsiaBuildLogKey = 'Test stdio';

  private static template = (ele: PointLinksSk) =>
    html`<div class="point-links" ?hidden=${Object.keys(ele.displayUrls || {}).length === 0}>
      <ul class="table">
        ${ele.renderPointLinks()} ${ele.renderRevisionLink()}
      </ul>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  renderPointLinks(): TemplateResult[] {
    if (Object.keys(this.displayTexts).length === 0 && Object.keys(this.displayUrls).length === 0) {
      return [];
    }
    const keys = Object.keys(this.displayUrls);
    const getHtml = (key: string): TemplateResult => {
      const link = this.displayUrls![key];
      // TODO(b/398878559): Strip after 'Git' string until json keys are ready.
      const keyText: string = key.split(' Git')[0];
      const linkText = this.displayTexts![key] || 'Link';
      const htmlUrl = html`<a
        href="${link}"
        title="${linkText}"
        style="cursor: pointer;"
        target="_blank"
        >${linkText}</a
      >`;
      // generate text contents
      return html` <li>
        <span id="tooltip-link">${keyText}</span>
        <span id="tooltip-text">
          ${link.startsWith('http') ? htmlUrl : link}
          <md-icon-button @click=${() => this.copyToClipboard(link)}>
            <md-icon id="copy-icon">content_copy</md-icon>
          </md-icon-button>
        </span>
      </li>`;
    };
    return keys.map(getHtml);
  }

  renderRevisionLink() {
    // TODO(seawardt@): Disabling until proper usage is identified.
    if (true) {
      return html``;
    }
    if (this.commitPosition === null) {
      return html``;
    }
    const link = `/u/?rev=${this.commitPosition}`;
    return html` <li>
      <span id="tooltip-link">Related</span>
      <span id="tooltip-text">
        <a href="${link}" title="${this.commitPosition}" style="cursor: pointer;" target="_blank"
          >Anomalies</a
        >
        <md-icon-button @click=${() => this.copyToClipboard(link)}>
          <md-icon id="copy-icon">content_copy</md-icon>
        </md-icon-button>
      </span>
    </li>`;
  }

  private copyToClipboard(text: string): void {
    navigator.clipboard.writeText(text);
  }

  // load and display the links for the given commit and trace.
  public async load(
    cid: CommitNumber,
    prev_cid: CommitNumber,
    traceid: string,
    keysForCommitRange: string[],
    keysForUsefulLinks: string[],
    commitLinks: (CommitLinks | null)[]
  ): Promise<(CommitLinks | null)[]> {
    this.commitPosition = cid;
    if (commitLinks.length > 0) {
      commitLinks.forEach((commitLink) => {
        if (commitLink && commitLink.cid === cid && commitLink.traceid === traceid) {
          // Commit and TraceID have already been loaded, reuse links.
          this.displayUrls = commitLink.displayUrls || {};
          this.displayTexts = commitLink.displayTexts || {};
        }
      });
      if (Object.keys(this.displayUrls).length > 0 || Object.keys(this.displayTexts).length > 0) {
        this._render();
        return commitLinks; // Return the commitLinks object
      }
    }

    const currentLinks: { [key: string]: string } | null = await this.getLinksForPoint(
      cid,
      traceid
    );

    if (currentLinks === null || currentLinks === undefined) {
      // No links found for the current commit, return with no change.
      this._render();
      return commitLinks; // Return the commitLinks object as is.
    }

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
          const currentCommitId = this.getCommitIdFromCommitUrl(currentCommitUrl).substring(0, 8);
          const prevCommitId = this.getCommitIdFromCommitUrl(prevCommitUrl).substring(0, 8);

          // The links should be different depending on whether the
          // commits are the same. If the commits are the same, simply point to
          // the commit. If they're not, point to the log list.
          const displayKey = `${key}`;
          if (currentCommitId === prevCommitId) {
            this.displayTexts[displayKey] = `${currentCommitId} (No Change)`;
            this.displayUrls[displayKey] = currentCommitUrl;
          } else {
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
    const commitLink: CommitLinks = {
      cid: cid,
      traceid: traceid,
      displayUrls: this.displayUrls,
      displayTexts: this.displayTexts,
    };

    if (keysForUsefulLinks === null) {
      this._render();
      commitLinks.push(commitLink);
      return commitLinks; // Return the commitLinks object
    }
    Object.keys(currentLinks).forEach((key) => {
      if (keysForUsefulLinks.includes(key)) {
        this.displayUrls[key] = currentLinks[key];
      }
    });
    // Extra links found, add them to the displayUrls.
    commitLink.displayUrls = this.displayUrls;
    this._render();
    commitLinks.push(commitLink);
    return commitLinks;
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
    let url = '/_/links/';
    let response = await this.invokeLinksForPointApi(cid, traceId, url);
    if (!response) {
      url = '/_/details/?results=false';
      response = await this.invokeLinksForPointApi(cid, traceId, url);
    }
    // Workaround for b/410254837 until data is fixed.
    const chroimumUrl = 'https://chromium.googlesource.com/';
    Object.keys(response).forEach((key) => {
      if (key === 'V8' && !response[key].startsWith('http')) {
        const v8Url = 'v8/v8/';
        response[key] = chroimumUrl.concat(v8Url).concat(response[key]);
      }
      if (key === 'WebRTC' && !response[key].startsWith('http')) {
        const webrtcUrl = 'external/webrtc';
        response[key] = chroimumUrl.concat(webrtcUrl).concat(response[key]);
      }
    });
    return response;
  }

  /**
   * Invoke the api with the given url to get links for the data point.
   * @param cid Commit id
   * @param traceId Trace id
   * @param url Url of the api
   * @returns Links relevant to the commit id and trace id.
   */
  private async invokeLinksForPointApi(
    cid: CommitNumber,
    traceId: string,
    url: string
  ): Promise<{ [key: string]: string }> {
    const body: CommitDetailsRequest = {
      cid: cid,
      traceid: traceId,
    };
    let response: { [key: string]: string } = {};
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
      response = format.links!;
      // Currently in fuchsia json response, the key-value pair is not "Build Log": "url".
      // For example, the key-value format for fuchsia instance is:
      // Test stdio: '[Build Log](https://ci.chromium.org/b/8719307892946930401)'
      if (format.links && format.links![this.fuchsiaBuildLogKey]) {
        const val = this.extractUrlFromStringForFuchsia(format.links![this.fuchsiaBuildLogKey]);
        response[this.buildLogText] = val;
        delete response[this.fuchsiaBuildLogKey];
        return response;
      }
    } catch (error) {
      errorMessage(error as string);
    }
    return response;
  }

  // Extract url from string such as: "[Build Log](url)"
  private extractUrlFromStringForFuchsia(value: string): string {
    const expression = /\[[^\]]+\]\((.*?)\)/;
    const match = value.match(expression);
    if (match && match[1]) {
      return match[1];
    }
    return '';
  }
}

define('point-links-sk', PointLinksSk);
