describe('Test Mailbox functions.',
  function() {
    function testMailbox() {

      var value1 = "aaa";
      var value2 = "bbb";
      var value3 = "ddd";

      // Test that subscribing to a new mailbox sets the value to null.
      sk.Mailbox.subscribe("foo", function(value) {
        value1 = value;
      });

      assert.isNull(value1);
      assert.equal(value2, "bbb");

      // Test sending a string value by mailbox.
      sk.Mailbox.send("foo", "ccc");
      assert.equal(value1, "ccc");
      assert.equal(value2, "bbb");

      // Test that sending before any subscribers works.
      sk.Mailbox.send("bar", {a: 1});
      sk.Mailbox.subscribe("bar", function(value) {
        value2 = value;
      });

      assert.equal(value1, "ccc");
      assert.deepEqual(value2, {a: 1});

      // Test multiple subscribers to a single mailbox.
      sk.Mailbox.subscribe("bar", function(value) {
        value3 = value;
      });

      assert.equal(value1, "ccc");
      assert.deepEqual(value2, {a: 1});
      assert.deepEqual(value3, {a: 1});

      // Test sending to multiple subscribers of a single mailbox.
      sk.Mailbox.send("bar", {b: 2});
      assert.equal(value1, "ccc");
      assert.deepEqual(value2, {b: 2});
      assert.deepEqual(value3, {b: 2});

      // Test unsubscribe, by unsubscribing within the callback.
      var f = function(value) {
        sk.Mailbox.unsubscribe("bar", f);
      }
      assert.equal(2, sk.Mailbox.boxes["bar"].callbacks.length);
      sk.Mailbox.subscribe("bar", f);
      assert.equal(2, sk.Mailbox.boxes["bar"].callbacks.length);
    }

    it('should be able to subscribe to mailboxes', function() {
      testMailbox();
    });
  }
);
