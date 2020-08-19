/**
 * @module modules/commits-table-sk
 * @description An element that manages fetching and processing commits data for Status.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { styleMap, StyleInfo } from 'lit-html/directives/style-map';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';


import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
    IncrementalCommitsResponse, Task, Comment, IncrementalUpdate,
    Branch, LongCommit, IncrementalCommitsRequest, ResponseMetadata
} from '../rpc/statusFe'
import 'elements-sk/select-sk';

import '../commits-data-sk';
import { CommitsDataSk, CategorySpec } from '../commits-data-sk/commits-data-sk';

type CommitHash = string;
type TaskSpec = string;
type TaskId = string;


export interface Commit extends LongCommit {
  shortAuthor: string;
  shortHash: string;
  shortSubject: string;
  issue: string;
  patchStorage: string;
  isRevert: boolean;
  isReland: boolean;
  ignoreFailure: boolean;
}

export class CommitsTableSk extends ElementSk {
  commits: Array<Commit> = [];
  commitsByHash: Map<CommitHash, Commit> = new Map();

  private static template = (el: CommitsTableSk) => html`
<commits-data-sk @commits-data-update=${el.draw}></commits-data-sk>
${el.data() ? CommitsTableSk.tableTemplate(el) : ''}
`;
// Break the template in two so we can conditionally use the commits-data-sk, we have to render once to make it exist, then we re render to fetch its data and fill in.
private static tableTemplate = (el: CommitsTableSk) => html`
<div id=commitsTableContainer>
  <div id=commits>
    <div id=legend>Legend placeholder</div>
    ${el.data().commits.map((commit: Commit, index: number) => html`
    <div class=commit>
      <div class=time-spacer></div>
      <span class=commit-details>${commit.author}</span>
    </div>
    `)}
  </div>
  <div id=tasksTable>
 // TODOOOOO do I need to nest anything if I'm using a css grid? or drop it all on there with really good styles computed ahead of time?] 
    ${el.data().categories.forEach((spec: CategorySpec, category: string) => {
      const style = { 'flexGrow': spec.colspan.toString() };
      return html`
      <div class=category style=${styleMap(style)}></div>
    `})}
  </div>
</div>`;
  constructor() {
      super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    ($$('commits-data-sk', this) as CommitsDataSk)
  }

  data(): CommitsDataSk {
    return $$('commits-data-sk') as CommitsDataSk;
  }
  
  draw() {
   this._render();
  }
};

define('commits-table-sk', CommitsTableSk);

// shortCommit returns the first 7 characters of a commit hash.
function shortCommit(commit: string): string {
    return commit.substring(0, 7);
}