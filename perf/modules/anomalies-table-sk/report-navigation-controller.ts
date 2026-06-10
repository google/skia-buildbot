import { ReactiveController, ReactiveControllerHost } from 'lit';
import {
  Anomaly,
  CalculateRegrShortcutResponse,
  GetGroupReportResponse,
  Timerange,
  ShiftResponse,
} from '../json';
import { errorMessage } from '../errorMessage';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

const weekInSeconds = 7 * 24 * 60 * 60;
const SECONDS_IN_DAY = 24 * 60 * 60;

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

  public async openMultiGraphUrl(anomaly: Anomaly, newTab: Window | null, isDryRun = false) {
    const response = await this.fetchGroupReportApi(String(anomaly.id));
    let timerangeMap = response?.timerange_map || {};

    if (!timerangeMap[anomaly.id] && isDryRun) {
      const shiftResp = await this.fetchShiftApi(anomaly.start_revision, anomaly.end_revision);
      if (shiftResp) {
        timerangeMap = {
          ...timerangeMap,
          [anomaly.id]: {
            begin: shiftResp.begin,
            end: shiftResp.end + SECONDS_IN_DAY, // Shift end time by a day to match backend getTimerangeMap
          },
        };
      }
    }

    if (!timerangeMap[anomaly.id]) {
      console.warn('ReportNavigationController: timerange missing for anomaly', anomaly.id);
      if (newTab) newTab.close();
      return;
    }

    const urlList = await this.generateMultiGraphUrl([anomaly], timerangeMap);

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

  private async fetchShiftApi(begin: number, end: number): Promise<ShiftResponse | null> {
    try {
      const response = await fetch('/_/shift', {
        method: 'POST',
        body: JSON.stringify({
          begin: begin,
          end: end,
        }),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      return jsonOrThrow(response);
    } catch (msg) {
      errorMessage('Failed to fetch shift data: ' + msg);
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
