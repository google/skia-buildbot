import { expect } from 'chai';
import * as sinon from 'sinon';
import { CountMetric, SummaryMetric, telemetry } from './telemetry';

describe('telemetry', () => {
  const BUFFER_FLUSH_INTERVAL_MS = 5000; // 5 seconds

  let fetchStub: sinon.SinonStub;
  let setTimeoutStub: sinon.SinonStub;
  let clearTimeoutStub: sinon.SinonStub;

  beforeEach(() => {
    // Mock fetch to prevent actual network requests
    fetchStub = sinon.stub(window, 'fetch').resolves(new Response());
    // Mock setTimeout and clearTimeout to control time in tests
    setTimeoutStub = sinon.stub(window, 'setTimeout').returns(123 as any);
    clearTimeoutStub = sinon.stub(window, 'clearTimeout');

    // Reset the internal buffer and timer before each test
    telemetry._forTesting.reset();
  });

  afterEach(() => {
    sinon.restore();
    telemetry._forTesting.reset();
  });

  it('should buffer increaseCounter metrics and send them after the interval', async () => {
    telemetry.increaseCounter(CountMetric.DataFetchFailure, { test: 'tag1' });
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { test: 'tag2' });

    // Expect fetch not to have been called immediately
    expect(fetchStub.callCount).to.equal(0);
    // Expect setTimeout to have been called to schedule the flush
    expect(setTimeoutStub.callCount).to.equal(1);
    expect(setTimeoutStub.getCall(0).args[1]).to.equal(BUFFER_FLUSH_INTERVAL_MS);

    // Manually trigger the setTimeout callback
    setTimeoutStub.getCall(0).args[0]();

    // Expect fetch to have been called with the buffered metrics
    expect(fetchStub.callCount).to.equal(1);
    const expectedBody = JSON.stringify([
      {
        metric_name: CountMetric.DataFetchFailure,
        metric_value: 1,
        tags: { test: 'tag1' },
        metric_type: 'counter',
      },
      {
        metric_name: CountMetric.TriageActionTaken,
        metric_value: 1,
        tags: { test: 'tag2' },
        metric_type: 'counter',
      },
    ]);
    expect(fetchStub.getCall(0).args[0]).to.equal('/_/fe_telemetry');
    expect(fetchStub.getCall(0).args[1]).to.deep.equal({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expectedBody,
    });
  });

  it('should buffer recordSummary metrics and send them after the interval', async () => {
    telemetry.recordSummary(SummaryMetric.GoogleGraphPlotTime, 100, { test: 'tag3' });
    telemetry.recordSummary(SummaryMetric.MultiGraphDataLoadTime, 200, { test: 'tag4' });

    expect(fetchStub.callCount).to.equal(0);
    expect(setTimeoutStub.callCount).to.equal(1);
    expect(setTimeoutStub.getCall(0).args[1]).to.equal(BUFFER_FLUSH_INTERVAL_MS);

    setTimeoutStub.getCall(0).args[0]();

    expect(fetchStub.callCount).to.equal(1);
    const expectedBody = JSON.stringify([
      {
        metric_name: SummaryMetric.GoogleGraphPlotTime,
        metric_value: 100,
        tags: { test: 'tag3' },
        metric_type: 'summary',
      },
      {
        metric_name: SummaryMetric.MultiGraphDataLoadTime,
        metric_value: 200,
        tags: { test: 'tag4' },
        metric_type: 'summary',
      },
    ]);
    expect(fetchStub.getCall(0).args[0]).to.equal('/_/fe_telemetry');
    expect(fetchStub.getCall(0).args[1]).to.deep.equal({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expectedBody,
    });
  });

  it('should send buffered metrics when document visibility changes to hidden', async () => {
    telemetry.increaseCounter(CountMetric.DataFetchFailure);
    expect(fetchStub.callCount).to.equal(0);
    expect(setTimeoutStub.callCount).to.equal(1); // Timer started

    // Simulate visibility change to hidden
    Object.defineProperty(document, 'visibilityState', {
      value: 'hidden',
      writable: true,
    });
    document.dispatchEvent(new Event('visibilitychange'));

    // Expect clearTimeout to be called
    expect(clearTimeoutStub.callCount).to.equal(1);
    expect(clearTimeoutStub.getCall(0).args[0]).to.equal(123); // 123 is the mocked timerId

    // Expect fetch to have been called immediately
    expect(fetchStub.callCount).to.equal(1);
    const expectedBody = JSON.stringify([
      {
        metric_name: CountMetric.DataFetchFailure,
        metric_value: 1,
        tags: {},
        metric_type: 'counter',
      },
    ]);
    expect(fetchStub.getCall(0).args[0]).to.equal('/_/fe_telemetry');
    expect(fetchStub.getCall(0).args[1]).to.deep.equal({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expectedBody,
    });
  });

  it('should implement FIFO behavior when buffer is full', async () => {
    const MAX_BUFFER_SIZE = telemetry._forTesting.MAX_BUFFER_SIZE;
    // Fill the buffer to its maximum capacity
    for (let i = 0; i < MAX_BUFFER_SIZE; i++) {
      telemetry.increaseCounter(CountMetric.DataFetchFailure, { id: `metric_${i}` });
    }
    expect(telemetry._forTesting.getBuffer().length).to.equal(MAX_BUFFER_SIZE);

    // Add one more metric, which should trigger FIFO behavior
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { id: 'new_metric' });

    // Expect the buffer size to remain the same
    expect(telemetry._forTesting.getBuffer().length).to.equal(MAX_BUFFER_SIZE);

    // Expect the oldest metric (id: metric_0) to be removed
    const buffer = telemetry._forTesting.getBuffer();

    // Expect the second oldest metric (id: metric_1) to now be the oldest
    expect(buffer[0].tags.id).to.equal('metric_1');
  });

  it('should not send metrics if buffer is empty', async () => {
    telemetry.increaseCounter(CountMetric.DataFetchFailure);
    // Clear the buffer, but the timer is still scheduled.
    telemetry._forTesting.getBuffer().length = 0;
    // Trigger the timer.
    setTimeoutStub.getCall(0).args[0]();

    // Expect fetch not to have been called because the buffer was empty.
    expect(fetchStub.callCount).to.equal(0);
  });
});
