import './index';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { EditIgnoreRuleSk } from './edit-ignore-rule-sk';
import { EditIgnoreRuleSkPO } from './edit-ignore-rule-sk_po';
import { expect } from 'chai';

describe('edit-ignore-rule-sk', () => {
  const newInstance = setUpElementUnderTest<EditIgnoreRuleSk>('edit-ignore-rule-sk');

  // This date is arbitrary
  const fakeNow = Date.parse('2020-02-01T00:00:00Z');
  const regularNow = Date.now;

  let editIgnoreRuleSk: EditIgnoreRuleSk;
  let editIgnoreRuleSkPO: EditIgnoreRuleSkPO;

  beforeEach(() => {
    editIgnoreRuleSk = newInstance();
    // All tests will have the paramset loaded.
    editIgnoreRuleSk.paramset = {
      alpha_type: ['Opaque', 'Premul'],
      arch: ['arm', 'arm64', 'x86', 'x86_64'],
    };
    Date.now = () => fakeNow;
    editIgnoreRuleSkPO = new EditIgnoreRuleSkPO(editIgnoreRuleSk);
  });

  afterEach(() => {
    Date.now = regularNow;
  });

  describe('inputs and outputs', () => {
    it('has no query, note or expires', () => {
      expect(editIgnoreRuleSk.query).to.equal('');
      expect(editIgnoreRuleSk.note).to.equal('');
      expect(editIgnoreRuleSk.expires).to.equal('');
    });

    it('reflects typed in values', async () => {
      await editIgnoreRuleSkPO.setExpires('2w');
      await editIgnoreRuleSkPO.setNote('this is a bug');
      expect(editIgnoreRuleSk.expires).to.equal('2w');
      expect(editIgnoreRuleSk.note).to.equal('this is a bug');
    });

    it('reflects interactions with the query-sk element', async () => {
      const querySkPO = await editIgnoreRuleSkPO.getQuerySkPO();
      await querySkPO.clickKey('alpha_type');
      await querySkPO.clickValue('Opaque');
      expect(editIgnoreRuleSk.query).to.equal('alpha_type=Opaque');
    });

    it('converts future dates to human readable durations', () => {
      editIgnoreRuleSk.expires = '2020-02-07T06:00:00Z';
      // It is ok that the 6 hours gets rounded out.
      expect(editIgnoreRuleSk.expires).to.equal('6d');
    });

    it('converts past or invalid dates to nothing (requiring them to be re-input)', () => {
      editIgnoreRuleSk.expires = '2020-01-07T06:00:00Z';
      expect(editIgnoreRuleSk.expires).to.equal('');
      editIgnoreRuleSk.expires = 'invalid date';
      expect(editIgnoreRuleSk.expires).to.equal('');
    });

    it('can add a custom key and value', async () => {
      editIgnoreRuleSk.query = 'arch=arm64';

      // Add a new value to an existing param
      await editIgnoreRuleSkPO.setCustomKey('arch');
      await editIgnoreRuleSkPO.setCustomValue('y75');
      await editIgnoreRuleSkPO.clickAddCustomParamBtn();

      // add a brand new key and value
      await editIgnoreRuleSkPO.setCustomKey('custom');
      await editIgnoreRuleSkPO.setCustomValue('value');
      await editIgnoreRuleSkPO.clickAddCustomParamBtn();

      expect(editIgnoreRuleSk.query).to.equal('arch=arm64&arch=y75&custom=value');
      // ParamSet should be mutated to have the new values
      expect(editIgnoreRuleSk.paramset.arch)
          .to.deep.equal(['arm', 'arm64', 'x86', 'x86_64', 'y75']);
      expect(editIgnoreRuleSk.paramset.custom).to.deep.equal(['value']);
    });
  });

  describe('validation', () => {
    it('has the error msg hidden by default', async () => {
      expect(await editIgnoreRuleSkPO.isErrorMessageVisible()).to.be.false;
    });

    it('does not validate when query is empty', async () => {
      editIgnoreRuleSk.query = '';
      editIgnoreRuleSk.expires = '2w';
      expect(editIgnoreRuleSk.verifyFields()).to.be.false;
      expect(await editIgnoreRuleSkPO.isErrorMessageVisible()).to.be.true;
    });

    it('does not validate when expires is empty', async () => {
      editIgnoreRuleSk.query = 'alpha_type=Opaque';
      editIgnoreRuleSk.expires = '';
      expect(editIgnoreRuleSk.verifyFields()).to.be.false;
      expect(await editIgnoreRuleSkPO.isErrorMessageVisible()).to.be.true;
    });

    it('does not validate when both expires and query are empty', async () => {
      editIgnoreRuleSk.query = '';
      editIgnoreRuleSk.expires = '';
      expect(editIgnoreRuleSk.verifyFields()).to.be.false;
      expect(await editIgnoreRuleSkPO.isErrorMessageVisible()).to.be.true;
    });

    it('does passes validation when both expires and query are set', async () => {
      editIgnoreRuleSk.query = 'foo=bar';
      await editIgnoreRuleSkPO.setExpires('1w');
      expect(editIgnoreRuleSk.verifyFields()).to.be.true;
      expect(await editIgnoreRuleSkPO.isErrorMessageVisible()).to.be.false;
    });

    it('requires both a custom key and value', async () => {
      expect(editIgnoreRuleSk.query).to.equal('');

      await editIgnoreRuleSkPO.setCustomKey('');
      await editIgnoreRuleSkPO.setCustomValue('');
      await editIgnoreRuleSkPO.clickAddCustomParamBtn();

      expect(await editIgnoreRuleSkPO.getErrorMessage()).to.contain('both a key and a value');
      expect(editIgnoreRuleSk.query).to.equal('');

      await editIgnoreRuleSkPO.setCustomKey('custom');
      await editIgnoreRuleSkPO.setCustomValue('');
      await editIgnoreRuleSkPO.clickAddCustomParamBtn();

      expect(await editIgnoreRuleSkPO.getErrorMessage()).to.contain('both a key and a value');
      expect(editIgnoreRuleSk.query).to.equal('');

      await editIgnoreRuleSkPO.setCustomKey('');
      await editIgnoreRuleSkPO.setCustomValue('value');
      await editIgnoreRuleSkPO.clickAddCustomParamBtn();

      expect(await editIgnoreRuleSkPO.getErrorMessage()).to.contain('both a key and a value');
      expect(editIgnoreRuleSk.query).to.equal('');
    });
  });
});
