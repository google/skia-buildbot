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

  private abortController: AbortController = new AbortController();

  // The point links for the current commit.
  commitPosition: CommitNumber | null = null;

  // Contains the urls to be displayed.
  _displayUrls: { [key: string]: string } = {};

  // Contains texts to be displayed.
  _displayTexts: { [key: string]: string } = {};

  private buildLogText = 'Build Log';

  private fuchsiaBuildLogKey = 'Test stdio';

  // _pointLinks stores the asynchronously rendered TemplateResult[] for the point links.
  // This is necessary because `lit-html` cannot directly render an array of Promises.
  private _pointLinks: TemplateResult[] = [];

  private static template = (ele: PointLinksSk) =>
    html`<div class="point-links" ?hidden=${Object.keys(ele.displayUrls || {}).length === 0}>
      <ul class="table">
        ${ele._pointLinks} ${ele.renderRevisionLink()}
      </ul>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    // Initial render is handled by the `load` and `reset` functions,
    // which now call `renderPointLinks` to manage asynchronous updates.
    this.render();
  }

  /**
   * Asynchronously renders the point links and updates the `_pointLinks` property.
   * This function is now `async` to `await` the `isRange` check and uses `Promise.all`
   * to ensure all individual link rendering promises resolve before updating the DOM.
   */
  async renderPointLinks(): Promise<void> {
    if (Object.keys(this.displayTexts).length === 0 && Object.keys(this.displayUrls).length === 0) {
      this._pointLinks = [];
      this.render();
      return;
    }
    const keys = Object.keys(this.displayUrls);
    const getHtml = async (key: string): Promise<TemplateResult> => {
      const link = this.displayUrls![key];
      // TODO(b/398878559): Strip after 'Git' string until json keys are ready.
      const keyText: string = key.split(' Git')[0];
      let linkText = this.displayTexts![key] || 'Link';
      // This is a specific change for just v8.
      if (keyText === 'V8') {
        const isRange = await this.isRange(link);
        if (!isRange && linkText.includes(' - ')) {
          linkText = linkText.split(' - ')[1];
        }
      }
      const htmlUrl = html`<a
        href="${link}"
        title="${linkText}"
        style="cursor: pointer;"
        target="_blank"
        >${linkText}</a
      >`;
      // generate text contents
      return html` <li>
        <span id="tooltip-key">${keyText}</span>
        <span id="tooltip-text"> ${link.startsWith('http') ? htmlUrl : link} </span>
      </li>`;
    };
    const htmlPromises = keys.map(getHtml);
    this._pointLinks = await Promise.all(htmlPromises);
    this.render();
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
      <span id="tooltip-key">Related</span>
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

  // isRange returns true if the given link refers to a range of commits, i.e.
  // more than one commit.
  //
  // The ingested data always contains 2 commit hashes, but it is possible that
  // there is only a single commit between the 2 hashes. We request a JSON
  // formatted response from googlesource and check the response to see how many
  // commits exist in the log.
  private async isRange(link: string): Promise<boolean> {
    // We can only inspect googlesource links that point to a commit.
    if (!link.includes('googlesource.com')) {
      // Not a googlesource commit link we can inspect, assume it's not a single
      // commit, so we treat it as a range to be safe.
      return true;
    }

    const url = new URL(link);
    url.searchParams.set('format', 'JSON');

    const proxyUrl = `/_/json//?url=${encodeURIComponent(url.toString())}`;

    try {
      const resp = await fetch(proxyUrl);
      if (!resp.ok) {
        // If the proxy fails or the underlying request fails, we can't determine.
        // Let's log the error and assume it's a range.
        const text = await resp.text();
        console.error(
          `Failed to fetch through proxy for ${url.toString()}: ${resp.status} ${text}`
        );
        return true;
      }
      const text = await resp.text();

      // Handle googlesource's JSON prefix: )]}'
      if (!text.startsWith(")]}'")) {
        // Not the JSON format we expect.
        return true;
      }
      const jsonStr = text.substring(4);
      // It's possible to get an empty response body.
      if (!jsonStr.trim()) {
        return true;
      }
      const json = JSON.parse(jsonStr);

      // A range is defined as having more than one commit in the log.
      if (json && Array.isArray(json.log)) {
        return json.log.length > 1;
      }
      // If we don't have a log array, we can't determine, so assume it's a range.
      return true;
    } catch (error) {
      errorMessage(`Error while determining if link is a range: ${error}`);
      // On any other error (e.g. network, JSON parsing), we can't be sure.
      // Returning true is safer as it will display the full link text.
      return true;
    }
  }

  // load and display the links for the given commit and trace.
  public async load(
    commit_position: CommitNumber | null,
    prev_commit_position: CommitNumber | null,
    trace_id: string,
    keysForCommitRange: string[],
    keysForUsefulLinks: string[],
    commitLinks: (CommitLinks | null)[]
  ): Promise<(CommitLinks | null)[]> {
    this.abortController.abort(); // Abort any previous ongoing request
    this.abortController = new AbortController(); // Create a new controller for the current request
    const signal = this.abortController.signal;

    this.commitPosition = commit_position;
    await this.reset();
    if (commit_position === null) {
      return Promise.resolve(commitLinks);
    }
    if (commitLinks.length > 0) {
      // Check if the commit and traceID have already been loaded. Also verify that the existing
      // link contains urls.
      const existingLink = commitLinks.find(
        (commitLink) =>
          commitLink &&
          commitLink.cid === commit_position &&
          commitLink.traceid === trace_id &&
          commitLink.displayUrls &&
          Object.keys(commitLink.displayUrls).length > 0
      );
      if (existingLink) {
        // Reuse the existing links
        this.displayUrls = existingLink.displayUrls || {};
        this.displayTexts = existingLink.displayTexts || {};
        await this.renderPointLinks();
        return Promise.resolve(commitLinks);
      }
    }

    try {
      const currentLinks: { [key: string]: string } | null = await this.getLinksForPoint(
        commit_position,
        trace_id
      );

      if (signal.aborted) {
        console.log(`Request aborted for ${commit_position})`);
        return Promise.resolve(commitLinks);
      }

      if (currentLinks === null || currentLinks === undefined) {
        // No links found for the current commit, return with no change.
        return commitLinks; // Return the commitLinks object as is.
      }
      const displayTexts: { [key: string]: string } = {};
      const displayUrls: { [key: string]: string } = {};

      if (keysForCommitRange !== null && prev_commit_position !== null) {
        const prevLinks = await this.getLinksForPoint(prev_commit_position, trace_id);
        if (signal.aborted) {
          console.log(`Request aborted for ${commit_position})`);
          return Promise.resolve(commitLinks);
        }
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
            // Workaround to ensure no duplication with links.
            const displayKey = `${key.split(' Git')[0]}`;
            // The links should be different depending on whether the
            // commits are the same. If the commits are the same, simply point to
            // the commit. If they're not, point to the log list.

            if (currentCommitId === prevCommitId) {
              displayTexts[displayKey] = `${currentCommitId} (No Change)`;
              displayUrls[displayKey] = currentCommitUrl;
            } else {
              displayTexts[displayKey] = this.getFormattedCommitRangeText(
                prevCommitId,
                currentCommitId
              );
              const repoUrl = this.getRepoUrlFromCommitUrl(currentCommitUrl);
              // Set pagination to large value to ease skipping (1000 is arbitrary).
              const commitRangeUrl = `${repoUrl}+log/${prevCommitId}..${currentCommitId}?n=1000`;
              displayUrls[displayKey] = commitRangeUrl;
            }
          }
        });
      }
      // Extra links found, add them to the displayUrls.
      Object.keys(currentLinks).forEach((key) => {
        if (keysForUsefulLinks && keysForUsefulLinks.includes(key)) {
          displayUrls[key] = currentLinks[key];
        }
      });

      const commitLink: CommitLinks = {
        cid: commit_position,
        traceid: trace_id,
        displayUrls: displayUrls,
        displayTexts: displayTexts,
      };

      if (commitLinks.indexOf(commitLink) === -1) {
        commitLinks.push(commitLink);
      }

      this.displayTexts = displayTexts;
      this.displayUrls = displayUrls;
      await this.renderPointLinks();

      // Before adding a new commit link, check if it already exists in the array.
      // This should not be necessary, but it is a safeguard due to async calls.
      const existingLink = commitLinks.find(
        (commitLink) =>
          commitLink && commitLink.cid === commit_position && commitLink.traceid === trace_id
      );
      if (!existingLink) {
        commitLinks.push(commitLink);
      }
      this.render();
      return commitLinks;
    } catch (error: any) {
      if (error.name === 'AbortError') {
        console.log(`Request aborted for ${commit_position})`);
      } else {
        console.error(`Error fetching ${commit_position}:`, error);
      }
    }
    return Promise.resolve(commitLinks);
  }

  /** Clear Point Links */
  async reset(): Promise<void> {
    this.commitPosition = null;
    this.displayUrls = {};
    this.displayTexts = {};
    await this.renderPointLinks();
  }

  render(): void {
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
    if (response) {
      // Workaround for b/410254837 until data is fixed.
      const chromiumUrl = 'https://chromium.googlesource.com/';
      Object.keys(response).forEach((key) => {
        if (key === 'V8' && !response[key].startsWith('http')) {
          const v8Url = 'v8/v8/+/';
          response[key] = chromiumUrl.concat(v8Url).concat(response[key]);
        }
        if (key === 'WebRTC' && !response[key].startsWith('http')) {
          const webrtcUrl = 'external/webrtc/+/';
          response[key] = chromiumUrl.concat(webrtcUrl).concat(response[key]);
        }
      });
    }
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
      // Currently in fuchsia json response. The key-value pair is not "Build Log": "url".
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

  set displayTexts(val: { [key: string]: string }) {
    this._displayTexts = val;
  }

  get displayTexts(): { [key: string]: string } {
    return this._displayTexts;
  }

  set displayUrls(val: { [key: string]: string }) {
    this._displayUrls = val;
  }

  get displayUrls(): { [key: string]: string } {
    return this._displayUrls;
  }
}

define('point-links-sk', PointLinksSk);
