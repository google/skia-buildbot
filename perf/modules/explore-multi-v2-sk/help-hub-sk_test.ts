import './help-hub-sk';
import { assert } from 'chai';
import { HelpHubSk } from './help-hub-sk';

describe('help-hub-sk', () => {
  let element: HelpHubSk;

  beforeEach(async () => {
    element = document.createElement('help-hub-sk') as HelpHubSk;
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('renders a floating FAB button by default and drawer is closed', async () => {
    await element.updateComplete;
    const fab = element.shadowRoot!.querySelector('.help-fab');
    assert.isNotNull(fab);
    const panel = element.shadowRoot!.querySelector('.help-panel');
    assert.isFalse(panel?.classList.contains('open'));
  });

  it('opens panel drawer when clicking the FAB button', async () => {
    await element.updateComplete;
    const fab = element.shadowRoot!.querySelector('.help-fab') as HTMLButtonElement;
    fab.click();
    await element.updateComplete;

    const panel = element.shadowRoot!.querySelector('.help-panel');
    assert.isTrue(panel?.classList.contains('open'));
  });

  it('dispatches start-tour event when tour button clicked', async () => {
    await element.updateComplete;
    element.openPanel(); // utility helper to open
    await element.updateComplete;

    let tourStarted = false;
    element.addEventListener('start-tour', () => {
      tourStarted = true;
    });

    const tourBtn = element.shadowRoot!.querySelector('.tour-trigger-btn') as HTMLButtonElement;
    assert.isNotNull(tourBtn);
    tourBtn.click();

    assert.isTrue(tourStarted);
  });
});
