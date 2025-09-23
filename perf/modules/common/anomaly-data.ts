import { Anomaly } from '../json';

export interface AnomalyData {
  x: number;
  y: number;
  anomaly: Anomaly;
  highlight: boolean;
}
