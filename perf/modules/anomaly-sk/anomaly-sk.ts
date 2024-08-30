/**
 * @module modules/anomaly-sk
 * @description <h2><code>anomaly-sk</code></h2>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  Anomaly,
  AnomalyMap,
  ColumnHeader,
  CommitNumber,
  TraceSet,
} from '../json';
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';
import { lookupCids } from '../cid/cid';
import '../window/window';

/**
 * Use DataFrame and AnomalyMap to construct an AnomalyDataMap object. This
 * object maps each trace to its anomalies and their coordinates. Constructing
 * this object, makes it easier to plot the anomalies and display their
 * metadata when clicked.
 *
 * Example Input:
 * traceSet:
 * {
 * 'traceA': [142.3, 120.0, 120.0],
 * 'traceB': [500.0, 632.2, 120.0],
 * }
 *
 * header:
 * [{offset: 1234, ...}, {offset: 1236, ...}, {offset: 1239}]
 *
 * anomalymap:
 * {
 * 'traceA': {'1234': a1},
 * 'traceB': {'1234': a2, '1236': a3}
 * }
 *
 * Example Output:
 * {
 * 'traceA': [{'x': 1, 'y': 142.3, 'anomaly': a1}],
 * 'traceB': [{'x': 1, 'y': 500.0, 'anomaly': a2},
 * {'x': 2, 'y': 623.2, 'anomaly': a3}]
 * }
 *
 * @param {TraceSet} traceSet - A TraceSet object. We only look for anomalies
 * from traces in traceSet. The traces also provide y coordinate positions.
 * @param {Object} header - A ColumnHeader array. We use header to map commit positions
 * to x coordinate positions.
 * @param {Object} anomalymap - an AnomalyMap object. This object contains all
 * the Anomaly objects used to populate the anomalyDataMap.
 */
export const getAnomalyDataMap = (
  traceSet: TraceSet,
  header: (ColumnHeader | null)[],
  anomalymap: AnomalyMap,
  highlight_anomalies: string[]
): { [key: string]: AnomalyData[] } => {
  const anomalyDataMap: { [traceId: string]: AnomalyData[] } = {};

  // Iterate the traceSet and see if a trace has anomalies in the anomaly map.
  for (const traceId in traceSet) {
    const trace = traceSet[traceId];
    if (traceId in anomalymap!) {
      const cidAnomalyMap = anomalymap![traceId];
      if (cidAnomalyMap !== null) {
        anomalyDataMap[traceId] = [];

        // If it has anomalies, add all of them as AnomalyData objects.
        // To find the y coord, search the commit number on columnHeader.
        Object.keys(cidAnomalyMap)
          .map(Number)
          .forEach((cid) => {
            for (let i = 0; i < header!.length; i++) {
              const columnHeader = header![i];
              // There are scenarios where a trace is missing due to upload failure,
              // so we may not get a perfect match for the cid in the header offset.
              // Simply use the next available commit in this case. This way if the
              // anomaly is specified on a trace that failed uploading to skia perf,
              // we show the anomaly on the next available data point instead of not
              // displaying it at all.
              if (columnHeader!.offset >= cid) {
                const currentAnomaly = cidAnomalyMap[cid];
                // If the currentAnomaly is present in the highlight list, mark it for highlighting.
                const highlight =
                  highlight_anomalies !== null &&
                  highlight_anomalies.includes(currentAnomaly.id.toString());

                anomalyDataMap[traceId].push({
                  anomaly: cidAnomalyMap[cid],
                  x: i,
                  y: trace[i],
                  highlight: highlight,
                });
                break;
              }
            }
          });
      }
    }
  }
  return anomalyDataMap;
};

const commitNumberToHashes = async (
  cids: CommitNumber[]
): Promise<string[]> => {
  const json = await lookupCids(cids);
  return [json.commitSlice![0].hash, json.commitSlice![1].hash];
};

export class AnomalySk extends ElementSk {
  private _anomaly: Anomaly | null = null;

  private _bugHostUrl: string = 'https://bugs.chromium.org';

  private _revision: TemplateResult = html``;

  constructor() {
    super(AnomalySk.template);
  }

  static formatNumber = (num: number): string =>
    num.toLocaleString('en-US', {
      maximumFractionDigits: 4,
    });

  static formatPercentage = (num: number): string =>
    num.toLocaleString('en-US', {
      maximumFractionDigits: 4,
      signDisplay: 'exceptZero',
    });

  static getPercentChange = (
    median_before: number,
    median_after: number
  ): number => {
    const difference = median_after - median_before;
    // Division by zero is represented by infinity symbol.
    return (100 * difference) / median_before;
  };

  formatRevisionRange = async (): Promise<void> => {
    if (this.anomaly == null) return;

    const start_rev = this.anomaly.start_revision;
    const end_rev = this.anomaly.end_revision;

    const cids: CommitNumber[] = [
      CommitNumber(start_rev),
      CommitNumber(end_rev),
    ];

    const hashes = await commitNumberToHashes(cids);

    let url = window.perf.commit_range_url;
    if ([null, undefined, ''].includes(url)) {
      this._revision = html`${start_rev} - ${end_rev}`;
      return;
    }

    url = url.replace('{begin}', hashes[0]);
    url = url.replace('{end}', hashes[1]);

    this._revision = html`<a href="${url}" target=_blank>${start_rev} - ${end_rev}</td>`;
    this._render();
  };

  static formatBug(bugHostUrl: string, bugId: number): TemplateResult {
    if (bugId === -1) {
      return html``;
    }
    return html`<a href="${`${bugHostUrl}/${bugId}`}" target=_blank>${bugId}</td>`;
  }

  private static template = (ele: AnomalySk) => {
    if (ele._anomaly === null) {
      return html``;
    }
    const anomaly = ele._anomaly!;
    return html`
      <div>
        <table>
          <thead>
            <tr>
              <th colspan="2">Anomaly Details</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <th>Score</th>
              <td>${AnomalySk.formatNumber(anomaly.median_after_anomaly)}</td>
            </tr>
            <tr>
              <th>Prior Score</th>
              <td>${AnomalySk.formatNumber(anomaly.median_before_anomaly)}</td>
            </tr>
            <tr>
              <th>Percent Change</th>
              <td>
                ${AnomalySk.formatPercentage(
                  AnomalySk.getPercentChange(
                    anomaly.median_before_anomaly,
                    anomaly.median_after_anomaly
                  )
                )}%
              </td>
            </tr>
            <tr>
              <th>Revision Range</th>
              <td>${ele.revision}</td>
            </tr>
            <tr>
              <th>Improvement</th>
              <td>${anomaly.is_improvement}</td>
            </tr>
            <tr>
              <th>Bug Id</th>
              <td>${AnomalySk.formatBug(ele.bugHostUrl, anomaly.bug_id)}</td>
            </tr>
          </tbody>
        </table>
      </div>
    `;
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('anomaly');
    this._render();
  }

  get anomaly(): Anomaly | null {
    return this._anomaly;
  }

  set anomaly(anomaly: Anomaly | null) {
    this._anomaly = anomaly;
    this.formatRevisionRange();
    this._render();
  }

  get bugHostUrl(): string {
    return this._bugHostUrl;
  }

  set bugHostUrl(url: string) {
    if (url !== '') {
      // Trim the trailing '/' since we are adding it in the format.
      if (url.endsWith('/')) {
        url = url.substring(0, url.length - 1);
      }
      this._bugHostUrl = url;
    }
  }

  get revision(): TemplateResult {
    return this._revision;
  }
}

define('anomaly-sk', AnomalySk);
