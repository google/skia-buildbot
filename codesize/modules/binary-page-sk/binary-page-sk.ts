/** Shows code size statistics about a single binary. */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { load } from '@google-web-components/google-chart/loader';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';
import { BloatyOutputMetadata, BinaryRPCRequest, BinaryRPCResponse, TreeMapDataTableRow } from '../rpc_types';
import '../../../infra-sk/modules/human-date-sk';
import '@google-web-components/google-chart/';
import Fuse from 'fuse.js';
import { $$ } from 'common-sk/modules/dom';

/**
 * Convert the data we get from the server into a format that can be visualized by the treemap.
 * It will shorten file names by removing the path, for example "foo/bar/baz.cc" becomes "baz.cc".
 * Because the name is the key to the row, we also then must shorten usages of those names later
 * (in the parent column), but cannot shorten the same name more than once (e.g. we have two
 * files with the same name in different subfolders).
 */
export function convertResponseToDataTable(inputRows: TreeMapDataTableRow[]): [string, string, string|number][] {
  const out: [string, string, string|number][] = [];
  out.push(['Name', 'Parent', 'Size']);

  const shortNames = new Set<string>();
  const parentsWhichWereShortened = new Set<string>();

  for (const row of inputRows) {
    // Shorten the file name, but only if the resulting shortened file name is unique.
    const shortenedName = shortenName(row.name);
    const shortenedNameIsUnique = !shortNames.has(shortenedName);
    if (shortenedNameIsUnique && row.name !== shortenedName) {
      shortNames.add(shortenedName);
      parentsWhichWereShortened.add(row.name);
    }
    // If the parent was shortened, use the shortened version.
    // Otherwise, it was a duplicate, so keep the full name.
    out.push([
      shortenedNameIsUnique ? shortenedName : row.name,
      parentsWhichWereShortened.has(row.parent) ? shortenName(row.parent) : row.parent,
      row.size,
    ]);
  }
  return out;
}

/**
 * Shortens a file name path to be just the name by splitting the string that contains '.'
 * (indicating that it is a file and not a folder or function) on '/' and taking the last element
 * which should be the file name.
 */
export function shortenName(rowName: string): string {
  if (!rowName) {
    return '';
  }
  if (rowName.includes('.') && rowName.includes('/')) {
    rowName = rowName.split('/').pop()!;
  }
  return rowName;
}

const MAX_SEARCH_RESULTS = 10;

export class BinaryPageSk extends ElementSk {
  private static template = (el: BinaryPageSk) => {
    if (el.metadata === null) {
      return html`<p>Loading...</p>`;
    }

    const isTryJob = el.metadata?.patch_issue || el.metadata?.patch_set;
    const commitOrCLAnchorText = isTryJob
      ? `Issue ${el.metadata?.patch_issue}, PS ${el.metadata?.patch_set}`
      : el.metadata?.revision.substring(0, 7);
    const commitOrCLAnchorHref = isTryJob
      ? `https://review.skia.org/${el.metadata?.patch_issue}/${el.metadata?.patch_set}`
      : `https://skia.googlesource.com/skia/+/${el.metadata?.revision}`;

    const compileTaskNameHref = `https://task-scheduler.skia.org/task/${el.metadata?.task_id}`;
    return html`
      <h2>
        Code size statistics for <code>${el.metadata?.binary_name}</code>
        <span class="compile-task">
          (<a href="${compileTaskNameHref}">${el.metadata?.compile_task_name}</a>)
        </span>
      </h2>

      <p>
        <a href="${commitOrCLAnchorHref}">${commitOrCLAnchorText}</a>
        ${el.metadata?.subject}
        <br/>
        <span class="author-and-timestamp">
          ${el.metadata?.author},
          <human-date-sk .date=${el.metadata?.timestamp} .diff=${true}></human-date-sk> ago.
        </span>
      </p>

      <p class="instructions">Instructions:</p>

      <ul>
        <li><strong>Click</strong> on a node to navigate down the tree.</li>
        <li><strong>Right click</strong> anywhere on the treemap go back up one level.</li>
        <li><strong> Use the seach bar</strong> to navigate to a node within the tree</li>

      </ul>

      <div class="search-bar">
        <input type="search" placeholder="Search for node..." aria-label="Search for node..."
        autocomplete="on" @input=${el.onSearchInput} @keyup=${el.onSearchKeyUp}>
        <ol id="searchSuggestions" class="search-match-list" ?hidden=${!el.listOfSearchResults.length}
            @mouseover=${el.clearDefaultSelected}>
          ${el.listOfSearchResults.map((result, i) => el.searchResult(result, i))}
        </ol>
      </div>

      <div id="treemap"></div>
    `;
  }

  // Returns a <li> with the matching string. If it is the first element, mark it selected to
  // give an affordance that something is auto selected and the user can hit enter to pick it.
  private searchResult = (match: Fuse.FuseResult<string>, idx: number): TemplateResult => html`
<li class=${'search-match-list-item ' + (idx === 0 ? 'selected': '')}
    @click=${() => this.showElement(match)}>
  ${match.item}
</li>
`;

  private tree: google.visualization.TreeMap | null = null;
  private fuse: Fuse<string> | null = null;
  private listOfSearchResults: Fuse.FuseResult<string>[] = [];

