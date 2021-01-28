import { expect } from 'chai';
import * as paramset from './index';

/* eslint-disable no-unused-expressions */

describe('ParamSet',
  () => {
    function testParamSet() {
      const ps = {};
      // The empty paramset matches everything.
      expect(paramset.match(ps, {})).to.be.true;
      expect(paramset.match(ps, { foo: '2', bar: 'a' })).to.be.true;

      const p = {
        foo: '1',
        bar: 'a',
      };
      paramset.add(ps, p);
      expect(paramset.match(ps, p)).to.be.true;
      expect(paramset.match(ps, {})).to.be.false;

      paramset.add(ps, {
        foo: '2',
        bar: 'b',
      });

      paramset.add(ps, {
        foo: '1',
        bar: 'b',
      });

      expect(paramset.match(ps, { foo: '2', bar: 'a' })).to.be.true;
      expect(paramset.match(ps, { foo: '2', bar: 'a', baz: 'other' })).to.be.true;
      expect(paramset.match(ps, { bar: 'a' })).to.be.false;
      expect(paramset.match(ps, { foo: '2' })).to.be.false;
      expect(paramset.match(ps, { })).to.be.false;
      expect(paramset.match(ps, { foo: '3', bar: 'a' })).to.be.false;
      expect(paramset.match(ps, { foo: '2', bar: 'c' })).to.be.false;
    }

    function testParamSetWithIgnore() {
      const ps = {};
      const p = {
        foo: '1',
        bar: 'a',
        description: 'long rambling text',
      };
      paramset.add(ps, p, ['description']);
      expect(paramset.match(ps, p)).to.be.true;
      expect(paramset.match(ps, {
        foo: '1',
        bar: 'a',
      })).to.be.true;
      expect(paramset.match(ps, {})).to.be.false;
    }

    function testParamSetWithRegex() {
      const ps = {};
      paramset.add(ps, { foo: '2.*' });
      expect(paramset.match(ps, { foo: '2345', bar: 'aa' })).to.be.true;
      expect(paramset.match(ps, { foo: '345', bar: 'bb' })).to.be.false;
      paramset.add(ps, { bar: '.*a' });
      expect(paramset.match(ps, { foo: '2345', bar: 'aa' })).to.be.true;
      expect(paramset.match(ps, { foo: '2345', bar: 'bb' })).to.be.false;

      // Test with paramset with both regex and non-regex by adding another
      // value to existing key.
      paramset.add(ps, { foo: 'blah' });
      expect(paramset.match(ps, { foo: '2345', bar: 'aa' })).to.be.true;
      expect(paramset.match(ps, { foo: 'blah', bar: 'aa' })).to.be.true;
      expect(paramset.match(ps, { foo: '345', bar: 'aa' })).to.be.false;
      expect(paramset.match(ps, { foo: '2345', bar: 'bb' })).to.be.false;
    }

    it('should be able get match against params', () => {
      testParamSet();
      testParamSetWithIgnore();
      testParamSetWithRegex();
    });
  });
