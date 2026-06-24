package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

func DoBackoff(opName string, fn func() error) error {
	// These are default values at the time of writing, but we lay them out
	// explicitly for clarity.
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.RandomizationFactor = 0.5
	b.Multiplier = 1.5
	b.MaxInterval = 60 * time.Second
	b.MaxElapsedTime = 15 * time.Minute
	return backoff.RetryNotify(fn, b, func(err error, d time.Duration) {
		metrics2.GetCounter("autogardener_gemini_backoff_retry", map[string]string{"op": opName}).Inc(1)
		sklog.Warningf("%s failed; retrying in %s: %s", opName, d, extractErrorMessage(err))
	})
}

func extractErrorMessage(err error) string {
	if apiError, ok := err.(genai.APIError); ok {
		return fmt.Sprintf("Error %d %s; Message: %s", apiError.Code, apiError.Status, apiError.Message)
	}
	return err.Error()
}

type RateLimiter struct {
	model          string
	requestLimiter *rate.Limiter
	requestCounter metrics2.Counter
	tokenLimiter   *rate.Limiter
	tokenCounter   metrics2.Counter
}

func NewRateLimiter(rpm, tpm int, model string) *RateLimiter {
	return &RateLimiter{
		model:          model,
		requestLimiter: rate.NewLimiter(rate.Limit(float64(rpm)/60.0), rpm),
		requestCounter: metrics2.GetCounter("gemini-request-count", map[string]string{"model": model}),
		tokenLimiter:   rate.NewLimiter(rate.Limit(float64(tpm)/60.0), tpm),
		tokenCounter:   metrics2.GetCounter("gemini-token-count", map[string]string{"model": model}),
	}
}

// Wait for the given request to be able to run. Returns the estimated token
// count for the request.
func (l *RateLimiter) Wait(ctx context.Context, client *genai.Client, history []*genai.Content, parts []genai.Part, config *genai.GenerateContentConfig) (int32, error) {
	if err := l.requestLimiter.Wait(ctx); err != nil {
		return 0, skerr.Wrap(err)
	}
	contentParts := make([]*genai.Part, 0, len(parts))
	for _, p := range parts {
		p := p // Copy to avoid pointing to loop variable
		contentParts = append(contentParts, &p)
	}
	contents := append(history, &genai.Content{
		Parts: contentParts,
	})
	resp, err := client.Models.CountTokens(ctx, l.model, contents, &genai.CountTokensConfig{
		HTTPOptions:       config.HTTPOptions,
		SystemInstruction: config.SystemInstruction,
		Tools:             config.Tools,
	})
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if err := l.tokenLimiter.WaitN(ctx, int(resp.TotalTokens)); err != nil {
		return resp.TotalTokens, skerr.Wrap(err)
	}
	l.requestCounter.Inc(1)
	l.tokenCounter.Inc(int64(resp.TotalTokens))
	return resp.TotalTokens, nil
}

func (l *RateLimiter) WaitTokens(ctx context.Context, tokens int32) error {
	if tokens <= 0 {
		return nil
	}
	if err := l.tokenLimiter.WaitN(ctx, int(tokens)); err != nil {
		return skerr.Wrap(err)
	}
	l.tokenCounter.Inc(int64(tokens))
	return nil
}
