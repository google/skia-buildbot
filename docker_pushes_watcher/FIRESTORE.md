Storing Docker Image tags marked as "Prod" in Firestore
=======================================================

We need to keep track of which hashes for different docker images have been
marked as "Prod".

Schema
------

We should have a Firestore Collection (i.e. tables) for each docker image we
are interested in tracking. Using "infra-v2" as the image name in the below
example.

	infra-v2
		ImageName string  # The name of the image. Eg: gcr.io/skia-public/infra-v2
		Repo      string  # The repository the image was created from.
		Tag       string  # The commit hash of the above repo the image was created with.

Indexing
--------
Simple Indices should be fine.

Usage
-----
We simply query the Tag of an image to see if it should be updated with a more
recent Tag.
