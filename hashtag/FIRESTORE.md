# Firestore

Hashtag mostly serves search results from other sources, but there is some need
for local data storage. For example, users can mark a document as 'hidden' for a
specific hashtag search.

Here we outline how data is stored in Firestore.


## Hidden

For each hashtag we want to keep a list of URLs that should be hidden. The URLs
correspond to the URLs in source.Artifact. We will only store information on
URLs that have been hidden, along with the id of the user that marked the URL as
hidden.

For each hidden URL we will write a document at:

    /hashtag/[instance - skia]/hidden/[hashtag-url]

I.e. in a collection named 'hidden' we'll write a document with an id of the
hashtag and the url combined. That document will contain just the URL and the
hashtag. This will allow querying for a specific hashtag across all documents in
the 'hidden' collection.

    hashtag: foo
    url:  https://.....

Note that we never store the Artifact type, so this will work no matter what set
of artifacts we are displaying, and for each hashtag query we only do a single
query, which is to load all documents below ./[hashtag]/.