describe('sk.stateReflector',
  function() {
    var clock;

    afterEach(function () { if (clock) { clock.restore(); } });

    function testStateReflector() {
      clock = sinon.useFakeTimers();
      var page = {
        state: {
          height: 2.0,
          name: "Fritz",
          alive: true,
          labels: "foo bar baz",
        }
      };

      var initHistoryLength = window.history.length;
      var spy = sinon.spy();
      var nextResolveCallback;
      var pending = new Promise(function(resolve, reject) {
        nextResolveCallback = resolve;
        var callback = function() {
          spy();
          nextResolveCallback();
        };
        sk.stateReflector(page, callback);
        // Fake the polymer-ready event.
        window.dispatchEvent(new Event('polymer-ready'));
      }).then(function() {
        assert.equal(spy.callCount, 1);
        page.state.height = 1.5;
        page.state.name = "Lucy";
        clock.tick(200);  // Causes timers to be called.
        assert.equal(window.history.length - initHistoryLength, 1);
        page.state.alive = false;
        page.state.labels = "foo bar";
        clock.tick(200);  // Causes timers to be called.
        assert.equal(window.history.length - initHistoryLength, 2);
        assert.deepEqual(page.state,
                         { height: 1.5,
                           name: "Lucy",
                           alive: false,
                           labels: "foo bar" });
        // Trigger popstate due to history.back() and wait for callback.
        return new Promise(function (resolve, reject) {
          nextResolveCallback = resolve;
          window.history.back();
        });
      }).then(function() {
        assert.equal(spy.callCount, 2);
        assert.deepEqual(page.state,
                         { height: 1.5,
                           name: "Lucy",
                           alive: true,
                           labels: "foo bar baz" });
        // Trigger popstate due to history.back() and wait for callback.
        return new Promise(function (resolve, reject) {
          nextResolveCallback = resolve;
          window.history.back();
        });
      }).then(function() {
        assert.equal(spy.callCount, 3);
        assert.deepEqual(page.state,
                         { height: 2.0,
                           name: "Fritz",
                           alive: true,
                           labels: "foo bar baz" });
        // Trigger popstate due to history.forward() and wait for callback.
        return new Promise(function (resolve, reject) {
          nextResolveCallback = resolve;
          window.history.forward();
        });
      }).then(function() {
        assert.equal(spy.callCount, 4);
        assert.deepEqual(page.state,
                         { height: 1.5,
                           name: "Lucy",
                           alive: true,
                           labels: "foo bar baz" });
      });
      return pending;
    }

    it("should link browser history with a JS object's state",
       testStateReflector);
  }
);
