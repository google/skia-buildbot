import { $$ } from 'common-sk/modules/dom';
import { ParamSet, pivot } from '../json/all';
import './index';
import { PivotQueryChangedEventDetail, PivotQuerySk } from './pivot-query-sk';

const validPivotRequest: pivot.Request = {
  group_by: ['config', 'os'],
  operation: 'avg',
  summary: [],
};

const paramSet: ParamSet = {
  config: ['8888', '565'],
  arch: ['x86', 'risc-v'],
  model: ['Pixel2', 'Pixel3'],
};

const valid = $$<PivotQuerySk>('#valid')!;
valid.pivotRequest = validPivotRequest;
valid.paramset = paramSet;

document.addEventListener('pivot-changed', ((e: CustomEvent<PivotQueryChangedEventDetail>) => {
  $$('#results')!.textContent = JSON.stringify(e.detail, null, '  ');
}) as EventListener);
