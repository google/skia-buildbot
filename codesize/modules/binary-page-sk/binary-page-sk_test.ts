import { assert } from 'chai';
import {
  convertResponseToDataTable,
  shortenName,
} from './binary-page-sk';
import { TreeMapDataTableRow } from '../rpc_types';

describe('Binary Page Static Methods', () => {
  describe('shortenName()', () => {
    it('returns the last segment (the file name)', () => {
      const test = (name: string, input: string, expected: string) => {
        const actual = shortenName(input);
        assert.equal(actual, expected, name);
      };

      test('empty string is unchanged', '', '');
      test('no slashes means unchanged', 'alpha', 'alpha');
      test('filepath returns last segment', 'alpha/beta/gamma.cc', 'gamma.cc');
      test('function is unchanged', 'OT::OffsetTo<>::sanitize<>()', 'OT::OffsetTo<>::sanitize<>()');
      test('Symbol with a period but no slashes is unchanged', 'foo.bar()', 'foo.bar()');
    });
  });

  describe('convertResponseToDataTable()', () => {
    it('shortens file names', () => {
      const inputs: TreeMapDataTableRow[] = [
        { name: 'ROOT', parent: '', size: 0 },
        { name: 'third_party', parent: 'ROOT', size: 0 },
        { name: 'third_party/externals', parent: 'third_party', size: 0 },
        { name: 'third_party/externals/harfbuzz', parent: 'third_party/externals', size: 0 },
        { name: 'third_party/externals/harfbuzz/src', parent: 'third_party/externals/harfbuzz', size: 0 },
        { name: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', parent: 'third_party/externals/harfbuzz/src', size: 0 },
        { name: 'OT::OffsetTo<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 11221 },
        { name: 'OT::ArrayOf<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 6835 },
      ];

      const expectedRows = [
        ['Name', 'Parent', 'Size'],
        ['ROOT', '', 0],
        ['third_party', 'ROOT', 0],
        ['third_party/externals', 'third_party', 0],
        ['third_party/externals/harfbuzz', 'third_party/externals', 0],
        ['third_party/externals/harfbuzz/src', 'third_party/externals/harfbuzz', 0],
        ['hb-ot-layout.cc', 'third_party/externals/harfbuzz/src', 0], // This was shortened
        ['OT::OffsetTo<>::sanitize<>()', 'hb-ot-layout.cc', 11221], // As were these usages
        ['OT::ArrayOf<>::sanitize<>()', 'hb-ot-layout.cc', 6835],
      ];
      const actualRows = convertResponseToDataTable(inputs);
      assert.sameDeepOrderedMembers(actualRows, expectedRows);
    });

    it('does not shorten file names if there are duplicates', () => {
      const inputs: TreeMapDataTableRow[] = [
        { name: 'ROOT', parent: '', size: 0 },
        { name: 'third_party', parent: 'ROOT', size: 0 },
        { name: 'third_party/externals', parent: 'third_party', size: 0 },
        { name: 'third_party/externals/harfbuzz', parent: 'third_party/externals', size: 0 },
        { name: 'third_party/externals/harfbuzz/src', parent: 'third_party/externals/harfbuzz', size: 0 },
        { name: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', parent: 'third_party/externals/harfbuzz/src', size: 0 },
        { name: 'OT::OffsetTo<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 11221 },
        { name: 'OT::ArrayOf<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 6835 },
        { name: 'third_party/hb-ot-layout.cc', parent: 'third_party', size: 117 },
        { name: 'Merbulate()', parent: 'third_party/hb-ot-layout.cc', size: 65 },
      ];

      const expectedRows = [
        ['Name', 'Parent', 'Size'],
        ['ROOT', '', 0],
        ['third_party', 'ROOT', 0],
        ['third_party/externals', 'third_party', 0],
        ['third_party/externals/harfbuzz', 'third_party/externals', 0],
        ['third_party/externals/harfbuzz/src', 'third_party/externals/harfbuzz', 0],
        ['hb-ot-layout.cc', 'third_party/externals/harfbuzz/src', 0], // This was shortened
        ['OT::OffsetTo<>::sanitize<>()', 'hb-ot-layout.cc', 11221], // As were these usages
        ['OT::ArrayOf<>::sanitize<>()', 'hb-ot-layout.cc', 6835],
        // This file was a duplicate and not shortened
        ['third_party/hb-ot-layout.cc', 'third_party', 117],
        // neither were symbols belonging to it
        ['Merbulate()', 'third_party/hb-ot-layout.cc', 65],
      ];
      const actualRows = convertResponseToDataTable(inputs);
      assert.sameDeepOrderedMembers(actualRows, expectedRows);
    });
  });
});
