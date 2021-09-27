import { TriageLogResponse2 } from '../rpc_types';

export const firstPageV2: TriageLogResponse2 = {
  entries: [
    {
      id: 'aaa',
      name: 'alpha@google.com',
      ts: 1572000000000,
      details: [
        {
          grouping: {
            source_type: 'corpus1',
            name: 'async_rescale_and_read_dog_up',
          },
          digest: 'f16298eb14e19f9230fe81615200561f',
          label_before: 'untriaged',
          label_after: 'positive',
        }, {
          grouping: {
            source_type: 'corpus1',
            name: 'async_rescale_and_read_rose',
          },
          digest: '35c77280a7d5378033f9bf8f3c755e78',
          label_before: 'negative',
          label_after: 'positive',
        },
      ],
    }, {
      id: 'bbb',
      name: 'beta@google.com',
      ts: 1571900000000,
      details: [{
        grouping: {
          source_type: 'corpus2',
          name: 'draw_image_set',
        },
        digest: 'b788aadee662c2b0390d698cbe68b808',
        label_before: 'untriaged',
        label_after: 'positive',
      }],
    }, {
      id: 'ccc',
      name: 'gamma@google.com',
      ts: 1571800000000,
      details: [{
        grouping: {
          source_type: 'corpus1',
          name: 'filterbitmap_text_7.00pt',
        },
        digest: '454b4b547bc6ceb4cdeb3305553be98a',
        label_before: 'untriaged',
        label_after: 'positive',
      }],
    },
  ],
  offset: 0,
  size: 3,
  total: 9,
};

export const secondPageV2: TriageLogResponse2 = {
  entries: [
    {
      id: 'ddd',
      name: 'delta@google.com',
      ts: 1571700000000,
      details: [{
        grouping: {
          source_type: 'corpus1',
          name: 'filterbitmap_text_10.00pt',
        },
        digest: 'fc8392000945e68334c5ccd333b201b3',
        label_before: 'untriaged',
        label_after: 'positive',
      }],
    }, {
      id: 'eee',
      name: 'epsilon@google.com',
      ts: 1571600000000,
      details: [{
        grouping: {
          source_type: 'corpus1',
          name: 'filterbitmap_image_mandrill_32.png',
        },
        digest: '7606bfd486f7dfdf299d9d9da8f99c8e',
        label_before: 'untriaged',
        label_after: 'positive',
      }],
    }, {
      id: 'fff',
      name: 'zeta@google.com',
      ts: 1571500000000,
      details: [{
        grouping: {
          source_type: 'corpus1',
          name: 'drawminibitmaprect_aa',
        },
        digest: '95e1b42fcaaff5d0d08b4ed465d79437',
        label_before: 'untriaged',
        label_after: 'positive',
      }],
    },
  ],
  offset: 3,
  size: 3,
  total: 9,
};
