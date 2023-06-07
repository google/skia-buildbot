/**
 * @module modules/anomaly-sk
 * @description <h2><code>anomaly-sk</code></h2>
 *
 */
import { html, TemplateResult } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Anomaly, AnomalyMap, ColumnHeader, TraceSet } from '../json';
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';

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
 * @param {Object} traceSet - A TraceSet object. We only look for anomalies
 * from traces in traceSet. The traces also provide y coordinate positions.
 * @param {Object} header - A ColumnHeader array. We use header to map commit positions
 * to x coordinate positions.
 * @param {Object} anomalymap - an AnomalyMap object. This object contains all
 * the Anomaly objects used to populate the anomalyDataMap.
 */
export const getAnomalyDataMap = (
  traceSet: TraceSet,
  header: (ColumnHeader | null)[],
  anomalymap: AnomalyMap
): { [key: string]: AnomalyData[] } => {
  const anomalyDataMap: { [traceId: string]: AnomalyData[] } = {};

  // Iterate the traceSet and see if a trace has anomalies in the anomaly map.
  Object.entries(traceSet).forEach(([traceId, trace]) => {
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
              if (columnHeader!.offset === cid) {
                anomalyDataMap[traceId].push({
                  anomaly: cidAnomalyMap[cid],
                  x: i,
                  y: trace[i],
                });
                break;
              }
            }
          });
      }
    }
  });
  return anomalyDataMap;
};

export class AnomalySk extends ElementSk {
  private _anomaly: Anomaly | null = null;

  constructor() {
    super(AnomalySk.template);
  }

  private static formatNumber = (num: number): string => {
    return num.toLocaleString('en-US', { maximumFractionDigits: 4 });
  };

  private static getPercentChange = (
    median_before: number,
    median_after: number
  ): number => {
    const difference = median_after - median_before;
    // Division by zero is represented by infinity symbol.
    return (100 * difference) / median_before;
  };

  private static formatBug = (bugId: number): TemplateResult => {
    if (bugId == -1) {
      return html``;
    }
    return html`<a href="${'https://crbug.com/' + bugId}">${bugId}</td>`;
  };

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
                ${AnomalySk.formatNumber(
                  AnomalySk.getPercentChange(
                    anomaly.median_before_anomaly,
                    anomaly.median_after_anomaly
                  )
                )}%
              </td>
            </tr>
            <tr>
              <th>Revision Range</th>
              <td>${anomaly.start_revision} - ${anomaly.end_revision}</td>
            </tr>
            <tr>
              <th>Bug Id</th>
              <td>${AnomalySk.formatBug(anomaly.bug_id)}</td>
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
    this._render();
  }
}

define('anomaly-sk', AnomalySk);
