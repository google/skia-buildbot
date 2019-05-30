Storing Expectations on Firestore
=================================

Gold Expectations are essentially a map of (Grouping, Digest) to Label where Grouping is
currently TestName (but could be a combination of TestName + ColorSpace or something
more complex), Digest is the md5 hash of an image's content, and Label is Positive, Negative,
or Untriaged (default).

These Triples are stored in a large map of maps, i.e. map[string]map[string]int. This is
encapsulated by the type Expectations. If a given (Grouping, Digest) doesn't have a label,
it is assumed to be Untriaged.

There is the idea of the MasterExpectations, which is the Expectations belonging to the
git branch "master". Additionally, there can be smaller BranchExpectations that belong
to a ChangeList and stay separate from the MasterExpectations until the ChangeList lands.

We'd like to be able to do the following:

  - Store and retrieve Expectations (both MasterExpectations and BranchExpectations).
  - Update the Label for a (Grouping, Digest).
  - Keep an audit record of what user updated the Label for a given (Grouping, Digest).
  - Undo a previous change.

