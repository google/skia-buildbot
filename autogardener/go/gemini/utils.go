package gemini

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

func doBackoff(opName string, fn func() error) error {
	// These are default values at the time of writing, but we lay them out
	// explicitly for clarity.
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.RandomizationFactor = 0.5
	b.Multiplier = 1.5
	b.MaxInterval = 60 * time.Second
	b.MaxElapsedTime = 15 * time.Minute
	return backoff.RetryNotify(fn, b, func(err error, d time.Duration) {
		sklog.Warningf("%s failed; retrying in %s: %s", opName, d, extractErrorMessage(err))
	})
}

func extractErrorMessage(err error) string {
	if apiError, ok := err.(*genai.APIError); ok {
		return fmt.Sprintf("Error %d %s; Message: %s", apiError.Code, apiError.Status, apiError.Message)
	}
	return err.Error()
}

type rateLimiter struct {
	requestLimiter *rate.Limiter
	tokenLimiter   *rate.Limiter
}

func newRateLimiter(rpm, tpm int) *rateLimiter {
	return &rateLimiter{
		requestLimiter: rate.NewLimiter(rate.Limit(float64(rpm)/60.0), rpm),
		tokenLimiter:   rate.NewLimiter(rate.Limit(float64(tpm)/60.0), tpm),
	}
}

func (l *rateLimiter) Wait(ctx context.Context, model string, client *genai.Client, history []*genai.Content, parts []genai.Part) error {
	if err := l.requestLimiter.Wait(ctx); err != nil {
		return skerr.Wrap(err)
	}
	contentParts := make([]*genai.Part, 0, len(parts))
	for _, p := range parts {
		p := p // Copy to avoid pointing to loop variable
		contentParts = append(contentParts, &p)
	}
	contents := append(history, &genai.Content{
		Parts: contentParts,
	})
	resp, err := client.Models.CountTokens(ctx, model, contents, nil)
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := l.tokenLimiter.WaitN(ctx, int(resp.TotalTokens)); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
