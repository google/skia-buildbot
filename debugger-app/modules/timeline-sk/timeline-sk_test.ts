import './index';
import { expect } from 'chai';
import { TimelineSk, TimelineSkMoveFrameEventDetail } from './timeline-sk';

import {
  setUpElementUnderTest, eventPromise, noEventPromise,
} from '../../../infra-sk/modules/test_util';

describe('timeline-sk', () => {
  const newInstance = setUpElementUnderTest<TimelineSk>('timeline-sk');

  let timelineSk: TimelineSk;
  beforeEach(() => {
    timelineSk = newInstance((el: TimelineSk) => {});
  });

  describe('playback', () => {
    it('Starts playing from last position', async () => {
      const promise1 = eventPromise<CustomEvent<TimelineSkMoveFrameEventDetail>>('move-frame', 100);
      timelineSk.item = 0;
      expect((await promise1).detail.frame).to.equal(0);
      // click play
      const promise2 = eventPromise<CustomEvent<TimelineSkMoveFrameEventDetail>>('move-frame', 100);
      const pb = timelineSk.querySelector<HTMLElement>('#play-button-v')!;
      pb.click();
      expect((await promise2).detail.frame).to.equal(1);
      // click pause
      pb.click();
      await noEventPromise('move-frame', 100);
    });
  });
});
