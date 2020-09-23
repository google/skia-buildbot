import './index';
import { CommentsSk } from './comments-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { taskspecComments } from './test_data';
import { $, $$ } from 'common-sk/modules/dom';

describe('comments-sk', () => {
  const newInstance = setUpElementUnderTest<CommentsSk>('comments-sk');

  let element: CommentsSk;
  beforeEach(() => {
    element = newInstance((el: CommentsSk) => {
      el.comments = taskspecComments;
    });
  });

  describe('comments-sk', () => {
    it('displays comments', () => {
      expect($('.comment')).to.have.length(2);
      expect($('td', element)).to.have.length(6);
      expect($('th', element)).to.have.length(3);
    });

    it('obeys allowAdd', () => {
      expect($('input-sk', element)).to.have.length(0);
      expect($('button', element)).to.have.length(0);
      element.allowAdd = true;
      expect($('input-sk', element)).to.have.length(1);
      expect($('button', element)).to.have.length(1);
    });

    it('obeys allowDelete', () => {
      expect($('delete-icon-sk', element)).to.have.length(0);
      element.allowDelete = true;
      expect($('delete-icon-sk', element)).to.have.length(2);
    });

    it('obeys showFlaky', () => {
      expect($('check-box-icon-sk', element)).to.have.length(0);
      expect($('check-box-outline-blank-icon-sk', element)).to.have.length(0);
      expect($('checkbox-sk', element)).to.have.length(0);
      element.showFlaky = true;
      expect($('check-box-icon-sk', element)).to.have.length(1);
      expect($('check-box-outline-blank-icon-sk', element)).to.have.length(1);
      element.allowAdd = true;
      expect($('checkbox-sk', element)).to.have.length(1);
    });

    it('obeys showIgnoreFailure', () => {
      expect($('check-box-icon-sk', element)).to.have.length(0);
      expect($('check-box-outline-blank-icon-sk', element)).to.have.length(0);
      expect($('checkbox-sk', element)).to.have.length(0);
      element.showIgnoreFailure = true;
      expect($('check-box-icon-sk', element)).to.have.length(2);
      expect($('check-box-outline-blank-icon-sk', element)).to.have.length(0);
      element.allowAdd = true;
      expect($('checkbox-sk', element)).to.have.length(1);
    });
  });
});
