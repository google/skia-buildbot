#Mocking Examples

The basic flow for making a test involving mock_gcs_client
involves creating a mock, and calling `On()` on it to mock out
the function calls.

```
var testBytes []byte = ...

func TestFetchFromGCS(t *testing.T) {
	testutils.SmallTest(t)
	m := test_gcsclient.NewMockClient()
	defer m.AssertExpectations(t)

	o := MyObjStruct{client:m}

	// AnythingOfType is handy when you don't really care what gets passed in, as long
	// as it matches the type specified.
	ctx := mock.AnythingOfType("*context.emptyCtx")

	m.On("GetFileContents", ctx, "my-file").Return(testBytes, nil)

	o.DoAThing()

	// Make sure we don't spam GCS with requests
	m.AssertNumberOfCalls(t, "GetFileContents", 1)
}

type MyObjStruct {
	client gcs.GCSClient
}


func (o *MyObj) DoAThing() {
	...
	content, err := o.client.GetFileContents(context.Background(), path)
	...
}
```

If you don't mock out a call, or the parameters don't match exactly, the test will fail and
give you a (hopefully) helpful message about how close the params were.

If you mock out a call that doesn't get used at all,
`AssertExpectations(t)` will catch this and make the test fail.

`mock.Anything()` and `mock.AnythingOfType(string)` are useful for keeping the
specifications too crazy.

In the above example, every call to `m.GetFileContents("my-file")` will return
the same set of `testBytes`. If your test wants to change this (i.e. simulating the
file goes missing), you'll need to do something like:

```
m.On("GetFileContents", ctx, "my-file").Return(testBytes, nil).Times(6)
m.On("GetFileContents", ctx, "my-file").Return(nil, errors.New("Not found"))
```

Where it will return the test bytes for the first 6 calls, and then return an error.

It is very tempting to do something like

```
	m.On("GetFileContents", ctx, "my-file").Return(testBytes, nil)

	o.DoAThing()

	m.On("GetFileContents", ctx, "my-file").Return(differentTestBytes, nil)

	// One might expect GetFileContents to start returning differentTestBytes, for "my-file",
	// but that is not the case.
	o.DoAThing()
```

But this does not work. As of this writing, unless `.Times()` is called, only the first
`m.On()` is used.


##More Information

The Mock API is at https://godoc.org/github.com/stretchr/testify/mock

