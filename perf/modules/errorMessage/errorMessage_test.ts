import { errorMessage, logErrorMessage } from './index';
import { assert } from 'chai';
import sinon from 'sinon';
import { telemetry, CountMetric } from '../telemetry/telemetry';

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

describe('errorMessageWithTelemetry', () => {
  let increaseCounterStub: sinon.SinonStub;

  beforeEach(() => {
    increaseCounterStub = sinon.stub(telemetry, 'increaseCounter');
  });

  afterEach(() => {
    increaseCounterStub.restore();
  });

  it('dispatches error-sk event with default duration 0', (done) => {
    const message = 'telemetry message';
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

  it('extracts errorCode from Response object', () => {
    const resp = new Response('error', { status: 404, statusText: 'Not Found' });
    errorMessage({ resp: resp }, 0, {
      countMetricSource: CountMetric.FrontendErrorReported,
    });

    assert.isTrue(
      increaseCounterStub.calledWith(CountMetric.FrontendErrorReported, {
        source: 'default',
        errorCode: '404',
      })
    );
  });

  it('uses provided errorCode even if Response is present', () => {
    const resp = new Response('error', { status: 404, statusText: 'Not Found' });
    errorMessage({ resp: resp }, 0, {
      countMetricSource: CountMetric.FrontendErrorReported,
      errorCode: 'CUSTOM_ERROR',
    });

    assert.isTrue(
      increaseCounterStub.calledWith(CountMetric.FrontendErrorReported, {
        source: 'default',
        errorCode: 'CUSTOM_ERROR',
      })
    );
  });
});

describe('logErrorMessage', () => {
  let logErrorStub: sinon.SinonStub;

  beforeEach(() => {
    logErrorStub = sinon.stub(telemetry, 'reportErrorToServer');
  });

  afterEach(() => {
    logErrorStub.restore();
  });

  it('calls reportErrorToServer with message string', () => {
    const errorBody = 'test error';
    const errorSource = 'test source';
    logErrorMessage(errorBody, errorSource);
    assert.isTrue(logErrorStub.calledWith(errorBody, errorSource));
  });

  it('calls reportErrorToServer with object containing message', () => {
    const errorObj = { message: 'object error' };
    const errorSource = 'test source';

    logErrorMessage(errorObj, errorSource);
    assert.isTrue(logErrorStub.calledWith(errorObj.message, errorSource));
  });

  it('calls reportErrorToServer with response object (statusText)', () => {
    const errorBody = {
      resp: new Response(null, { statusText: 'Not Found' }),
    };
    const errorSource = 'test source';
    logErrorMessage(errorBody, errorSource);
    assert.isTrue(logErrorStub.calledWith('Not Found', errorSource));
  });
});
