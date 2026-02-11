import { ReactiveController, ReactiveControllerHost } from 'lit';
import { Anomaly, GetGroupReportResponse, Timerange } from '../json';
import { errorMessage } from '../errorMessage';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { CountMetric, telemetry } from '../telemetry/telemetry';

// Just below the 2000 limit - we need to leave some space for the instance address.
const urlMaxLength = 1900;
const weekInSeconds = 7 * 24 * 60 * 60;

export class ReportNavigationController implements ReactiveController {
  host: ReactiveControllerHost;

  private traceFormatter: ChromeTraceFormatter;

  private shortcutUrl: string = '';

  constructor(host: ReactiveControllerHost) {
    this.host = host;
    this.host.addController(this);
    this.traceFormatter = new ChromeTraceFormatter();
  }

  hostConnected() {}

  async openReportForAnomalyIds(anomalies: Anomaly[]) {
    const idList = anomalies.map((a) => a.id);

    // If only one anomaly is selected, open the report page using
    // the anomaly id directly.
    if (idList.length === 1) {
      const key = idList[0];
      window.open(`/u/?anomalyIDs=${key}`, '_blank');
      return;
    }

    if (window.perf.fetch_anomalies_from_sql) {
      const idString = idList.join(',');
      const urlForAnomalyIDsList = `/u/?anomalyIDs=${encodeURIComponent(idString)}`;
      if (urlForAnomalyIDsList.length < urlMaxLength) {
        window.open(urlForAnomalyIDsList, '_blank');
        return;
      }

      errorMessage(
        'Tried to open a report page with too many anomalies. Please file a bug to request access.'
      );
      console.warn('anomalyIDs url would be too long, need to use SID');
      telemetry.increaseCounter(CountMetric.SIDRequiringActionTaken, {
        module: 'anomalies-table-sk',
        function: 'openReportForAnomalyId',
      });
    } else {
      const idString = idList.join(',');
      const response = await this.fetchGroupReportApi(idString);
      if (!response) {
        return;
      }
      const sid: string = response.sid || '';
      const url = `/u/?sid=${sid}`;
      window.open(url, '_blank');
    }
  }

  // openMultiGraphLink generates a multi-graph url for the given parameters
  public async openMultiGraphUrl(anomaly: Anomaly, newTab: Window | null) {
    const response = await this.fetchGroupReportApi(String(anomaly.id));
    if (!response || !response.timerange_map) {
      console.warn('ReportNavigationController: response or timerange_map missing', response);
      if (newTab) newTab.close();
      return;
    }

    const urlList = await this.generateMultiGraphUrl([anomaly], response.timerange_map);

    this.openAnomalyUrl(urlList[0], newTab);
  }

  private openAnomalyUrl(url: string | undefined, newTab: Window | null): void {
    if (!newTab || !url) {
      console.warn('Multi chart URL not found or tab was blocked.');
      if (newTab) newTab.close(); // Clean up the blank tab on failure.
      return;
    }

    // Navigate the already-opened tab to the final destination.
    newTab.location.href = url;
  }

  private async fetchGroupReportApi(idString: string): Promise<GetGroupReportResponse | null> {
    try {
      const response = await fetch('/_/anomalies/group_report', {
        method: 'POST',
        body: JSON.stringify({
          anomalyIDs: idString,
        }),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      return jsonOrThrow(response);
    } catch (msg) {
      errorMessage(msg as string);
      return null;
    }
  }

  private async generateMultiGraphUrl(
    anomalies: Anomaly[],
    timerangeMap: { [key: string]: Timerange }
  ): Promise<string[]> {
    const shortcutUrlList: string[] = [];
    for (let i = 0; i < anomalies.length; i++) {
      const timerange = this.calculateTimeRange(timerangeMap[anomalies[i]!.id]);
      const graphConfigs = [] as GraphConfig[];
      const config: GraphConfig = {
        keys: '',
        formulas: [],
        queries: [],
      };
      config.queries = [this.traceFormatter.formatQuery(anomalies[i]!.test_path)];
      graphConfigs.push(config);
      await updateShortcut(graphConfigs)
        .then((shortcut) => {
          if (shortcut === '') {
            this.shortcutUrl = '';
            return;
          }
          this.shortcutUrl = shortcut;
        })
        .catch(errorMessage);

      // request_type=0 only selects data points for within the range
      // rather than show 250 data points by default
      const url =
        `${window.location.protocol}//${window.location.host}` +
        `/m/?begin=${timerange[0]}&end=${timerange[1]}` +
        `&request_type=0&shortcut=${this.shortcutUrl}&totalGraphs=1`;
      shortcutUrlList.push(url);
    }

    return shortcutUrlList;
  }

  private calculateTimeRange(timerange: Timerange): string[] {
    if (!timerange) {
      return ['', ''];
    }
    const timerangeBegin = timerange.begin;
    const timerangeEnd = timerange.end;

    // generate data one week ahead and one week behind to make it easier
    // for user to discern trends
    const newTimerangeBegin = timerangeBegin ? (timerangeBegin - weekInSeconds).toString() : '';
    const newTimerangeEnd = timerangeEnd ? (timerangeEnd + weekInSeconds).toString() : '';

    return [newTimerangeBegin, newTimerangeEnd];
  }
}