  // Uses Fuse.js to match a users input to a node within the tree. Returns the top
  // MAX_SEARCH_RESULTS results.
  private onSearchInput(e: Event):void {
    if (!this.fuse) {
      return;
    }
    const target = e.target as HTMLInputElement;
    this.listOfSearchResults = this.fuse.search(target.value).slice(0, MAX_SEARCH_RESULTS);
    this._render();
  }

  private showElement(match: Fuse.FuseResult<string> | undefined): void {
    if (match) {
      // https://developers.google.com/chart/interactive/docs/events#the-select-event
      // The row number should correspond to the order of which the data was passed into the
      // treemap upon creation. This is the same order we passed into Fuse, and we can find
      // that matching index by looking at refIndex.
      const selectedIdx = match.refIndex;
      this.tree!.setSelection([{column: null, row: selectedIdx}]);
    }
    this.listOfSearchResults = []; // hide other results
    $$<HTMLInputElement>('.search-bar input', this)!.value = ''; // clear search bar
    this._render();
  }

  // If the user hits enter, change the selected element to the top search result.
  // Otherwise, make sure the first element is highlighted to indicate this behavior.
  private onSearchKeyUp(e: Event): void {
      if (!this.tree) {
        return;
      }
      const evt = (e as KeyboardEvent);
      if (evt.key === 'Enter') {
        // User hit enter, use the top result.
        this.showElement(this.listOfSearchResults[0]);
        return;
      }
      // Auto-highlight the first list element again (in case it was cleared via mouse over),
      // thus resetting the affordance that it is the auto-picked version.
      if (this.listOfSearchResults.length >= 1) {
        const firstItem = $$<HTMLLIElement>('#searchSuggestions li:first-child', this);
        if (firstItem) {
          firstItem.classList.add('selected');
        }
      }
  }

  // If the user starts typing, then mouses over the list, we clear the default selected option.
  private clearDefaultSelected(e: Event): void {
    const defaultSelected = $$<HTMLLIElement>('#searchSuggestions li.selected', this);
    if (defaultSelected) {
      defaultSelected.classList.remove('selected');
    }
  }

  private metadata: BloatyOutputMetadata | null = null;

  constructor() {
    super(BinaryPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Show a loading indicator while the tree is loading.
    CodesizeScaffoldSk.waitFor(this.loadTreeMap());
  }

  private async loadTreeMap(): Promise<void> {
    const params = new URLSearchParams(window.location.search);
    const request: BinaryRPCRequest = {
      commit: params.get('commit') || '',
      patch_issue: params.get('patch_issue') || '',
      patch_set: params.get('patch_set') || '',
      binary_name: params.get('binary_name') || '',
      compile_task_name: params.get('compile_task_name') || '',
    };
    const [, response] = await Promise.all([
      load({ packages: ['treemap'] }),
      fetch('/rpc/binary/v1', { method: 'POST', body: JSON.stringify(request) })
        .then(jsonOrThrow)
        .then((r: BinaryRPCResponse) => r),
    ]);

    this.metadata = response.metadata;
    this._render();

    const rows = convertResponseToDataTable(response.rows);
    const data = google.visualization.arrayToDataTable(rows);
    this.tree = new google.visualization.TreeMap(this.querySelector('#treemap')!);

    // For some reason the type definition for TreeMapOptions does not include the generateTooltip
    // option (https://developers.google.com/chart/interactive/docs/gallery/treemap#tooltips), so
    // a type assertion is necessary to keep the TypeScript compiler happy.
    // TODO(kjlubick): add categorical coloring
    const treeOptions = {
      generateTooltip: showTooltip,
      minColor: '#E8DAFF',
      midColor: '#E8DAFF',
      maxColor: '#E8DAFF',
    } as google.visualization.TreeMapOptions;

    const searchOptions = {
      isCaseSensitive: false,
      // Ignore single character matches
      minMatchCharLength: 1,
      // At what point does the algorithm gives up (0.0 is a perfect match)
      threshold: 0.6,
    };

    // Strip off the first row, which is the column headings.
    this.fuse = new Fuse(rows.slice(1, rows.length).map((row): string => row[0]), searchOptions);

    // Draw the tree and wait until the tree finishes drawing.
    await new Promise((resolve) => {
      google.visualization.events.addOneTimeListener(this.tree, 'ready', resolve);
      this.tree!.draw(data, treeOptions);
      document.addEventListener('theme-chooser-toggle', () => {
        // if a user toggles the theme to/from darkmode then redraw
        this.tree!.draw(data, treeOptions);
      });
    });

    // Shows the label of the treemap cell. Returns a string with the HTML to be shown whenever.
    // the user hovers over a treemap cell.
    function showTooltip(row: number, size: string) {
      const escapedLabel = data.getValue(row, 0)
        .replace('&', '&amp;')
        .replace('<', '&lt;')
        .replace('>', '&gt;');
      const backgroundColor = isDarkMode() ? '#232F34' : '#FFFFFF';
      return `<div class= "cell-tooltip" style="background: ${backgroundColor};">
          <span>
            ${escapedLabel} <br/>
            Size: ${size} <br/>
          </span>
        </div>`;
    }
  }
}
define('binary-page-sk', BinaryPageSk);
