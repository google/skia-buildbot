import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import { Anomaly } from '../json';
import { AnomalySk } from './anomaly-sk';

const dummyAnomaly = (bugId: number): Anomaly => {
  return {
    id: 1,
    test_path: '',
    bug_id: bugId,
    start_revision: 1234,
    end_revision: 1239,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
  };
};

window.customElements.whenDefined('anomaly-sk').then(() => {
  $$<AnomalySk>('#good')!.anomaly = dummyAnomaly(12345);
  $$<AnomalySk>('#good-dark')!.anomaly = dummyAnomaly(12345);
  $$<AnomalySk>('#empty-bug')!.anomaly = dummyAnomaly(-1);
});
