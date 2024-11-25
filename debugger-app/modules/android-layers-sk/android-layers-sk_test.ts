import './index';
import { expect } from 'chai';
import { AndroidLayersSk } from './android-layers-sk';
import { LayerInfo } from '../commands-sk/commands-sk';
import { LayerSummary } from '../debugger';
import { DefaultMap } from '../default-map';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('android-layers-sk', () => {
  const newInstance = setUpElementUnderTest<AndroidLayersSk>('android-layers-sk');

  let androidLayersSk: AndroidLayersSk;
  beforeEach(() => {
    androidLayersSk = newInstance((el: AndroidLayersSk) => {});
  });

  describe('update function', () => {
    it('update', () => {
      const summaries = <LayerSummary[]>[
        {
          nodeId: 111,
          frameOfLastUpdate: 0,
          fullRedraw: true,
          layerWidth: 100,
          layerHeight: 100,
        },
        {
          nodeId: 222,
          frameOfLastUpdate: 4,
          fullRedraw: true,
          layerWidth: 100,
          layerHeight: 100,
        },
        {
          nodeId: 333,
          frameOfLastUpdate: 2,
          fullRedraw: true,
          layerWidth: 100,
          layerHeight: 100,
        },
      ];
      const maps: LayerInfo = {
        uses: new DefaultMap<number, number[]>(() => []),
        names: new Map<number, string>([
          [111, 'pie crust'],
          [222, 'marmalade'],
          [333, 'apples'],
        ]),
      };
      maps.uses.set(111, [0, 4]);
      maps.uses.set(222, [4]);
      // note that layer 333 is neither updated or used on frame 4
      const frame = 4;

      androidLayersSk.update(maps, summaries, frame);
    });
  });
});
