import { _convertToArray } from './word-cloud-sk.js';

describe('word-cloud-sk', function() {
  describe('_convertToArray', function() {
    it('converts Objects to Arrays in the form needed for items', function() {
      return window.customElements.whenDefined('triage2-sk').then(() => {
        let from = {
          "config": [
            {value:"565", weight: 20},
            {value:"8888", weight: 11},
          ],
          "cpu_or_gpu": [
            {value:"cpu", weight: 24},
            {value:"gpu", weight: 8},
          ]
        };


        let expected = [
          {
            name: "cpu_or_gpu",
            values: [
              {value:"cpu", weight: 24},
              {value:"gpu", weight: 8},
            ],
          },
          {
            name: "config",
            values: [
              {value:"565", weight: 20},
              {value:"8888", weight: 11},
            ],
          },
        ];

        assert.equal(JSON.stringify(expected, null, '  '), JSON.stringify(_convertToArray(from), null, '  '))
      });
    });
	});
});
