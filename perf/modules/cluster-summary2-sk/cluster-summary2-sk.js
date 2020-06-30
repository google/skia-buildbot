/**
 * @module modules/cluster-summary2-sk
 * @description <h2><code>cluster-summary2-sk</code></h2>
 *
 * @evt open-keys - An event that is fired when the user clicks the 'View on
 *     dashboard' button that contains the shortcut id, and the timestamp range of
 *     traces in the details that should be opened in the explorer, and the xbar
 *     location specified as a serialized cid.CommitID, for example:
 *
 *     {
 *       shortcut: 'X1129832198312',
 *       begin: 1476982874,
 *       end: 1476987166,
 *       xbar: {'source':'master','offset':24750,'timestamp':1476985844},
 *     }
 *
 * @evt triaged - An event generated when the 'Update' button is pressed, which
 *     contains the new triage status. The detail contains the cid and triage
 *     status, for example:
 *
 *     {
 *       cid: {
 *         source: 'master',
 *         offset: 25004,
 *       },
 *       triage: {
 *         status: 'negative',
 *         messge: 'This is a regression in ...',
 *       },
 *     }
 *
 * @attr {Boolean} notriage - If true then don't display the triage controls.
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import 'elements-sk/styles/buttons';
import 'elements-sk/collapse-sk';

import '../commit-detail-panel-sk';
import '../plot-simple-sk';
import '../triage2-sk';
import '../word-cloud-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Login } from '../../../infra-sk/modules/login';

function _trunc(value) {
  return (+value).toPrecision(3);
}

const template = (ele) => html`
  <div class='regression ${ele._statusClass()}'>
    Regression: <span>${_trunc(ele._summary.step_fit.regression)}</span>
  </div>
  <div class=stats>
    <div class=labelled>Cluster Size: <span>${ele._summary.num}</span></div>
    <div class=labelled>Least Squares Error: <span>${_trunc(ele._summary.step_fit.least_squares)}</span></div>
    <div class=labelled>Step Size: <span>${_trunc(ele._summary.step_fit.step_size)}</span></div>
  </div>
  <plot-simple-sk class=plot width=800 height=250 specialevents @trace_selected=${ele._traceSelected}></plot-simple-sk>
  <div id=status class=${ele._hiddenClass()}>
    <p class=disabledMessage>You must be logged in to change the status.</p>
    <triage2-sk value=${ele._triage.status} @change=${(e) => { ele._triage.status = e.detail; }}></triage2-sk>
    <input type=text .value=${ele._triage.message} @change=${(e) => { ele._triage.message = e.target.value; }} label=Message>
    <button class=action @click=${ele._update}>Update</button>
  </div>
  <commit-detail-panel-sk id=commits selectable></commit-detail-panel-sk>
  <div class=actions>
    <button id=shortcut @click=${ele._openShortcut}>View on dashboard</button>
    <a id=permalink class=${ele._hiddenClass()} href=${ele._permaLink()}>Permlink</a>
    <a id=rangelink href='' target=_blank></a>
    <button @click=${ele._toggleWordCloud}>Word Cloud</button>
  </div>
  <collapse-sk class=wordCloudCollapse closed>
    <word-cloud-sk .items=${ele._summary.param_summaries2}></word-cloud-sk>
  </collapse-sk>
`;

export class ClusterSummary2Sk extends ElementSk {
  constructor() {
    super(template);
    this._summary = {
      num: 0,
      step_fit: {
        regression: 0,
        least_squares: 0,
        step_size: 0,
      },
      param_summaries2: [],
    };
    this.triage = {
      status: '',
      message: '',
    };
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('full_summary');
    this._upgradeProperty('triage');
    this._render();
    this._wordCloud = this.querySelector('.wordCloudCollapse');
    this._status = this.querySelector('#status');
    this._graph = this.querySelector('plot-simple-sk');
    this._rangelink = this.querySelector('#rangelink');
    this._commits = this.querySelector('#commits');
    Login.then((status) => {
      this._status.classList.toggle('disabled', status.Email === '');
    }).catch(errorMessage);
    this.full_summary = this.full_summary;
    this.triage = this.triage;
  }

  _update() {
    const cid = this._summary.step_point;
    const detail = {
      cid,
      triage: this.triage,
    };
    this.dispatchEvent(new CustomEvent('triaged', { detail, bubbles: true }));
  }

  _openShortcut() {
    const detail = {
      shortcut: this._summary.shortcut,
      begin: this._frame.dataframe.header[0].timestamp,
      end:
        this._frame.dataframe.header[this._frame.dataframe.header.length - 1]
          .timestamp + 1,
      xbar: this._summary.step_point,
    };
    this.dispatchEvent(new CustomEvent('open-keys', { detail, bubbles: true }));
  }

  /**
   * Look up the commit ids for the given offsets and sources.
   *
   * @param {Array} cids - An array of serialized cid.CommitID.
   * @returns {Promise} A Promise that resolves the cids and returns an Array of serialized cid.CommitDetails.
   */
  static _lookupCids(cids) {
    return fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(cids),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow);
  }

  _traceSelected(e) {
    const h = this._frame.dataframe.header[e.detail.x];
    ClusterSummary2Sk._lookupCids([h])
      .then((json) => {
        this._commits.details = json;
      })
      .catch(errorMessage);
  }

  _toggleWordCloud() {
    this._wordCloud.closed = !this._wordCloud.closed;
  }

  _hiddenClass() {
    return this.hasAttribute('notriage') ? 'hidden' : '';
  }

  _permaLink() {
    // Bounce to the triage page, but with the time range narrowed to
    // contain just the step_point commit.
    if (!this._summary || !this._summary.step_point) {
      return '';
    }
    const begin = this._summary.step_point.timestamp;
    const end = begin + 1;
    return `/t/?begin=${begin}&end=${end}&subset=all`;
  }

  _statusClass() {
    if (!this._summary) {
      return '';
    }
    const status = this._summary.step_fit.status || '';
    return status.toLowerCase();
  }

  /** @prop full_summary {string} A serialized:
   *
   *  {
   *    summary: cluster2.ClusterSummary,
   *    frame: dataframe.FrameResponse,
   *  }
   *
   */
  get full_summary() {
    return this._full_summary;
  }

  set full_summary(val) {
    if (!val) {
      return;
    }
    if (!val.frame) {
      return;
    }
    this._full_summary = val;
    this._summary = val.summary;
    this._frame = val.frame;
    if (!this._graph) {
      return;
    }

    // Set the data- attributes used for sorting cluster summaries.
    this.dataset.clustersize = this._summary.num;
    this.dataset.steplse = this._summary.step_fit.least_squares;
    this.dataset.stepsize = this._summary.step_fit.step_size;
    this.dataset.stepregression = this._summary.step_fit.regression;
    // We take in a ClusterSummary, but need to transform all that data
    // into a format that plot-sk can handle.
    this._graph.removeAll();
    const labels = [];
    this.full_summary.frame.dataframe.header.forEach((header) => {
      labels.push(new Date(header.timestamp * 1000));
    });
    this._graph.addLines({ centroid: this._summary.centroid }, labels);
    // Set the x-bar but only if status != uninteresting.
    if (this._summary.step_fit.status !== 'Uninteresting') {
      // Loop through the dataframe header to find the location we should
      // place the x-bar at.
      const step = this._summary.step_point;
      let xbar = -1;
      this._frame.dataframe.header.forEach((h, i) => {
        if (h.source === step.source && h.offset === step.offset) {
          xbar = i;
        }
      });
      if (xbar !== -1) {
        this._graph.xbar = xbar;
      }
      // Populate rangelink.
      if (window.sk.perf.commit_range_url !== '') {
        // First find the commit at step_fit, and the next previous commit that has data.
        let prevCommit = xbar - 1;
        while (prevCommit > 0 && this._summary.centroid[prevCommit] === 1e32) {
          prevCommit -= 1;
        }
        const cids = [
          this._frame.dataframe.header[prevCommit],
          this._frame.dataframe.header[xbar],
        ];
        // Run those through cid lookup to get the hashes.
        ClusterSummary2Sk._lookupCids(cids)
          .then((json) => {
            // Create the URL.
            let url = window.sk.perf.commit_range_url;
            url = url.replace('{begin}', json[0].hash);
            url = url.replace('{end}', json[1].hash);
            // Now populate link, including text and href.
            this._rangelink.href = url;
            this._rangelink.innerText = 'Commits At Step';
          })
          .catch(errorMessage);
      } else {
        this._rangelink.href = '';
        this._rangelink.innerText = '';
      }
    } else {
      this._rangelink.href = '';
      this._rangelink.innerText = '';
    }

    this._render();
  }

  /** @prop triage {string} The triage status of the cluster.
   *     Something of the form:
   *
   *    {
   *      status: 'untriaged',
   *      message: 'This is a regression.',
   *    }
   *
   */
  get triage() {
    return this._triage;
  }

  set triage(val) {
    if (!val) {
      return;
    }
    this._triage = val;
    this._render();
  }
}

define('cluster-summary2-sk', ClusterSummary2Sk);
