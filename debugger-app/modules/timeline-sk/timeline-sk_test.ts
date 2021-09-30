import './index';
import { expect } from 'chai';
import { TimelineSk } from './timeline-sk';

import {
  setUpElementUnderTest, eventPromise, noEventPromise,
} from '../../../infra-sk/modules/test_util';
import { MoveFrameEvent, MoveFrameEventDetail } from '../events';

describe('timeline-sk', () => {
  const newInstance = setUpElementUnderTest<TimelineSk>('timeline-sk');

  let timelineSk: TimelineSk;
  beforeEach(() => {
    timelineSk = newInstance();
  });

  describe('playback', () => {
    it('Starts playing from last position', async () => {
      const promise1 = eventPromise<CustomEvent<MoveFrameEventDetail>>(MoveFrameEvent, 100);
      timelineSk.item = 0;
      expect((await promise1).detail.frame).to.equal(0);
      // click play
      const promise2 = eventPromise<CustomEvent<MoveFrameEventDetail>>(MoveFrameEvent, 100);
      const pb = timelineSk.querySelector<HTMLElement>('#play-button-v')!;
      pb.click();
      expect((await promise2).detail.frame).to.equal(1);
      // click pause
      pb.click();
      await noEventPromise(MoveFrameEvent, 100);
    });
  });
});
