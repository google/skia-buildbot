import './index';

import { expect } from 'chai';
import chai from 'chai';
import chaiDom from 'chai-dom';
import { ExpandableTextareaSk } from './expandable-textarea-sk';
import { CollapseSk } from '../../../elements-sk/modules/collapse-sk/collapse-sk';
import { setUpElementUnderTest } from '../test_util';
import 'chai-dom';

chai.use(chaiDom);

describe('expandable-textarea-sk', () => {
  const newInstance = setUpElementUnderTest<ExpandableTextareaSk>('expandable-textarea-sk');

  let expandableTextareaSk: ExpandableTextareaSk;
  let collapseSk: CollapseSk;
  let expanderButton: HTMLButtonElement;
  let textarea: HTMLTextAreaElement;

  beforeEach(() => {
    expandableTextareaSk = newInstance((el) => {
      el.displayText = 'Click to toggle';
    });
    collapseSk = expandableTextareaSk.querySelector<CollapseSk>('collapse-sk')!;
    expanderButton = expandableTextareaSk.querySelector('button')!;
    textarea = expandableTextareaSk.querySelector('textarea')!;
  });

  it('displays clickable text', () => {
    expect(expandableTextareaSk).to.not.have.attribute('open');
    expect(collapseSk).to.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-more-icon-sk')
      .and.not.contain('expand-less-icon-sk');
    expect(expanderButton).to.contain.text('Click to toggle');
  });

  it('expands on click with textarea in focus', () => {
    expanderButton.click();
    expect(expandableTextareaSk).to.have.attribute('open');
    expect(collapseSk).to.not.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-less-icon-sk')
      .and.not.contain('expand-more-icon-sk');
    expect(textarea).to.equal(document.activeElement);
  });

  it('collapses on second click', () => {
    expanderButton.click();
    expanderButton.click();
    expect(expandableTextareaSk).to.not.have.attribute('open');
    expect(collapseSk).to.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-more-icon-sk')
      .and.not.contain('expand-less-icon-sk');
  });

  it('reflects textarea value', () => {
    textarea.value = 'foo';
    expect(expandableTextareaSk).to.have.property('value', 'foo');
  });
});
