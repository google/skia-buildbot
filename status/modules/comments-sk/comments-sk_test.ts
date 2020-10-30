import './index';
import { CommentsSk } from './comments-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { taskspecComments } from './test_data';
import { $, $$ } from 'common-sk/modules/dom';
import { MockStatusService, SetupMocks } from '../rpc-mock';

describe('comments-sk', () => {
  const newInstance = setUpElementUnderTest<CommentsSk>('comments-sk');

  let element: CommentsSk;
  let clientMock: MockStatusService;
  beforeEach(async () => {
    clientMock = SetupMocks();
    element = newInstance((el: CommentsSk) => {
      el.commentData = {
        repo: 'skia',
        taskId: 'deadbeef',
        taskSpec: '',
        commit: '',
        comments: taskspecComments,
      };
      el.editRights = true;
    });
  });

  afterEach(() => {
    expect(clientMock.exhausted()).to.be.true;
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

    it('adds comments', async () => {
      element.showIgnoreFailure = true;
      element.showFlaky = true;
      element.allowAdd = true;
      clientMock.expectAddComment({}, (req) => {
        expect(req).to.deep.equal({
          repo: 'skia',
          taskId: 'deadbeef',
          taskSpec: '',
          commit: '',
          message: 'This is flaky, lets ignore it.',
          ignoreFailure: true,
          flaky: true,
        });
      });
      expect($('checkbox-sk', element)).to.have.length(2);
      ($('checkbox-sk', element)[0] as any).click();
      ($('checkbox-sk', element)[1] as any).click();
      ($$('input-sk', element) as any).value = 'This is flaky, lets ignore it.';
      const ep = eventPromise('data-update');
      ($$('button', element) as any).click();
      await ep;
      expect(element.comments).to.have.length(3);
    });

    it('deletes comments', async () => {
      element.allowDelete = true;
      clientMock.expectDeleteComment({}, (req) => {
        expect(req).to.deep.equal({
          timestamp: '2020-09-22T14:21:52.000Z',
          repo: 'skia',
          taskId: '',
          taskSpec: 'Build-Some-Stuff',
          commit: '',
        });
      });
      const ep = eventPromise('data-update');
      ($$('delete-icon-sk', element) as any).click();
      await ep;
    });
  });
});
