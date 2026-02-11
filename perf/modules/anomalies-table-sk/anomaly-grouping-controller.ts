import { ReactiveController, ReactiveControllerHost } from 'lit';
import { Anomaly } from '../json';
import {
  AnomalyGroup,
  AnomalyGroupingConfig,
  groupAnomalies,
  RevisionGroupingMode,
  GroupingCriteria,
} from './grouping';

const GROUPING_CONFIG_STORAGE_KEY = 'perf-grouping-config';

export class AnomalyGroupingController implements ReactiveController {
  private host: ReactiveControllerHost;

  groups: AnomalyGroup[] = [];

  config: AnomalyGroupingConfig = {
    revisionMode: 'OVERLAPPING',
    groupBy: new Set(['BENCHMARK']),
    groupSingles: true,
  };

  private anomalyList: Anomaly[] = [];

  constructor(host: ReactiveControllerHost) {
    this.host = host;
    this.host.addController(this);
  }

  hostConnected() {
    this.loadGroupingConfig();
  }

  setConfig(config: AnomalyGroupingConfig) {
    this.config = { ...config };
    if (this.config.groupBy && Array.isArray(this.config.groupBy)) {
      this.config.groupBy = new Set(this.config.groupBy);
    }
    this.refreshGrouping();
  }

  setAnomalies(anomalies: Anomaly[]) {
    this.anomalyList = anomalies;
    this.refreshGrouping();
  }

  setRevisionMode(mode: RevisionGroupingMode) {
    this.config.revisionMode = mode;
    this.refreshGrouping();
  }

  toggleGroupBy(criteria: GroupingCriteria, enabled: boolean) {
    if (enabled) {
      this.config.groupBy.add(criteria);
    } else {
      this.config.groupBy.delete(criteria);
    }
    this.refreshGrouping();
  }

  setGroupSingles(enabled: boolean) {
    this.config.groupSingles = enabled;
    this.refreshGrouping();
  }

  private refreshGrouping() {
    this.saveGroupingConfig();
    this.groups = groupAnomalies(this.anomalyList, this.config);
    this.host.requestUpdate();
  }

  private loadGroupingConfig() {
    const storedConfig = localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY);
    if (storedConfig) {
      try {
        const parsed = JSON.parse(storedConfig);
        // Need to convert groupBy from array to Set.
        if (parsed.groupBy && Array.isArray(parsed.groupBy)) {
          parsed.groupBy = new Set(parsed.groupBy);
        }
        this.config = { ...this.config, ...parsed };
      } catch (e) {
        console.error('Failed to parse grouping config from localStorage', e);
        localStorage.removeItem(GROUPING_CONFIG_STORAGE_KEY);
      }
    }
  }

  private saveGroupingConfig() {
    // Need to convert Set to array for JSON serialization.
    const configToStore = {
      ...this.config,
      groupBy: Array.from(this.config.groupBy),
    };
    localStorage.setItem(GROUPING_CONFIG_STORAGE_KEY, JSON.stringify(configToStore));
  }
}
