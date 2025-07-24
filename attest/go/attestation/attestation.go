package attestation

import (
	"context"
	"fmt"

	binaryauthorization "cloud.google.com/go/binaryauthorization/apiv1"
	"cloud.google.com/go/binaryauthorization/apiv1/binaryauthorizationpb"
	containeranalysis "cloud.google.com/go/containeranalysis/apiv1beta1"
	"cloud.google.com/go/containeranalysis/apiv1beta1/grafeas/grafeaspb"
	"go.skia.org/infra/attest/go/types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/grafeas/v1"
)

// Client implements types.Client by delegating calls to the containeranalysis
// and binaryauthorization APIs.
type Client struct {
	attestor         string
	noteReference    string
	grafeasClient    *containeranalysis.GrafeasV1Beta1Client
	validationClient *binaryauthorization.ValidationHelperClient
}

// NewClient creates a Client which validates Docker image IDs using the given
// fully-qualified attestor resource name.
func NewClient(ctx context.Context, attestor string) (*Client, error) {
	// Validate the attestor before sending any requests.
	if err := types.ValidateAttestor(attestor); err != nil {
		return nil, err
	}

	// Create the token source.
	ts, err := google.DefaultTokenSource(ctx, binaryauthorization.DefaultAuthScopes()...)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed creating token source")
	}
	opt := option.WithTokenSource(ts)

	// Set up a temporary binary authorization client to obtain the attestor's
	// note reference.
	// TODO(borenet): We could add the note reference as a flag and avoid this
	// API call, since the result shouldn't be changing often if at all.
	binauthzClient, err := binaryauthorization.NewBinauthzManagementClient(ctx, opt)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed creating binauthz client")
	}
	defer util.Close(binauthzClient)
	att, err := binauthzClient.GetAttestor(ctx, &binaryauthorizationpb.GetAttestorRequest{
		Name: attestor,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "failed retrieving attestor")
	}
	note := att.GetUserOwnedGrafeasNote()
	if note == nil {
		return nil, skerr.Fmt("attestor has no grafeas note")
	}

	// Create the other API clients.
	grafeasClient, err := containeranalysis.NewGrafeasV1Beta1Client(ctx, opt)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed creating grafeas client")
	}
	validationClient, err := binaryauthorization.NewValidationHelperClient(ctx, opt)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed creating validation helper client")
	}

	return &Client{
		attestor:         attestor,
		noteReference:    note.NoteReference,
		grafeasClient:    grafeasClient,
		validationClient: validationClient,
	}, nil
}

// Verify implements types.Client.
func (c *Client) Verify(ctx context.Context, imageID string) (bool, error) {
	// Validate the image ID before we start sending requests.
	if err := types.ValidateImageID(imageID); err != nil {
		return false, skerr.Wrap(err)
	}

	// Find note occurrences matching the image ID.
	it := c.grafeasClient.ListNoteOccurrences(ctx, &grafeaspb.ListNoteOccurrencesRequest{
		Name:   c.noteReference,
		Filter: fmt.Sprintf("has_suffix(resourceUrl, %q)", imageID),
	})
	for {
		occurrence, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return false, skerr.Wrapf(err, "failed listing note occurrences")
		}
		verified, err := c.verifyOccurrence(ctx, occurrence)
		if err != nil {
			return false, skerr.Wrapf(err, "failed checking note occurrence")
		}
		if verified {
			// We only need one verified attestation.
			return true, nil
		}
	}
	return false, nil
}

// verifyOccurrence validates a single Container Analysis Note occurrence. It
// returns true if the occurrence is associated with an attestation with a valid
// signature and false if not, or an error if any of the required API calls
// failed.
func (c *Client) verifyOccurrence(ctx context.Context, occurrence *grafeaspb.Occurrence) (bool, error) {
	// There are several types of Occurrences of which Attestation is only one,
	// therefore we may run into non-Attestation Occurrences which are valid and
	// don't imply any error.
	attestation := occurrence.GetAttestation()
	if attestation == nil {
		return false, nil
	}

	// Convert the attestation struct from one API format to the other.
	var grafeasAttestationOccurrence grafeas.AttestationOccurrence
	if generic := attestation.Attestation.GetGenericSignedAttestation(); generic != nil {
		grafeasAttestationOccurrence.SerializedPayload = generic.SerializedPayload
		for _, s := range generic.Signatures {
			grafeasAttestationOccurrence.Signatures = append(grafeasAttestationOccurrence.Signatures, &grafeas.Signature{
				Signature:   s.Signature,
				PublicKeyId: s.PublicKeyId,
			})
		}
	} else if pgpSignedAttestation := attestation.Attestation.GetPgpSignedAttestation(); pgpSignedAttestation != nil {
		grafeasAttestationOccurrence.Signatures = append(grafeasAttestationOccurrence.Signatures, &grafeas.Signature{
			Signature:   []byte(pgpSignedAttestation.GetSignature()),
			PublicKeyId: pgpSignedAttestation.GetPgpKeyId(),
		})
	} else {
		sklog.Errorf("Attestation has no signature: %v", attestation)
		return false, nil
	}

	// Validate the signature(s) on the attestation.
	resp, err := c.validationClient.ValidateAttestationOccurrence(ctx, &binaryauthorizationpb.ValidateAttestationOccurrenceRequest{
		Attestor:              c.attestor,
		Attestation:           &grafeasAttestationOccurrence,
		OccurrenceNote:        occurrence.NoteName,
		OccurrenceResourceUri: occurrence.Resource.Uri,
	})
	if err != nil {
		return false, skerr.Wrapf(err, "failed requesting validation")
	}
	if resp.Result == binaryauthorizationpb.ValidateAttestationOccurrenceResponse_VERIFIED {
		// If there's one valid attestation for this image, we're done.
		return true, nil
	}
	sklog.Debugf("Attestation not verified for %s (%s). Result %q, denial reason: %q", occurrence.Resource.Uri, occurrence.Name, resp.Result.String(), resp.DenialReason)
	return false, nil
}

var _ types.Client = &Client{}
