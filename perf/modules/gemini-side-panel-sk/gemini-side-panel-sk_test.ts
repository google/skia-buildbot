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
    await element.updateComplete;

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

  it('does nothing when input is empty', async () => {
    fetchMock.post('/_/chat', { response: 'Should not happen' });

    const input = element.shadowRoot!.querySelector('input')!;
    const sendButton = element.shadowRoot!.querySelector('send-icon-sk') as HTMLElement;

    input.value = '   ';
    input.dispatchEvent(new Event('input'));

    sendButton.click();

    await element.updateComplete;

    assert.isFalse(fetchMock.called('/_/chat'));
    const messages = element.shadowRoot!.querySelectorAll('.message');
    assert.equal(messages.length, 0);
  });

  it('sends message on Enter key', async () => {
    fetchMock.post('/_/chat', {
      response: 'Hello from backend',
    });

    const input = element.shadowRoot!.querySelector('input')!;

    input.value = 'Hello Gemini';
    input.dispatchEvent(new Event('input'));

    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));

    // Wait for async operations
    await element.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));
    await element.updateComplete;

    assert.isTrue(fetchMock.called('/_/chat'));
    const messages = element.shadowRoot!.querySelectorAll('.message');
    assert.equal(messages.length, 2);
  });

  it('clears input after sending', async () => {
    fetchMock.post('/_/chat', {
      response: 'Hello from backend',
    });

    const input = element.shadowRoot!.querySelector('input')!;
    const sendButton = element.shadowRoot!.querySelector('send-icon-sk') as HTMLElement;

    input.value = 'Hello Gemini';
    input.dispatchEvent(new Event('input'));

    sendButton.click();

    await element.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));
    await element.updateComplete;

    assert.equal(input.value, '');
  });
});
