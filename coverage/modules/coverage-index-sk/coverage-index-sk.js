import 'common-sk/modules/error-toast-sk'

import { errorMessage } from 'common-sk/modules/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import { html, render } from 'lit-html/lib/lit-extended'

// How often to update the data.
const UPDATE_INTERVAL_MS = 60000;

// Main template for this element
const template = (ele) => html`
<header>Code Coverage</header>

<main>
  ${commitsList(ele._ingested)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>`;

const commitsList = (commits) => commits.map(commit => html`
<h3>${_header(commit.info)}</h3>
${(commit.combined && commit.combined.total_lines && _combinedCoverage(commit)) || "" }
${jobsList(commit, commit.jobs)}`);

const _combinedCoverage = (commit) => html`
<div class=combined>
    <a href="coverage?commit=${commit.info.hash}&job=Combined">Combined</a>
     - ${commit.combined.coverage} %
    (${commit.combined.covered_lines} / ${commit.combined.total_lines} lines)
</div>`;

const jobsList = (commit, jobs) => jobs.map(job => html`
<div>
  <a href="coverage?commit=${commit.info.hash}&job=${job.name}">${job.name}</a>
  - ${job.coverage}%
  (${job.covered_lines} / ${job.total_lines} lines)
</div>`);

function _header(info){
  let subject = info.subject
  if (subject.length > 60) {
    subject = subject.substr(0, 57) + "...";
  }
  return info.hash.substr(0, 10) + " - " + subject + " - " + info.author.split("(")[0];
}

// The <coverage-index-sk> custom element declaration.
//
//  This is the main page for coverage.skia.org.
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    None
//
window.customElements.define('coverage-index-sk', class extends HTMLElement {

  constructor() {
    super();
    this._ingested = [];
  }

  connectedCallback() {
    this._render();
    // make a fetch ASAP, but not immediately (demo mock up may not be set up yet)
    window.setTimeout(() => this.update());
  }

  update() {
    fetch('/ingested')
      .then(jsonOrThrow)
      .then((json) => {
        json.list = json.list || [];
        json.list.forEach(function(c){
          c.jobs = c.jobs || [];
          c.jobs.forEach(function(j){
            if (j.total_lines && j.missed_lines) {
              j.coverage = (100 * (j.total_lines - j.missed_lines)/j.total_lines).toFixed(2);
              j.covered_lines = j.total_lines - j.missed_lines;
            }
          });
          if (c.combined && c.combined.total_lines) {
            let j = c.combined;
            j.coverage = (100 * (j.total_lines - j.missed_lines)/j.total_lines).toFixed(2);
            j.covered_lines = j.total_lines - j.missed_lines;
          }
        });
        this._ingested = json.list;
        this._render();
        window.setTimeout(() => this.update(), UPDATE_INTERVAL_MS);
      })
      .catch((e) => {
        errorMessage(e);
        window.setTimeout(() => this.update(), UPDATE_INTERVAL_MS);
      });
  }

  _render() {
    render(template(this), this);
  }

});
