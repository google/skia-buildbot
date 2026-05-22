import './interactive-tour-sk';
import { assert } from 'chai';
import { InteractiveTourSk, TourStep } from './interactive-tour-sk';

describe('interactive-tour-sk', () => {
  let element: InteractiveTourSk;
  const mockSteps: TourStep[] = [
    { selector: '#target1', title: 'Step 1', text: 'Hello 1', placement: 'bottom' },
    { selector: '#target2', title: 'Step 2', text: 'Hello 2', placement: 'top' },
  ];

  beforeEach(() => {
    element = document.createElement('interactive-tour-sk') as InteractiveTourSk;
    document.body.appendChild(element);
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('is hidden by default', () => {
    assert.isFalse(element.active);
    const overlay = element.shadowRoot!.querySelector('.tour-overlay');
    assert.isNull(overlay);
  });

  it('shows up when active property is true and starts at index 0', async () => {
    element.steps = mockSteps;
    element.active = true;
    await element.updateComplete;

    const overlay = element.shadowRoot!.querySelector('.tour-overlay');
    assert.isNotNull(overlay);

    const title = element.shadowRoot!.querySelector('.bubble-title');
    assert.equal(title?.textContent?.trim(), 'Step 1');
  });

  it('fires tour-finished event when skip or finish is clicked', async () => {
    element.steps = mockSteps;
    element.active = true;
    await element.updateComplete;

    let finishedFired = false;
    element.addEventListener('tour-finished', () => {
      finishedFired = true;
    });

    const skipButton = element.shadowRoot!.querySelector('.tour-btn-skip') as HTMLButtonElement;
    skipButton.click();

    assert.isTrue(finishedFired);
  });
});
