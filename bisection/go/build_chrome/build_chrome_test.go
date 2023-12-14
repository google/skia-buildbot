package build_chrome

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
)

type mockBuildsClient struct {
	shouldErr bool
}

func TestSearchBuild(t *testing.T) {
	for i, test := range []struct {
		name          string
		builder       string
		commit        string
		patch         []*buildbucketpb.GerritChange
		expected      *buildbucketpb.SearchBuildsResponse
		expectedError bool
	}{
		{
			name:          "buildbucket search error",
			builder:       "builder",
			commit:        "commit",
			patch:         nil,
			expected:      &buildbucketpb.SearchBuildsResponse{},
			expectedError: true,
		},
		{
			name:    "build found",
			builder: "builder",
			commit:  "commit",
			patch:   nil,
			expected: &buildbucketpb.SearchBuildsResponse{
				Builds: []*buildbucketpb.Build{
					{
						Id:     1,
						Status: buildbucketpb.Status_SUCCESS,
						Input: &buildbucketpb.Build_Input{
							GerritChanges: []*buildbucketpb.GerritChange{},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name:          "build not found",
			builder:       "builder",
			commit:        "commit",
			patch:         nil,
			expected:      &buildbucketpb.SearchBuildsResponse{},
			expectedError: false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := buildbucketpb.NewMockBuildsClient(ctrl)

			if test.expectedError {
				client.EXPECT().SearchBuilds(ctx, gomock.Any()).Return(
					test.expected, fmt.Errorf("some error"))
			} else {
				client.EXPECT().SearchBuilds(ctx, gomock.Any()).Return(
					test.expected, nil)
			}

			id, status, err := SearchBuild(ctx, client, test.builder, test.commit, test.patch)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if len(test.expected.Builds) > 0 {
					assert.Equal(t, test.expected.Builds[0].Id, id)
					assert.Equal(t, test.expected.Builds[0].Status, status)
				} else {
					assert.Zero(t, id)
					assert.Zero(t, status)
				}
			}
		})
	}
}
