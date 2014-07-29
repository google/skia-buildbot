// A simple JS unit test to demonstrate the system.
//
// Note that you can run this test simply by running
//
//   ./node_modules/.bin/grunt test
//
// But if you want to have Karma run continuously and rerun
// tests when it sees file change then you can run Karma
// directly and pass in the --no-single-run flag:
//
//  ./node_modules/.bin/karma start --no-single-run
//
describe('Test our implementation of $$',
  function() {
    beforeEach(function() { });
    afterEach(function() { });

    // Add some HTML to the page we need for testing purposes.
    var div = document.createElement('div');
    div.innerHTML = '<p class=foo></p><p id=bar></p>';
    document.body.appendChild(div);

    function testFindByClass() {
      // Assertions are handled by the Chai Assertion Library.
      // Documentation: http://chaijs.com/api/assert/
      assert.equal(skiaperf.$$('.foo').length, 1, 'Can search by class name.')
    }

    function testFindByName() {
      assert.equal(skiaperf.$$('p').length, 2, 'Can search by element name.')
    }

    function testFindByID() {
      assert.equal(skiaperf.$$('#bar').length, 1, 'Can search by element id.')
    }

    // Bundle all tests under an 'it' call, which is part of Mocha.
    it('should be able to query for elements', function() {
      testFindByClass();
      testFindByName();
      testFindByID();
    });
  }
);
