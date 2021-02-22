import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { UniformTimeSk } from './uniform-time-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-time-sk', () => {
  const newInstance = setUpElementUnderTest<UniformTimeSk>('uniform-time-sk');

  let element: UniformTimeSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('unform-time-sk', () => {
    it('throws on invalid uniform', () => {
      assert.throws(() => {
        element.uniform = {
          name: 'iTime',
          rows: 2,
          columns: 2,
          slot: 1,
        };
      });
    });

    it('starts in play mode', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s
      element.dateNow = () => 200; // ms

      // The reported time should advance 0.2 seconds.
      assert.equal(10.2, element.time);
    });

    it('toggles to pause mode', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s
      // Switch to pause mode.
      $$<HTMLButtonElement>('#playpause', element)!.click();
      element.dateNow = () => 200; // ms

      // Even though dateNow advanced the reported time should not change.
      assert.equal(10, element.time);
    });

    it('goes to zero on restart while playing', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s
      $$<HTMLButtonElement>('#restart', element)!.click();
      assert.equal(0, element.time);
    });

    it('goes to zero on restart while paused', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s
      // Switch to pause mode.
      $$<HTMLButtonElement>('#playpause', element)!.click();
      element.dateNow = () => 200; // ms

      // Even though dateNow advanced the reported time should not change.
      assert.equal(10, element.time);

      // The time goes to zero even though we are paused.
      $$<HTMLButtonElement>('#restart', element)!.click();
      assert.equal(0, element.time);
    });

    it('applied uniforms correctly', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s

      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = [0, 0, 0];

      // The control defaults to a value of 0.5.
      element.uniform = {
        name: 'iTime',
        columns: 1,
        rows: 1,
        slot: 1,
      };
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 10, 0]);
    });

    it('needs raf updates', () => {
      assert.isTrue(element.needsRAF());
    });

    it('updates on a call to onRAF', () => {
      element.dateNow = () => 0; // ms
      element.time = 10; // s
      element.onRAF();
      assert.equal('10.000', $$('#ms', element)?.textContent);
    });
  });
});
