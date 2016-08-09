describe('sk.request, sk.get, sk.post, and sk.delete',
  function() {
    var server;

    beforeEach(function () { server = sinon.fakeServer.create(); });
    afterEach(function () { server.restore(); });

    function testRequestSuccess() {
      var method = 'POST';
      var url = '/test-url';
      var requestBody = 'Test body';
      var requestHeaders = {'Content-Type': 'text/plain',
                            'X-Foo': 'Bar'};
      var promise = sk.request(method, url, requestBody, requestHeaders);
      assert.equal(server.requests.length, 1);
      var request = server.requests[0];
      assert.equal(request.method, method);
      assert.equal(request.url, url);
      assert.equal(request.requestBody, requestBody);
      assert.deepEqual(request.requestHeaders,
                       {'Content-Type': 'text/plain;charset=utf-8',
                        'X-Foo': 'Bar'});
      var responseBody = 'Hey, look at this body!';
      request.respond(200, {}, responseBody);
      return promise.should.eventually.equal(responseBody);
    }

    it('should send an async XMLHttpRequest', testRequestSuccess);

    function testRequestErrorResponse() {
      var method = 'GET';
      var url = '/test-url';
      var requestHeaders = {'X-Foo': 'Bar'};
      var promise = sk.request(method, url, null, requestHeaders);
      assert.equal(server.requests.length, 1);
      var request = server.requests[0];
      assert.equal(request.method, method);
      assert.equal(request.url, url);
      assert(!request.requestBody);
      assert.deepEqual(request.requestHeaders, requestHeaders);
      var responseBody = "This didn't work out.";
      request.respond(503, {}, responseBody);
      return promise.should.be.rejectedWith(responseBody);
    }

    it('should reject when non-OK response', testRequestErrorResponse);

    function testRequestNetworkError() {
      // Sinon does not seem to have a way to simulate network errors. Instead,
      // make a request to an invalid domain.
      server.restore();
      var method = 'GET';
      var url = 'http://x.invalid/test-url';
      return sk.request(method, url).should.be.rejectedWith(Error);
    }

    it('should fail with a network error', testRequestNetworkError);

    function testGet() {
      var url = '/test-url';
      var promise = sk.get(url);
      assert.equal(server.requests.length, 1);
      var request = server.requests[0];
      assert.equal(request.method, 'GET');
      assert.equal(request.url, url);
      assert(!request.requestBody);
      assert.deepEqual(request.requestHeaders, {});
      var responseBody = 'Got';
      request.respond(200, {}, responseBody);
      return promise.should.eventually.equal(responseBody);
    }

    it('should use GET for sk.get', testGet);

    function testPost() {
      var url = '/test-url';
      var obj = {
        green: {tea: 'matcha'},
        teas: ['chai', 'matcha', {tea: 'konacha'}]
      };
      var promise = sk.post(url, JSON.stringify(obj));
      assert.equal(server.requests.length, 1);
      var request = server.requests[0];
      assert.equal(request.method, 'POST');
      assert.equal(request.url, url);
      assert.deepEqual(JSON.parse(request.requestBody), obj);
      assert.deepEqual(request.requestHeaders,
                       {'Content-Type': 'application/json;charset=utf-8'});
      var responseBody = JSON.stringify({ brewed: true });
      request.respond(200, {}, responseBody);
      return promise.should.eventually.equal(responseBody);
    }

    it('should use POST and application/json for sk.post', testPost);

    function testDelete() {
      var url = '/test-url';
      var promise = sk.delete(url);
      assert.equal(server.requests.length, 1);
      var request = server.requests[0];
      assert.equal(request.method, 'DELETE');
      assert.equal(request.url, url);
      assert(!request.requestBody);
      // For some reason, Chrome automatically adds a Content-Type header even
      // though there's no body. (We probably don't care.)
      //assert.deepEqual(request.requestHeaders, {});
      request.respond(200, {}, "");
      return promise.should.eventually.equal("");
    }

    it('should use DELETE for sk.delete', testDelete);
  }
);
