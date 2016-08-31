// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package engine

import (
	"net/http"
	"sort"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/pubsub/v1"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth"
)

// createPubSubService returns configured instance of pubsub.Service.
func createPubSubService(c context.Context, pubSubURL string) (*pubsub.Service, error) {
	// In real mode (not a unit test), use authenticated transport.
	var transport http.RoundTripper
	if pubSubURL == "" {
		var err error
		transport, err = auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(pubsub.PubsubScope))
		if err != nil {
			return nil, err
		}
	} else {
		transport = http.DefaultTransport
	}
	service, err := pubsub.New(&http.Client{Transport: transport})
	if err != nil {
		return nil, err
	}
	if pubSubURL != "" {
		service.BasePath = pubSubURL
	}
	return service, nil
}

// configureTopic creates PubSub topic and subscription, allowing given
// publisher to send messages to the topic.
//
// Both topic and subscription names are fully qualified PubSub resource IDs,
// e.g. "projects/<id>/topics/<id>".
//
// Idempotent.
func configureTopic(c context.Context, topic, sub, pushURL, publisher, pubSubURL string) error {
	service, err := createPubSubService(c, pubSubURL)
	if err != nil {
		return err
	}

	// Create the topic. Ignore HTTP 409 (it means the topic already exists).
	logging.Infof(c, "Ensuring topic %q exists", topic)
	_, err = service.Projects.Topics.Create(topic, &pubsub.Topic{}).Context(c).Do()
	if err != nil && !isHTTP409(err) {
		logging.Errorf(c, "Failed - %s", err)
		return errors.WrapTransient(err)
	}

	// Create the subscription to this topic. Ignore HTTP 409.
	logging.Infof(c, "Ensuring subscription %q exists", sub)
	_, err = service.Projects.Subscriptions.Create(sub, &pubsub.Subscription{
		Topic:              topic,
		AckDeadlineSeconds: 70, // GAE request timeout plus some spare time
		PushConfig: &pubsub.PushConfig{
			PushEndpoint: pushURL, // if "", the subscription will be pull based
		},
	}).Context(c).Do()
	if err != nil && !isHTTP409(err) {
		logging.Errorf(c, "Failed - %s", err)
		return errors.WrapTransient(err)
	}

	// Modify topic's IAM policy to allow publisher to publish.
	if strings.HasSuffix(publisher, ".gserviceaccount.com") {
		publisher = "serviceAccount:" + publisher
	} else {
		publisher = "user:" + publisher
	}
	logging.Infof(c, "Ensuring %q can publish to the topic", publisher)

	// Do two attempts, to account for possible race condition. Two attempts
	// should be enough to handle concurrent calls to 'configureTopic': second
	// attempt will read already correct IAM policy and will just end right away.
	for attempt := 0; attempt < 2; attempt++ {
		err = modifyTopicIAMPolicy(c, service, topic, func(policy iamPolicy) error {
			policy.grantRole("roles/pubsub.publisher", publisher)
			return nil
		})
		if err == nil {
			return nil
		}
		logging.Errorf(c, "Failed - %s", err)
	}
	return errors.WrapTransient(err)
}

// pullSubcription pulls one message from PubSub subscription.
//
// Used on dev server only. Returns the message and callback to call to
// acknowledge the message.
func pullSubcription(c context.Context, subscription, pubSubURL string) (*pubsub.PubsubMessage, func(), error) {
	service, err := createPubSubService(c, pubSubURL)
	if err != nil {
		return nil, nil, err
	}

	resp, err := service.Projects.Subscriptions.Pull(subscription, &pubsub.PullRequest{
		ReturnImmediately: true,
		MaxMessages:       1,
	}).Context(c).Do()
	if err != nil {
		return nil, nil, err
	}

	switch len(resp.ReceivedMessages) {
	case 0:
		return nil, nil, nil
	case 1:
		ackID := resp.ReceivedMessages[0].AckId
		ackCb := func() {
			_, err := service.Projects.Subscriptions.Acknowledge(subscription, &pubsub.AcknowledgeRequest{
				AckIds: []string{ackID},
			}).Context(c).Do()
			if err != nil {
				logging.Errorf(c, "Failed to acknowledge the message - %s", err)
			}
		}
		return resp.ReceivedMessages[0].Message, ackCb, nil
	default:
		panic(errors.New("received more than one message from PubSub while asking only one"))
	}
}

func isHTTP409(err error) bool {
	apiErr, _ := err.(*googleapi.Error)
	return apiErr != nil && apiErr.Code == 409
}

// modifyTopicIAMPolicy reads IAM policy, calls callback to modify it, and then
// puts it back (if callback really changed it).
func modifyTopicIAMPolicy(c context.Context, service *pubsub.Service, topic string, cb func(iamPolicy) error) error {
	policy, err := service.Projects.Topics.GetIamPolicy(topic).Context(c).Do()
	if err != nil {
		return err
	}

	// Convert the policy to a map. Make a copy to be mutated by the callback.
	// Need to store the original to detect changes done by the callback.
	roles := iamPolicyFromBindings(policy.Bindings)
	clone := roles.clone()
	if err = cb(clone); err != nil {
		return err
	}

	// Skip storing if no changes are made.
	if clone.isEqual(roles) {
		return nil
	}

	// Convert back to IamPolicy struct.
	logging.Infof(c, "Updating IAM policy of %q", topic)
	request := &pubsub.SetIamPolicyRequest{
		Policy: &pubsub.Policy{
			Bindings: clone.toBindings(),
			Etag:     policy.Etag,
		},
	}
	_, err = service.Projects.Topics.SetIamPolicy(topic, request).Context(c).Do()
	return err
}

// iamPolicy is the IAM policy doc: map {role -> set of members}.
type iamPolicy map[string]stringset.Set

func iamPolicyFromBindings(bindings []*pubsub.Binding) iamPolicy {
	roles := make(iamPolicy, len(bindings))
	for _, b := range bindings {
		roles[b.Role] = stringset.NewFromSlice(b.Members...)
	}
	return roles
}

func (p iamPolicy) toBindings() []*pubsub.Binding {
	// Sort by role name.
	roles := make([]string, 0, len(p))
	for role := range p {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	// Sort members list too.
	bindings := make([]*pubsub.Binding, 0, len(p))
	for _, role := range roles {
		members := p[role].ToSlice()
		sort.Strings(members)
		bindings = append(bindings, &pubsub.Binding{
			Role:    role,
			Members: members,
		})
	}
	return bindings
}

func (p iamPolicy) clone() iamPolicy {
	clone := make(iamPolicy, len(p))
	for k, v := range p {
		clone[k] = v.Dup()
	}
	return clone
}

func (p iamPolicy) isEqual(another iamPolicy) bool {
	if len(p) != len(another) {
		return false
	}
	for k, right := range another {
		left := p[k]
		if left.Len() != right.Len() {
			return false
		}
		equal := true
		left.Iter(func(item string) bool {
			if !right.Has(item) {
				equal = false
				return false
			}
			return true
		})
		if !equal {
			return false
		}
	}
	return true
}

func (p iamPolicy) grantRole(role, principal string) {
	switch existing := p[role]; {
	case existing != nil && existing.Has(principal): // already there
		return
	case existing != nil: // the role is there, but not the principal
		existing.Add(principal)
	default:
		p[role] = stringset.NewFromSlice(principal)
	}
}
