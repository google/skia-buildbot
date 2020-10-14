import './index';
import { PlaySk, PlaySkMoveToEventDetail } from './play-sk';

import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('play-sk', () => {
  const newInstance = setUpElementUnderTest<PlaySk>('play-sk');

  let play: PlaySk;
  beforeEach(() => {
    play = newInstance((el: PlaySk) => {
      // Put player in well-defined state.
      el.mode = 'pause';
      el.size = 10; // a sequence of 10 items
      el.playbackDelay = 100; // ms
      el.movedTo(3); // start on item 3
    });
  });
  // Number of ms to wait for moveto events to be produced.
  // must exceed el.playbackDelay above, but not by too much.
  const delay = 120;

  describe('Events', () => {
    it('emits moveto when playing', async () => {
      let ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play';
      expect((await ep).detail.item).to.equal(4);
      // Expect it doesn't happen again, because we haven't called movedTo yet.
      await noEventPromise('moveto', delay);
      ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.movedTo(4);
      // now expect it asks to play the next thing in 100 ms.
      expect((await ep).detail.item).to.equal(5);
    });

    // State 1 being right after it emits moveto, but before the app calls movedTo
    it('Does not emit moveto after paused in state 1', async () => {
      const ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play';
      await ep;
      // now in state 1 indefinitely.
      play.mode = 'pause';
      await noEventPromise('moveto', delay);
    });

    // State 2 being right after the app calls movedTo but it's sitting out it's internal delay.
    it('Does not emit moveto after paused in state 2', async () => {
      const ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play';
      const item = (await ep).detail.item;
      play.movedTo(item);
      // now in state 2 for 100 ms.
      play.mode = 'pause';
      await noEventPromise('moveto', delay);
    });

    it('Continues after skipping to an arbitrary position while playing in state 1', async () => {
      let ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play';
      await ep;
      ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.movedTo(8);
      // expect it to emit moveto 9
      expect((await ep).detail.item).to.equal(9);
    });

    it('Continues after skipping to an arbitrary position while playing in state 2', async () => {
      let ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play';
      const item = (await ep).detail.item;
      play.movedTo(item);
      // now in state 2 for 100 ms
      ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.movedTo(8);
      // expect it to emit moveto 9
      expect((await ep).detail.item).to.equal(9);
    });

    it('Plays after skipping to an arbitrary position while paused', async () => {
      play.mode = 'pause';
      play.movedTo(8);
      // expect jumps don't start playback
      await noEventPromise('moveto', delay);
      // expect it to emit moveto 9
      const ep = eventPromise<CustomEvent<PlaySkMoveToEventDetail>>('moveto', delay);
      play.mode = 'play'
      expect((await ep).detail.item).to.equal(9);
    });

  });
});
