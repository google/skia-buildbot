describe('Test sk.getRoundNumber.',
  function() {
    function testRounding() {
      var checks = [
        [    0,       200,           0,      10],
        [    1,       200,         200,      10],
        [    1,       199,         100,      10],
        [ -200,         1,           0,      10],
        [ -200,        -1,        -200,      10],
        [ -199,        -1,        -100,      10],
        [12919.256, 19378.8852,  15000,      10],
        [    1.2,       1.4,         1.2,    10],
        [    1.11,      1.16,        1.15,   10],
        [    0.3,       0.6,         0.5,    10],
        [    0.0004,    0.00045,     0.0004, 10],
        [    0,         1,           0,      10],
        [   10,        10,          10,      10],
        [    9.13,      9.13,        9.13,   10],
        [    3,         7,           4,       2],
        [    0.1,      10,           8,       2],
      ];
      for (var i = 0; i < checks.length; i++) {
        assert.equal(sk.getRoundNumber(checks[i][0], checks[i][1], checks[i][3]), checks[i][2]);
      }
    }

    function testBadInput() {
      var err = false;
      try {
        sk.getRoundNumber(10, 9, 9.5); // min > max
      } catch(e) {
        err = true;
      }
      assert.equal(err, true);
    }

    it('Verify that we get the expected rounding result.', function() {
      testRounding();
      testBadInput();
    });
  }
);
