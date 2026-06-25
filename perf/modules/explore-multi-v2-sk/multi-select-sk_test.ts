import './multi-select-sk';
import { MultiSelectSk } from './multi-select-sk';
import { expect } from 'chai';

describe('multi-select-sk', () => {
  let element: MultiSelectSk;

  beforeEach(async () => {
    element = document.createElement('explore-multi-v2-select-sk') as unknown as MultiSelectSk;
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('uses native input for the search input', async () => {
    element.isOpen = true;
    await element.updateComplete;
    const input = element.shadowRoot!.querySelectorAll('input[placeholder="Search..."]');
    expect(input.length).to.be.greaterThan(0);
  });

  it('emits diff-base event when Diff button is clicked', async () => {
    element.label = 'test-key';
    element.options = [{ value: 'value1', count: 10 }];
    element.showDiffButton = true;
    element.isOpen = true;
    await element.updateComplete;

    let eventDetail: any = null;
    element.addEventListener('diff-base', (e: any) => {
      eventDetail = e.detail;
    });

    const diffBtn = element.shadowRoot!.querySelector('.ms-diff-btn') as HTMLElement;
    expect(diffBtn).to.not.be.null;
    diffBtn.click();

    expect(eventDetail).to.deep.equal({ key: 'test-key', value: 'value1' });
  });

  it('emits split event when Split button is clicked', async () => {
    element.showSplitButton = true;
    element.isOpen = true;
    await element.updateComplete;

    let eventEmitted = false;
    element.addEventListener('split', () => {
      eventEmitted = true;
    });

    const splitBtn = element.shadowRoot!.querySelector('.ms-split-btn') as HTMLElement;
    expect(splitBtn).to.not.be.null;
    splitBtn.click();

    expect(eventEmitted).to.be.true;
  });

  it('toggles selection when option is clicked', async () => {
    element.options = [{ value: 'value1', count: 10 }];
    element.isOpen = true;
    await element.updateComplete;

    let eventDetail: any = null;
    element.addEventListener('selection-change', (e: any) => {
      eventDetail = e.detail;
    });

    const optionDiv = element.shadowRoot!.querySelector('.multiselect-option') as HTMLElement;
    expect(optionDiv).to.not.be.null;
    optionDiv.click();

    expect(eventDetail).to.deep.equal({ value: 'value1' });
  });

  it('toggles selection when checkmark is clicked', async () => {
    element.options = [{ value: 'value1', count: 10 }];
    element.isOpen = true;
    await element.updateComplete;

    let eventDetail: any = null;
    element.addEventListener('selection-change', (e: any) => {
      eventDetail = e.detail;
    });

    const checkmark = element.shadowRoot!.querySelector('.checkmark') as HTMLElement;
    expect(checkmark).to.not.be.null;
    checkmark.click();

    expect(eventDetail).to.deep.equal({ value: 'value1' });
  });

  it('uses CSS variable for pill border and color', async () => {
    element.variant = 'pill';
    await element.updateComplete;

    element.style.setProperty('--md-sys-color-outline', 'rgb(200, 200, 200)');
    element.style.setProperty('--on-surface', 'rgb(32, 33, 36)');
    await element.updateComplete;

    const trigger = element.shadowRoot!.querySelector('.multiselect-trigger.pill') as HTMLElement;
    expect(trigger).to.not.be.null;
    const style = window.getComputedStyle(trigger);

    const borderColor = style.borderColor;
    expect(borderColor).to.include('rgb(200, 200, 200)');

    const textColor = style.color;
    expect(textColor).to.equal('rgb(32, 33, 36)');
  });
});
