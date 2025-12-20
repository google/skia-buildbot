import { assert } from 'chai';
import { errorMessage } from './index';

describe('errorMessage', () => {
  it('dispatches error-sk event with default duration 0', (done) => {
    const message = 'test message';
    const onErrorMessage = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      assert.equal(detail.message, message);
      assert.equal(detail.duration, 0);
      document.removeEventListener('error-sk', onErrorMessage);
      done();
    };
    document.addEventListener('error-sk', onErrorMessage);
    errorMessage(message);
  });

  it('dispatches error-sk event with provided duration', (done) => {
    const message = 'another message';
    const duration = 5000;
    const onErrorMessage = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      assert.equal(detail.message, message);
      assert.equal(detail.duration, duration);
      document.removeEventListener('error-sk', onErrorMessage);
      done();
    };
    document.addEventListener('error-sk', onErrorMessage);
    errorMessage(message, duration);
  });
});
