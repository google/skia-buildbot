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
import { LitElement, TemplateResult, html } from 'lit';
import { property, state } from 'lit/decorators.js';
import { until } from 'lit/directives/until.js';
import { define } from '../../../elements-sk/modules/define';
import { CommitDetailsRequest, CommitNumber, ingest } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';

import '@material/web/icon/icon.js';
import { TrimHash } from '../common/commit';

export interface CommitLinks {
  traceid: string;
  cid: number;
  displayUrls: { [key: string]: string } | null;
  displayTexts: { [key: string]: string } | null;
  fetched?: boolean;
}

export function sanitizeUrl(url: string): string {
  try {
    const parsed = new URL(url, window.location.href);
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
      return url;
    }
  } catch (e) {
    console.log(`sanitize url error: ${e}`);
  }
  return '#';
}

export class PointLinksSk extends LitElement {
  protected createRenderRoot() {
    return this;
  }

  private abortController: AbortController = new AbortController();

  // The point links for the current commit.
  @state()
  commitPosition: CommitNumber | null = null;

  // Contains the urls to be displayed.
  @property({ attribute: false })
  displayUrls: { [key: string]: string } = {};

  // Contains texts to be displayed.
  @property({ attribute: false })
  displayTexts: { [key: string]: string } = {};

  private buildLogText = 'Build Log';

  private fuchsiaBuildLogKey = 'Test stdio';

  render() {
    return html`<div
      class="point-links"
      ?hidden=${Object.keys(this.displayUrls || {}).length === 0}>
      <ul class="table">
        ${Object.keys(this.displayUrls)
          .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()))
          .map((key) => until(this.getHtml(key), html`<li>Loading...</li>`))}
        ${this.renderRevisionLink()}
      </ul>
    </div>`;
  }

  private async getHtml(key: string): Promise<TemplateResult> {
    const link = this.displayUrls![key];
    // TODO(b/398878559): Strip after 'Git' string until json keys are ready.
    const keyText: string = key
      .replace(/ Git.*/i, '')
      .replace('Trace Iteration', 'Trace')
      .replace('Device fingerprint', 'Fingerprint')
      .replace('Profiling Traces and Test Artifacts', 'Artifacts');
    let linkText = this.displayTexts![key];
    let href = link;

    const mdLinkRegex = /^\[(.*?)\]\((.*?)\)$/;
    const match = link.match(mdLinkRegex);
    // This is a specific change for just v8.
    if (keyText === 'V8') {
      const isRange = await this.isRange(link);
      if (!isRange && linkText.includes(' - ')) {
        linkText = linkText.split(' - ')[1];
      }
    } else {
      if (match) {
        href = match[2];
      }
      if (!linkText) {
        linkText = 'Link';
      }
    }
    const htmlUrl = html`<a
      href="${sanitizeUrl(href)}"
      title="${linkText}"
      style="cursor: pointer;"
      target="_blank"
      >${linkText}</a
    >`;
    // generate text contents
    return html` <li>
      <span id="tooltip-key">${keyText}</span>
      <span id="tooltip-text"> ${href.startsWith('http') ? htmlUrl : link} </span>
    </li>`;
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
    navigator.clipboard.writeText(text).then(
      () => {},
      (error) => {
        console.error('Failed to copy to clipboard:', error);
      }
    );
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
      return commitLinks;
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
      if (existingLink && existingLink.fetched) {
        // Reuse the existing links
        this.displayUrls = existingLink.displayUrls || {};
        this.displayTexts = existingLink.displayTexts || {};
        return commitLinks;
      }
    }

    try {
      const currentLinks: { [key: string]: string } | null = await this.getLinksForPoint(
        commit_position,
        trace_id
      );

      if (signal.aborted) {
        console.log(`Request aborted for ${commit_position})`);
        return commitLinks;
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
          return commitLinks;
        }
        if (prevLinks) {
          keysForCommitRange.forEach((key) => {
            const currentCommitUrl = currentLinks[key];
            if (
              currentCommitUrl !== undefined &&
              currentCommitUrl !== null &&
              currentCommitUrl !== ''
            ) {
              const prevCommitUrl = prevLinks[key];
              const currentCommitId = TrimHash(this.getCommitIdFromCommitUrl(currentCommitUrl));
              const prevCommitId = TrimHash(this.getCommitIdFromCommitUrl(prevCommitUrl));
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
      }
      // Extra links found, add them to the displayUrls.
      Object.keys(currentLinks).forEach((key) => {
        // TODO(b/398878559): Strip after 'Git' string until json keys are ready.
        const cleanKey = key.replace(/ Git.*/i, '');
        if (keysForUsefulLinks) {
          const match = keysForUsefulLinks.some(
            (k) =>
              k.trim().toLowerCase() === key.trim().toLowerCase() ||
              k.trim().toLowerCase() === cleanKey.trim().toLowerCase()
          );
          if (match) {
            displayUrls[key] = currentLinks[key];
          }
        }
      });
      const commitLink: CommitLinks = {
        cid: commit_position,
        traceid: trace_id,
        displayUrls: displayUrls,
        displayTexts: displayTexts,
        fetched: true,
      };

      const existingIndex = commitLinks.findIndex(
        (cl) => cl && cl.cid === commit_position && cl.traceid === trace_id
      );

      if (existingIndex !== -1) {
        commitLinks[existingIndex] = commitLink;
      } else {
        commitLinks.push(commitLink);
      }

      this.displayTexts = displayTexts;
      this.displayUrls = displayUrls;

      return commitLinks;
    } catch (error: any) {
      if (error.name === 'AbortError') {
        console.log(`Request aborted for ${commit_position})`);
      } else {
        console.error(`Error fetching ${commit_position}:`, error);
      }
    }
    return commitLinks;
  }

  /** Clear Point Links */
  async reset(): Promise<void> {
    this.commitPosition = null;
    this.displayUrls = {};
    this.displayTexts = {};
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
  ): Promise<{ [key: string]: string } | null> {
    // TODO(b/398878559): Revert back to using /_/links/ as the primary source once
    // the endpoint is fixed to return all links.
    let url = '/_/details/?results=false';
    let response = await this.invokeLinksForPointApi(cid, traceId, url);
    if (!response) {
      url = '/_/links/';
      response = await this.invokeLinksForPointApi(cid, traceId, url);
    }
    if (response) {
      // Workaround for b/410254837 until data is fixed.
      const chromiumUrl = 'https://chromium.googlesource.com/';
      Object.keys(response).forEach((key) => {
        if (key === 'V8' && !response![key].startsWith('http')) {
          const v8Url = 'v8/v8/+/';
          response![key] = chromiumUrl.concat(v8Url).concat(response![key]);
        }
        if (key === 'WebRTC' && !response![key].startsWith('http')) {
          const webrtcUrl = 'external/webrtc/+/';
          response![key] = chromiumUrl.concat(webrtcUrl).concat(response![key]);
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
  ): Promise<{ [key: string]: string } | null> {
    const body: CommitDetailsRequest = {
      cid: cid,
      traceid: traceId,
    };
    let response: { [key: string]: string } | null = null;
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
      response = format.links || null;
      // Currently in fuchsia json response. The key-value pair is not "Build Log": "url".
      // For example, the key-value format for fuchsia instance is:
      // Test stdio: '[Build Log](https://ci.chromium.org/b/8719307892946930401)'
      if (format.links && format.links![this.fuchsiaBuildLogKey]) {
        const val = this.extractUrlFromStringForFuchsia(format.links![this.fuchsiaBuildLogKey]);
        response![this.buildLogText] = val;
        delete response![this.fuchsiaBuildLogKey];
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
