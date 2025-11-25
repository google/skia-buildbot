import './gemini-side-panel-sk';
import { GeminiSidePanelSk } from './gemini-side-panel-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';

describe('gemini-side-panel-sk', () => {
  const newInstance = setUpElementUnderTest<GeminiSidePanelSk>('gemini-side-panel-sk');

  let element: GeminiSidePanelSk;

  beforeEach(async () => {
    element = newInstance();
    await element.updateComplete;
    fetchMock.reset();
  });

  afterEach(() => {
    fetchMock.reset();
  });

  it('starts closed', () => {
    assert.isFalse(element.open);
    assert.equal(getComputedStyle(element).right, '-400px');
  });

  it('opens when open property is set', async () => {
    element.open = true;
    await element.updateComplete;
    assert.equal(getComputedStyle(element).right, '0px');
  });

  it('toggles open state', async () => {
    element.toggle();
    await element.updateComplete;
    assert.isTrue(element.open);

    element.toggle();
    await element.updateComplete;
    assert.isFalse(element.open);
  });

  it('sends a message and displays response', async () => {
    fetchMock.post('/_/chat', {
      response: 'Hello from backend',
    });

    const input = element.shadowRoot!.querySelector('input')!;
    const sendButton = element.shadowRoot!.querySelector('send-icon-sk') as HTMLElement;

    input.value = 'Hello Gemini';
    input.dispatchEvent(new Event('input'));

    sendButton.click();

    // Wait for async operations
    await element.updateComplete;
    // Wait for fetch to resolve (microtasks)
    await new Promise((resolve) => setTimeout(resolve, 0));
    await element.updateComplete;

    const messages = element.shadowRoot!.querySelectorAll('.message');
    assert.equal(messages.length, 2);
    assert.include(messages[0].textContent, 'Hello Gemini');
    assert.include(messages[0].className, 'user');
    assert.include(messages[1].textContent, 'Hello from backend');
    assert.include(messages[1].className, 'model');
  });

  it('handles error response', async () => {
    fetchMock.post('/_/chat', 500);

    const input = element.shadowRoot!.querySelector('input')!;
    const sendButton = element.shadowRoot!.querySelector('send-icon-sk') as HTMLElement;

    input.value = 'Crash me';
    input.dispatchEvent(new Event('input'));

    sendButton.click();

    await element.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));
    await element.updateComplete;

    const messages = element.shadowRoot!.querySelectorAll('.message');
    assert.equal(messages.length, 2);
    assert.include(messages[1].textContent, 'Error');
  });

  it('handles network error', async () => {
    fetchMock.post('/_/chat', { throws: new Error('Network error') });

    const input = element.shadowRoot!.querySelector('input')!;
    const sendButton = element.shadowRoot!.querySelector('send-icon-sk') as HTMLElement;

    input.value = 'Network fail';
    input.dispatchEvent(new Event('input'));

    sendButton.click();

    await element.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));
    await element.updateComplete;

    const messages = element.shadowRoot!.querySelectorAll('.message');
    assert.equal(messages.length, 2);
    assert.include(messages[1].textContent, 'Error sending message');
  });
});
