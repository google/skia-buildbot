import { ReactiveController, ReactiveControllerHost } from 'lit';
import { Anomaly, CalculateRegrShortcutResponse, GetGroupReportResponse, Timerange } from '../json';
import { errorMessage } from '../errorMessage';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

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

  async openReportForAnomalyIds(anomalies: Anomaly[], newTab?: Window | null): Promise<boolean> {
    const idList = anomalies.map((a) => a.id);
    if (newTab === undefined) {
      newTab = null;
    }

    // If only one anomaly is selected, open the report page using
    // the anomaly id directly.
    if (idList.length === 1) {
      const key = idList[0];
      const url = `/u/?anomalyIDs=${key}`;
      if (newTab) {
        newTab.location.href = url;
      } else {
        newTab = window.open(url, '_blank');
      }
      return !!newTab;
    }

    const idString = idList.join(',');
    const response = window.perf.fetch_anomalies_from_sql
      ? await this.fetchCreateShortcutApi(idString)
      : await this.fetchGroupReportApi(idString);
    if (!response) {
      if (newTab) newTab.close();
      // Return true to avoid triggering the "Popups blocked" error in the caller,
      // as the error was due to an API failure, not a blocked popup.
      return true;
    }
    const sid: string = response.sid || '';
    const url = `/u/?sid=${sid}`;
    if (newTab) {
      newTab.location.href = url;
    } else {
      newTab = window.open(url, '_blank');
    }
    return !!newTab;
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

  private async fetchCreateShortcutApi(
    idString: string
  ): Promise<CalculateRegrShortcutResponse | null> {
    try {
      const response = await fetch('/_/anomalies/calculate_regr_shortcut', {
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
        `&request_type=0&shortcut=${this.shortcutUrl}`;
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
