import './index.js';
import { setUpElementUnderTest } from '../test_util';
import { $$ } from 'common-sk/modules/dom';

import { manyParams } from './test_data';

describe('edit-ignore-rule-sk', () => {
  const newInstance = setUpElementUnderTest('edit-ignore-rule-sk');

  // This date is arbitrary
  const fakeNow = Date.parse('2020-02-01T00:00:00Z');
  const regularNow = Date.now;
  let editor;
  beforeEach(() => {
    editor = newInstance();
    // All tests will have the params loaded.
    editor.params = manyParams;
    Date.now = () => fakeNow;
  });

  afterEach(() => {
    Date.now = regularNow;
  });

  describe('Inputs and outputs', () => {
    it('has no query, note or expires', () => {
      expect(editor.query).to.equal('');
      expect(editor.note).to.equal('');
      expect(editor.expires).to.equal('');
    });

    it('reflects typed in values', () => {
      getExpiresInput(editor).value = '2w';
      getNoteInput(editor).value = 'this is a bug';
      expect(editor.expires).to.equal('2w');
      expect(editor.note).to.equal('this is a bug');
    });

    it('reflects interactions with the query-sk element', () => {
      // Select alpha_type key, which displays Opaque and Premul as values.
      $$('query-sk .selection div:nth-child(1)').click();
      // Select Opaque as a value
      $$('query-sk #values div:nth-child(1)').click();
      expect(editor.query).to.equal('alpha_type=Opaque');
    });

    it('converts future dates to human readable durations', () => {
      editor.expires = '2020-02-07T06:00:00Z';
      // It is ok that the 6 hours gets rounded out.
      expect(editor.expires).to.equal('6d');
    });

    it('converts past or invalid dates to nothing (requiring them to be re-input)', () => {
      editor.expires = '2020-01-07T06:00:00Z';
      expect(editor.expires).to.equal('');
      editor.expires = 'invalid date';
      expect(editor.expires).to.equal('');
    });
  });

  describe('validation', () => {
    it('has the error msg hidden by default', () => {
      expect(getErrorMessage(editor).hasAttribute('hidden')).to.be.true;
    });

    it('does not validate when both expires and query are empty', () => {
      editor.query = '';
      editor.expires = '';
      expect(editor.verifyFields()).to.be.false;
      expect(getErrorMessage(editor).hasAttribute('hidden')).to.be.false;
    });

    it('does passes validation when both expires and query are set', () => {
      editor.query = 'foo=bar';
      getExpiresInput(editor).value = '1w';
      expect(editor.verifyFields()).to.be.true;
      expect(getErrorMessage(editor).hasAttribute('hidden')).to.be.true;
    });
  });
});

const getExpiresInput = (ele) => $$('#expires', ele);

const getNoteInput = (ele) => $$('#note', ele);

const getErrorMessage = (ele) => $$('.error', ele);
