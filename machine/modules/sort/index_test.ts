import { assert } from 'chai';
import { FrontendDescription } from '../json';
import {
  columnSortFunctions,
  down, SortHistory, SortSelection, up,
} from './index';

describe('SortSelection', () => {
  describe('toggleDirection', () => {
    it('toggles up to down', () => {
      const s = new SortSelection(1, down);
      s.toggleDirection();
      assert.equal(up, s.directionMultiplier());
      s.toggleDirection();
      assert.equal(down, s.directionMultiplier());
    });
  });

  it('encodes correctly', () => {
    const s = new SortSelection(1, down);
    assert.equal(s.encode(), 'd1');
  });

  it('decodes correctly', () => {
    const s = SortSelection.decode('u12');
    assert.equal(s.dir, up);
    assert.equal(s.column, 12);
  });

  it('handles invalid input to decode', () => {
    const s = SortSelection.decode('');
    assert.equal(s.dir, down);
    assert.equal(s.column, 0);
  });
});

describe('SortHistory', () => {
  const columnSorters: columnSortFunctions<FrontendDescription> = [
    () => 0,
    () => 0,
  ];

  describe('selectColumnToSortOn', () => {
    it('updates sort history', () => {
      const sh = new SortHistory(columnSorters);
      assert.equal(sh.history[0].column, 0);
      assert.equal(sh.history[0].dir, up);
      assert.equal(sh.history[1].column, 1);
      assert.equal(sh.history[1].dir, up);
      sh.selectColumnToSortOn(1);
      assert.equal(sh.history[0].column, 1);
      assert.equal(sh.history[0].dir, down);
      assert.equal(sh.history[1].column, 0);
      assert.equal(sh.history[1].dir, up);
    });
  });

  describe('encode', () => {
    it('reflects default history', () => {
      const sh = new SortHistory(columnSorters);
      assert.equal(sh.encode(), 'u0-u1');
    });

    it('reflects updated history', () => {
      const sh = new SortHistory(columnSorters);
      sh.selectColumnToSortOn(1);
      assert.equal(sh.encode(), 'd1-u0');
    });
  });

  describe('decode', () => {
    it('mirrors encode', () => {
      const sh = new SortHistory(columnSorters);
      sh.selectColumnToSortOn(1);
      const hydrated = new SortHistory(columnSorters);
      hydrated.decode(sh.encode());
      assert.equal(hydrated.encode(), sh.encode());
    });

    it('is robust to garbage input', () => {
      const hydrated = new SortHistory(columnSorters);
      hydrated.decode('u1-d0-some garbage value');
      assert.equal(hydrated.encode(), 'u0-u1', 'The decode value was invalid, so stick with the default value of SortHistory.history.');
    });
  });

  describe('compare', () => {
    // Start with a base FrontendDescription;
    const desc1: FrontendDescription = {
      Mode: 'available',
      AttachedDevice: 'nodevice',
      Annotation: {
        Message: 'Requested powercycle for "skia-e-linux-101"',
        User: 'jcgregorio@google.com',
        Timestamp: '2022-02-15T19:40:45.192013Z',
      },
      Note: {
        Message: '',
        User: '',
        Timestamp: '0001-01-01T00:00:00Z',
      },
      Version: '2022-02-09T13_06_42Z-jcgregorio-7605d80-clean',
      PowerCycle: false,
      PowerCycleState: 'available',
      LastUpdated: '2022-02-26T16:40:38.008347Z',
      Battery: 0,
      Temperature: {},
      RunningSwarmingTask: false,
      LaunchedSwarming: true,
      DeviceUptime: 0,
      SSHUserIP: '',
      Dimensions: {
        id: [
          'linux-101',
        ],
      },
    };

    // Now create desc2 based on its differences from desc1.
    const desc2 = Object.assign({}, desc1, {
      AttachedDevice: 'ios',
      PowerCycle: true,
      Battery: 100,
      Dimensions: {
        id: [
          'linux-102',
        ],
      },
    });

    // Now create desc3 based on its differences from desc1.
    const desc3 = Object.assign({}, desc1, {
      AttachedDevice: 'adb',
      Battery: 50,
      Dimensions: {
        id: [
          'linux-103',
        ],
      },
    });

    // Sort functions for different clumns, i.e. values in FrontendDescription.
    const sortByAttachDevice = (a: FrontendDescription, b: FrontendDescription): number => a.AttachedDevice.localeCompare(b.AttachedDevice);
    const sortByBattery = (a: FrontendDescription, b: FrontendDescription): number => a.Battery - b.Battery;
    const sortByPowerCycle = (a: FrontendDescription, b: FrontendDescription): number => {
      if (a.PowerCycle === b.PowerCycle) {
        return 0;
      }
      if (a.PowerCycle) {
        return 1;
      }
      return -1;
    };

    it('sorts', () => {
      const descriptions: FrontendDescription[] = [
        desc1, desc2, desc3,
      ];

      const sh = new SortHistory<FrontendDescription>([
        sortByAttachDevice,
        sortByBattery,
        sortByPowerCycle,
      ]);

      // Sort in the default order.
      descriptions.sort(sh.compare.bind(sh));
      assert.equal(sh.encode(), 'u0-u1-u2');
      assert.deepEqual(['adb', 'ios', 'nodevice'], descriptions.map((d: FrontendDescription): string => d.AttachedDevice));
      assert.deepEqual([50, 100, 0], descriptions.map((d: FrontendDescription): number => d.Battery));
      assert.deepEqual([false, true, false], descriptions.map((d: FrontendDescription): boolean => d.PowerCycle));

      // Now sort with col 1 (Battery), descending.
      sh.selectColumnToSortOn(1);
      assert.equal(sh.encode(), 'd1-u0-u2');
      descriptions.sort(sh.compare.bind(sh));
      assert.deepEqual(['ios', 'adb', 'nodevice'], descriptions.map((d: FrontendDescription): string => d.AttachedDevice));
      assert.deepEqual([100, 50, 0], descriptions.map((d: FrontendDescription): number => d.Battery));
      assert.deepEqual([true, false, false], descriptions.map((d: FrontendDescription): boolean => d.PowerCycle));

      // Now sort with col 2 (PowerCycle) descending and col 1 (Battery) ascending.
      sh.selectColumnToSortOn(1);
      sh.selectColumnToSortOn(2);
      assert.equal(sh.encode(), 'd2-u1-u0');
      descriptions.sort(sh.compare.bind(sh));
      assert.deepEqual(['ios', 'nodevice', 'adb'], descriptions.map((d: FrontendDescription): string => d.AttachedDevice));
      // This shows Battery was sorted ascending when PowerCycle values were the same.
      assert.deepEqual([100, 0, 50], descriptions.map((d: FrontendDescription): number => d.Battery));
      assert.deepEqual([true, false, false], descriptions.map((d: FrontendDescription): boolean => d.PowerCycle));
    });
  });
});
