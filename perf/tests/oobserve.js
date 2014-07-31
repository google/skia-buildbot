describe('Confirm our polyfill of Object Observe works.',
  function() {
    beforeEach(function() { });
    afterEach(function() { });

    function testSimpleOO() {
      var model = {};

      var observations = [];

      // Which we then observe
      Object.observe(model, function(changes){
        changes.forEach(function(change) {
          observations.push(change);
        });
      });
      model["foo"] = "bar";

      setTimeout(function() {
        assert.equal(1, observations.length, "Observations should have been added.")
      });
    }


    it('should be able to use Object.observe', function() {
      testSimpleOO();
    });
  }
);
