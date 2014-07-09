describe('Confirm our polyfill of Object Observe works.',
  function() {
    beforeEach(function() { });
    afterEach(function() { });

    function testSimpleOO() {
      // Let's say we have a model with data
      var model = {};

      var observations = [];

      var observer = new ObjectObserver(model);
      // Which we then observe
      observer.open(function(added, removed, changed, getOldValueFn){
        Object.keys(added).forEach(function(change) {
          observations.push(change);
        });
      });
      model["foo"] = "bar";
      assert.equal(0, observations.length, "Observations should have been added.")

      Platform.performMicrotaskCheckpoint()

      assert.equal(1, observations.length, "Observations should have been added.")
    }


    it('should be able to use ObjectObserver', function() {
      testSimpleOO();
    });
  }
);
