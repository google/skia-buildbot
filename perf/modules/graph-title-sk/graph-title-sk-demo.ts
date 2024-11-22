import { $$ } from '../../../infra-sk/modules/dom';
import { GraphTitleSk } from './graph-title-sk';
import './index';

const titleEntries = new Map([
  ['bot', 'linux-perf'],
  ['benchmark', 'speedometer2'],
  ['test', 'Speedometer2'],
  ['subtest_1', 'abcdefghijklmnmopqrstuvwxyz'],
]);

$$<GraphTitleSk>('#good')!.set(titleEntries, 1);

const emptyEntries = new Map([
  ['bot', 'linux-perf'],
  ['benchmark', ''],
  ['test', 'Speedometer2'],
  ['', 'abcdefghijklmnmopqrstuvwxyz'],
]);
$$<GraphTitleSk>('#partial')!.set(emptyEntries, 4);

$$<GraphTitleSk>('#generic')!.set(new Map(), 4);
