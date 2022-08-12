import { TriageLogEntry } from '../rpc_types';

export const triageLogs: TriageLogEntry[] = [
  {
    id: '4275c86b-d64a-ae38-d931-24ea9b94c551',
    name: 'userFour@example.com',
    ts: 1607691600000,
    details: [{
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a09a09a09a09a09a09a09a09a09a09a0', label_before: 'untriaged', label_after: 'negative',
    }],
  },
  {
    id: '734d45d8-555a-aca5-6c55-c45039e43f89',
    name: 'fuzzy',
    ts: 1607685060000,
    details: [{
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a08a08a08a08a08a08a08a08a08a08a0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: 'fe054e2f-822a-7e0c-3dfb-0e9586adffe4',
    name: 'userThree@example.com',
    ts: 1607595010000,
    details: [{
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a07a07a07a07a07a07a07a07a07a07a0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: '65693cef-0220-f0aa-3503-1d5df6548ac9',
    name: 'userThree@example.com',
    ts: 1591877595000,
    details: [{
      grouping: { name: 'circle', source_type: 'round' }, digest: '00000000000000000000000000000000', label_before: 'untriaged', label_after: 'negative',
    }],
  },
  {
    id: 'a23a2b37-344e-83a1-fc71-c72f8071280a',
    name: 'userThree@example.com',
    ts: 1591877594000,
    details: [{
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a03a03a03a03a03a03a03a03a03a03a0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: 'c2b9779e-a0e7-9d48-7c91-0edfa48db809',
    name: 'userOne@example.com',
    ts: 1591518188000,
    details: [{
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a01a01a01a01a01a01a01a01a01a01a0', label_before: 'untriaged', label_after: 'positive',
    }, {
      grouping: { name: 'square', source_type: 'corners' }, digest: 'a02a02a02a02a02a02a02a02a02a02a0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: 'f9adaa96-df23-2128-2120-53ea2d57536b',
    name: 'userTwo@example.com',
    ts: 1591517708000,
    details: [{
      grouping: { name: 'triangle', source_type: 'corners' }, digest: 'b04b04b04b04b04b04b04b04b04b04b0', label_before: 'positive', label_after: 'negative',
    }],
  },
  {
    id: '931323d9-926d-3a24-0350-6440a54d52cc',
    name: 'userTwo@example.com',
    ts: 1591517707000,
    details: [{
      grouping: { name: 'triangle', source_type: 'corners' }, digest: 'b04b04b04b04b04b04b04b04b04b04b0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: '1d35d070-9ec6-1d0a-e7bd-1184870323b3',
    name: 'userTwo@example.com',
    ts: 1591517704000,
    details: [{
      grouping: { name: 'triangle', source_type: 'corners' }, digest: 'b03b03b03b03b03b03b03b03b03b03b0', label_before: 'untriaged', label_after: 'negative',
    }],
  },
  {
    id: 'fbbe2efb-5fc0-bd3c-76fa-b52714bad960',
    name: 'userOne@example.com',
    ts: 1591517383000,
    details: [{
      grouping: { name: 'triangle', source_type: 'corners' }, digest: 'b01b01b01b01b01b01b01b01b01b01b0', label_before: 'untriaged', label_after: 'positive',
    }, {
      grouping: { name: 'triangle', source_type: 'corners' }, digest: 'b02b02b02b02b02b02b02b02b02b02b0', label_before: 'untriaged', label_after: 'positive',
    }],
  },
  {
    id: '94a63df2-33d3-97ad-f4d7-341f76ff8cb6',
    name: 'userOne@example.com',
    ts: 1591517350000,
    details: [{
      grouping: { name: 'circle', source_type: 'round' }, digest: 'c01c01c01c01c01c01c01c01c01c01c0', label_before: 'untriaged', label_after: 'positive',
    }, {
      grouping: { name: 'circle', source_type: 'round' }, digest: 'c02c02c02c02c02c02c02c02c02c02c0', label_before: 'untriaged', label_after: 'positive',
    }],
  },

];
