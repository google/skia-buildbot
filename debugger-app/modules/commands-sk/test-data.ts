import { SkpJsonCommandList } from '../debugger';

export const testData: SkpJsonCommandList = {
  commands: [
    { // 0
      command: 'Save',
      shortDesc: 'the trees',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 1
      command: 'DrawAnnotation',
      shortDesc: '',
      key: 'What kind of SKP is this anyways',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 2
      command: 'ClipRect',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 3
      command: 'DrawImageRect',
      shortDesc: 'A picture of a corgy',
      key: '',
      imageIndex: 3,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 4
      command: 'Save',
      shortDesc: 'your money',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 5
      command: 'ClipRRect',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 6
      command: 'DrawTextBlob',
      shortDesc: 'user was panned for this boast',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: {
        Ops: [
          {
            'Name': 'North Korelana is Best Korelana',
            'ClientID': 1,
            'OpsTaskID': 2,
            'ChildID': 3,
          }
        ],
      },
    },
    { // 7
      command: 'DrawImageRectLayer',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      // TODO(nifong): make this actually refer to something once layer parsing added
      layerNodeId: 20,
      auditTrail: { Ops: [] },
    },
    { // 8
      command: 'Restore',
      shortDesc: 'your money',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 9
      command: 'Restore',
      shortDesc: 'the trees',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
  ],
};