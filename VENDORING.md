Vendoring
=========

NOTE: This is a TEMPORARY vendoring solution until versioning is implemented as
described here: https://blog.golang.org/versioning-proposal

Strategy:
=========

- Use vendoring sparingly only for packages that break us repeatedly.
- Check in the entire 'vendor' folder.
- Check out explicitly defined specific commits.
- Use 'govendor' (https://github.com/kardianos/govendor) as our tool because
  it widely used and easy to install via "go get".
  (See https://medium.com/@onuryilmaz/comparison-of-go-vendoring-tools-acf019ea476f
   for a dicussion of vendoring tools).


Vendoring a package:
====================

Let's say we want to add the go.example.com/awesome package and all it's
sub-packages.

- First fetch the latest version of the package to your GOPATH via 'go get':
  ```
  $ go get -u go.example.com/awesome/...
  ```

- Add a specific commit from that package to the vendor folder:
  ```
  $ govendor add go.example.com/awesome/...@076344b67ac19bcb8096c4b12440c3cf92ac3927
  ```
  This will copy the package from the GOPATH to the vendor folder.

- Check in the updated 'vendor' folder and the 'vendor.json' file it contains.

