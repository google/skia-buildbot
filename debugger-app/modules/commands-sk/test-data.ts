import { SkpJsonCommandList } from '../debugger';

export const testData: SkpJsonCommandList = {
  commands: [
    { // 0
      command: 'Save',
      shortDesc: 'the trees',
      auditTrail: { Ops: [] },
    },
    { // 1
      command: 'DrawAnnotation',
      key: 'What kind of SKP is this anyways',
      auditTrail: { Ops: [] },
    },
    { // 2
      command: 'ClipRect',
      auditTrail: { Ops: [] },
    },
    { // 3
      command: 'DrawImageRect',
      shortDesc: 'A picture of a corgy',
      imageIndex: 3,
      auditTrail: { Ops: [] },
    },
    { // 4
      command: 'Save',
      shortDesc: 'your money',
      auditTrail: { Ops: [] },
    },
    { // 5
      command: 'ClipRRect',
      auditTrail: { Ops: [] },
    },
    { // 6
      command: 'DrawTextBlob',
      shortDesc: 'user was panned for this boast',
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
      // TODO(nifong): make this actually refer to something once layer parsing added
      layerNodeId: 20,
      auditTrail: { Ops: [] },
    },
    { // 8
      command: 'Restore',
      shortDesc: 'your money',
      auditTrail: { Ops: [] },
    },
    { // 9
      command: 'Restore',
      shortDesc: 'the trees',
      auditTrail: { Ops: [] },
    },
  ],
};