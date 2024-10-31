import './anomalies-table-sk';

export interface AnomaliesTableColumn {
  check_header: boolean | null;
  graph_header: string | null;
  bug_id: string;
  end_revision: string;
  master: string;
  bot: string;
  test_suite: string;
  test: string;
  change_direction: string;
  percent_changed: string;
  absolute_delta: string;
}

export interface AnomaliesTableRow {
  columns: (AnomaliesTableColumn | null)[] | null;
}

export interface AnomaliesTableResponse {
  table: (AnomaliesTableRow | null)[] | null;
}
