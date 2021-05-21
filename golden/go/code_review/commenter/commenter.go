// Package commenter contains an implementation of the code_review.ChangelistCommenter interface.
// It should be CRS-agnostic.
package commenter

import (
	"bytes"
	"context"
	"text/template"

	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/code_review"
)

const (
	numRecentOpenCLsMetric = "gold_num_recent_open_cls"
)

type ReviewSystem struct {
	ID     string // e.g. "gerrit", "gerrit-internal"
	Client code_review.Client
}

type Impl struct {
	db              *pgxpool.Pool
	instanceURL     string
	messageTemplate *template.Template
	systems         []ReviewSystem
}

func New(db *pgxpool.Pool, systems []ReviewSystem, messageTemplate, instanceURL string) (*Impl, error) {
	templ, err := template.New("message").Parse(messageTemplate)
	if err != nil && messageTemplate != "" {
		return nil, skerr.Wrapf(err, "Message template %q", messageTemplate)
	}
	return &Impl{
		db:              db,
		instanceURL:     instanceURL,
		messageTemplate: templ,
		systems:         systems,
	}, nil
}

// CommentOnChangelistsWithUntriagedDigests implements the code_review.ChangelistCommenter
// interface.
func (i *Impl) CommentOnChangelistsWithUntriagedDigests(ctx context.Context) error {
	return nil
}

// maybeCommentOn either comments on the given CL/PS that there are untriaged digests on it or
// logs if this commenter is configured to not actually comment.
func (i *Impl) maybeCommentOn(ctx context.Context, system ReviewSystem, cl code_review.Changelist, ps code_review.Patchset, untriagedDigests int) error {
	msg, err := i.untriagedMessage(commentTemplateContext{
		CRS:           system.ID,
		ChangelistID:  cl.SystemID,
		PatchsetOrder: ps.Order,
		NumUntriaged:  untriagedDigests,
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := system.Client.CommentOn(ctx, cl.SystemID, msg); err != nil {
		if err == code_review.ErrNotFound {
			sklog.Warningf("Cannot comment on %s CL %s because it does not exist", system.ID, cl.SystemID)
			return nil
		}
		return skerr.Wrapf(err, "commenting on %s CL %s", system.ID, cl.SystemID)
	}
	return nil
}

// commentTemplateContext contains the fields that can be substituted into
type commentTemplateContext struct {
	ChangelistID      string
	CRS               string
	InstanceURL       string
	NumUntriaged      int
	PatchsetOrder     int
	PublicInstanceURL string
}

// untriagedMessage returns a message about untriaged images on the given CL/PS.
func (i *Impl) untriagedMessage(c commentTemplateContext) (string, error) {
	c.InstanceURL = i.instanceURL
	var b bytes.Buffer
	if err := i.messageTemplate.Execute(&b, c); err != nil {
		return "", skerr.Wrapf(err, "With template context %#v", c)
	}
	return b.String(), nil
}

// Make sure Impl fulfills the code_review.ChangelistCommenter interface.
var _ code_review.ChangelistCommenter = (*Impl)(nil)
