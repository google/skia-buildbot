export interface TraceRow {
  commit_number: number;
  val: number;
  createdat: number;
  metadata?: Record<string, string> | null;
  hash?: string;
  url?: string;
  smoothedVal?: number;
  author?: string;
  message?: string;
}

export interface TraceSeries {
  id: string;
  source?: string;
  originalId?: string;
  rows: TraceRow[];
  allStats?: Record<string, TraceRow[]>;
  color: string;
}

export interface ProcessedTraceSeries extends TraceSeries {
  parsedColor: { r: number; g: number; b: number };
}
