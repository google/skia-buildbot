package rbe

import (
	"context"
	"sort"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/chunker"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
	grpc_oauth "google.golang.org/grpc/credentials/oauth"
)

const (
	rbeService = "remotebuildexecution.googleapis.com:443"
)

// Client is a struct used to interact with RBE-CAS.
type Client struct {
	client *client.Client
}

// NewClient returns a Client instance.
func NewClient(ctx context.Context, instance string, ts oauth2.TokenSource) (*Client, error) {
	client, err := client.NewClient(ctx, instance, client.DialParams{
		Service:            rbeService,
		TransportCredsOnly: true,
	}, &client.PerRPCCreds{
		Creds: grpc_oauth.TokenSource{
			TokenSource: ts,
		},
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Client{
		client: client,
	}, nil
}

// Upload the given paths to RBE-CAS.
func (c *Client) Upload(ctx context.Context, root string, paths []string) (string, error) {
	is := command.InputSpec{
		Inputs: paths,
	}
	rootDigest, chunkers, _, err := c.client.ComputeMerkleTree(root, &is, chunker.DefaultChunkSize, filemetadata.NewNoopCache())
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, err := c.client.UploadIfMissing(ctx, chunkers...); err != nil {
		return "", skerr.Wrap(err)
	}
	return rootDigest.String(), nil
}

// Download the given digests from RBE-CAS.
func (c *Client) Download(ctx context.Context, root string, casDigest string) error {
	d, err := digest.NewFromString(casDigest)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := c.client.DownloadDirectory(ctx, d, root, filemetadata.NewNoopCache()); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// checkNodePropertiesIdentical returns an error if the two NodeProperties are
// not identical.
func checkNodePropertiesIdentical(a, b *remoteexecution.NodeProperties) error {
	if (a != nil) != (b != nil) {
		return skerr.Fmt("One NodeProperties is set while the other is not")
	}
	if a != nil {
		if (a.Mtime != nil) != (b.Mtime != nil) {
			return skerr.Fmt("One file has Mtime while the other does not")
		}
		if a.Mtime != nil {
			if !a.Mtime.AsTime().Equal(b.Mtime.AsTime()) {
				return skerr.Fmt("Mtime %q != %q", a.Mtime, b.Mtime)
			}
		}
		if (a.UnixMode != nil) != (b.UnixMode != nil) {
			return skerr.Fmt("One NodeProperties has UnixMode while the other does not")
		}
		if a.UnixMode != nil {
			if a.UnixMode.Value != b.UnixMode.Value {
				return skerr.Fmt("UnixMode %q != %q", a.UnixMode.Value, b.UnixMode.Value)
			}
		}
		if len(a.Properties) != len(b.Properties) {
			return skerr.Fmt("Properties differ in length; %d vs %d", len(a.Properties), len(b.Properties))
		}
		propsA := make([]string, 0, len(a.Properties))
		for _, prop := range a.Properties {
			propsA = append(propsA, prop.String())
		}
		sort.Strings(propsA)
		propsB := make([]string, 0, len(b.Properties))
		for _, prop := range b.Properties {
			propsB = append(propsB, prop.String())
		}
		sort.Strings(propsB)
		for idx, propA := range propsA {
			propB := propsB[idx]
			if propA != propB {
				return skerr.Fmt("Properties differ: %s vs %s", propA, propB)
			}
		}
	}
	return nil
}

// checkFilesIdentical returns an error if the two FileNodes are not identical.
func checkFilesIdentical(a, b *remoteexecution.FileNode) error {
	if a.Name != b.Name {
		return skerr.Fmt("Name %q != %q", a.Name, b.Name)
	}
	if a.IsExecutable != b.IsExecutable {
		return skerr.Fmt("Executable %v != %v", a.IsExecutable, b.IsExecutable)
	}
	if (a.Digest != nil) != (b.Digest != nil) {
		return skerr.Fmt("One file has a digest while the other does not")
	}
	if a.Digest != nil && b.Digest != nil {
		if a.Digest.Hash != b.Digest.Hash {
			return skerr.Fmt("Digest hash %q != %q", a.Digest.Hash, b.Digest.Hash)
		}
		if a.Digest.SizeBytes != b.Digest.SizeBytes {
			return skerr.Fmt("Size %q != %q", a.Digest.SizeBytes, b.Digest.SizeBytes)
		}
	}
	return checkNodePropertiesIdentical(a.NodeProperties, b.NodeProperties)
}

// checkSymlinksIdentical returns an error if the two SymlinkNodes are not
// identical.
func checkSymlinksIdentical(a, b *remoteexecution.SymlinkNode) error {
	if a.Name != b.Name {
		return skerr.Fmt("Name %q != %q", a.Name, b.Name)
	}
	if a.Target != b.Target {
		return skerr.Fmt("Target %q != %q", a.Target, b.Target)
	}
	return checkNodePropertiesIdentical(a.NodeProperties, b.NodeProperties)
}

// directoryNode helps to organize a tree of remoteexecution.Directory.
type directoryNode struct {
	*remoteexecution.Directory
	Children map[string]*directoryNode
}

// makeTree creates a tree of directoryNodes using the given Directories. It
// assumes that the list of Directories is complete and maps exactly to the
// sub-Directories of each of the Directories, in order.
func makeTree(dirs []*remoteexecution.Directory) (*directoryNode, []*remoteexecution.Directory) {
	rv := &directoryNode{
		Directory: dirs[0],
		Children:  map[string]*directoryNode{},
	}
	dirs = dirs[1:]
	for _, childDir := range rv.Directories {
		var childNode *directoryNode
		childNode, dirs = makeTree(dirs)
		rv.Children[childDir.Name] = childNode
	}
	return rv, dirs
}

// mergeTrees merges the given trees of directoryNodes into a new directoryNode.
// Returns an error if the two trees are incompatible, eg. trees that contain
// files with the same name but different digests.
func (c *Client) mergeTrees(ctx context.Context, a, b *directoryNode) (*directoryNode, error) {
	// Files.
	filesMap := map[string]*remoteexecution.FileNode{}
	for _, file := range a.Files {
		filesMap[file.Name] = file
	}
	for _, file := range b.Files {
		exist, ok := filesMap[file.Name]
		if ok {
			if err := checkFilesIdentical(file, exist); err != nil {
				return nil, skerr.Wrapf(err, file.Name)
			}
		} else {
			filesMap[file.Name] = file
		}
	}
	files := make([]*remoteexecution.FileNode, 0, len(filesMap))
	for _, file := range filesMap {
		files = append(files, file)
	}

	// Symlinks.
	symlinksMap := map[string]*remoteexecution.SymlinkNode{}
	for _, symlink := range a.Symlinks {
		symlinksMap[symlink.Name] = symlink
	}
	for _, symlink := range b.Symlinks {
		exist, ok := symlinksMap[symlink.Name]
		if ok {
			if err := checkSymlinksIdentical(symlink, exist); err != nil {
				return nil, skerr.Wrapf(err, symlink.Name)
			}
		} else {
			symlinksMap[symlink.Name] = symlink
		}
	}
	symlinks := make([]*remoteexecution.SymlinkNode, 0, len(symlinksMap))
	for _, symlink := range symlinksMap {
		symlinks = append(symlinks, symlink)
	}

	// Directories.
	children := map[string]*directoryNode{}
	dirsMap := map[string]*remoteexecution.DirectoryNode{}
	for _, dir := range a.Directories {
		dirsMap[dir.Name] = dir
	}
	for _, dir := range b.Directories {
		if _, ok := dirsMap[dir.Name]; ok {
			merged, err := c.mergeTrees(ctx, a.Children[dir.Name], b.Children[dir.Name])
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			chunk, err := chunker.NewFromProto(merged.Directory, chunker.DefaultChunkSize)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if _, err := c.client.UploadIfMissing(ctx, chunk); err != nil {
				return nil, skerr.Wrap(err)
			}
			dn := &remoteexecution.DirectoryNode{
				Name:   dir.Name,
				Digest: chunk.Digest().ToProto(),
			}
			dirsMap[dir.Name] = dn
			children[dir.Name] = merged
		} else {
			dirsMap[dir.Name] = dir
			children[dir.Name] = a.Children[dir.Name]
		}
	}
	dirs := make([]*remoteexecution.DirectoryNode, 0, len(dirsMap))
	for _, dir := range dirsMap {
		dirs = append(dirs, dir)
	}

	// Properties.
	properties := map[string]string{}
	var mTime *timestamp.Timestamp
	var unixMode *wrappers.UInt32Value
	if a.NodeProperties != nil {
		for _, prop := range a.NodeProperties.Properties {
			properties[prop.Name] = prop.Value
		}
		if a.NodeProperties.Mtime != nil {
			mTime = a.NodeProperties.Mtime
		}
		if a.NodeProperties.UnixMode != nil {
			unixMode = a.NodeProperties.UnixMode
		}
	}
	if b.NodeProperties != nil {
		// TODO(borenet): Do we need to make sure that both trees have the same
		// set of property keys?
		for _, prop := range b.NodeProperties.Properties {
			exist, ok := properties[prop.Name]
			if ok {
				if exist != prop.Value {
					return nil, skerr.Fmt("Property %q has conflicting values %q and %q", prop.Name, prop.Value, exist)
				}
			} else {
				properties[prop.Name] = prop.Value
			}
		}
		if b.NodeProperties.Mtime != nil {
			if mTime == nil {
				mTime = b.NodeProperties.Mtime
			} else if mTime.AsTime().Before(b.NodeProperties.Mtime.AsTime()) {
				mTime = b.NodeProperties.Mtime
			}
		}
		if b.NodeProperties.UnixMode != nil {
			if unixMode == nil {
				unixMode = b.NodeProperties.UnixMode
			} else if unixMode.Value != b.NodeProperties.UnixMode.Value {
				return nil, skerr.Fmt("Directory has conflicting modes %v and %v", unixMode, b.NodeProperties.UnixMode)
			}
		}
	}
	var nodeProps *remoteexecution.NodeProperties
	if len(properties) > 0 || mTime != nil || unixMode != nil {
		var propList []*remoteexecution.NodeProperty
		if len(properties) > 0 {
			propList = make([]*remoteexecution.NodeProperty, 0, len(properties))
			for name, value := range properties {
				propList = append(propList, &remoteexecution.NodeProperty{
					Name:  name,
					Value: value,
				})
			}
		}
		nodeProps = &remoteexecution.NodeProperties{
			Properties: propList,
			Mtime:      mTime,
			UnixMode:   unixMode,
		}
	}

	rv := &directoryNode{
		Directory: &remoteexecution.Directory{
			Files:          files,
			Directories:    dirs,
			Symlinks:       symlinks,
			NodeProperties: nodeProps,
		},
		Children: children,
	}
	return rv, nil
}

// Merge the given digests, returning a new digest which contains all of them.
func (c *Client) Merge(ctx context.Context, digests []string) (string, error) {
	// Shortcut for empty input.
	if len(digests) == 0 {
		return digest.Empty.String(), nil
	}

	// Obtain the contents of each of the digests.
	var trees []*directoryNode
	for _, casDigest := range digests {
		d, err := digest.NewFromString(casDigest)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		dirs, err := c.client.GetDirectoryTree(ctx, d.ToProto())
		if err != nil {
			return "", skerr.Wrap(err)
		}
		tree, _ := makeTree(dirs)
		trees = append(trees, tree)
	}

	// Merge the contents.
	root := trees[0]
	for _, tree := range trees[1:] {
		var err error
		root, err = c.mergeTrees(ctx, root, tree)
		if err != nil {
			return "", skerr.Wrap(err)
		}
	}

	// Upload the new root.
	chunk, err := chunker.NewFromProto(root.Directory, chunker.DefaultChunkSize)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, err := c.client.UploadIfMissing(ctx, chunk); err != nil {
		return "", skerr.Wrap(err)
	}
	return chunk.Digest().String(), nil
}
