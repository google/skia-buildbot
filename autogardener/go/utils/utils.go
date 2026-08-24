package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
		requestCounter: metrics2.GetCounter("gemini_request_count", map[string]string{"model": model}),
		tokenLimiter:   rate.NewLimiter(rate.Limit(float64(tpm)/60.0), tpm),
		tokenCounter:   metrics2.GetCounter("gemini_token_count", map[string]string{"model": model}),
	}
}

// Wait for the given request to be able to run. Returns the estimated token
// count for the request.
func (l *RateLimiter) Wait(ctx context.Context, client *genai.Client, history []*genai.Content, parts []genai.Part) (int32, error) {
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
	resp, err := client.Models.CountTokens(ctx, l.model, contents, nil)
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

// RecordResponseTokens runs after a given request runs. It initiates a Wait()
// which does not block the current goroutine but ensures that the response
// tokens are accounted for.
func (l *RateLimiter) RecordResponseTokens(ctx context.Context, tokens int32) error {
	if tokens <= 0 {
		sklog.Warningf("RecordResponseTokens called with non-positive token count %d", tokens)
		return nil
	}
	l.tokenCounter.Inc(int64(tokens))

	// Call WaitN in a goroutine to ensure that the response tokens are
	// accounted for when triggering future requests, but to avoid blocking the
	// current goroutine.
	go func() {
		_ = l.tokenLimiter.WaitN(ctx, int(tokens))
	}()

	return nil
}

var (
	hexRegex          = regexp.MustCompile(`\b0x[0-9a-fA-F]{9,}\b`)
	stackPointerRegex = regexp.MustCompile(`([{(,]\s*)0x[0-9a-fA-F]+(\??\b)`)
	funcOffsetRegex   = regexp.MustCompile(`\+0x[0-9a-fA-F]+\b`)
	pidTidRegex       = regexp.MustCompile(`\b(pid|PID|process|tid|TID|thread|Thread|LWP|goroutine)(\s*[=:]\s*|\s+)(\[[0-9]+\]|[0-9]+\b)`)
	swarmingRegex     = regexp.MustCompile(`/b/s/w/ir(?:/(?:cache|work|git|build|out|task_driver|recipe_bootstrap|kitchen-workdir))*/?`)
	tmpPrefixRegex    = regexp.MustCompile(`(\s|^|")(/tmp/[a-zA-Z0-9_\-\.]+/?)`)
	portRegex         = regexp.MustCompile(`(localhost|\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):([1-9][0-9]{3,4})\b`)
	dateRegex         = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	timeRegex         = regexp.MustCompile(`\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[-+]\d{2}:\d{2})?`)
	ipRegex           = regexp.MustCompile(`\b(?:127\.0\.0\.1|0\.0\.0\.0|169\.254\.169\.254|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[0-1])\.\d{1,3}\.\d{1,3})\b`)
	logStampRegex     = regexp.MustCompile(`(?:\b([DIWEF])(\d{4}) <TIME>\s+(\d+)\b|\[([DIWEF])<DATE>T<TIME>\s+(\d+)\s+(\d+)\s*)`)
	failuresRegex     = regexp.MustCompile(`(?m)^(\d+[ \t]+)?[Ff]ailures(:)?[ \t]*$`)
	stdoutStderr      = regexp.MustCompile(`; Stdout\+Stderr:\n`)
)

// SanitizeErrorText normalizes dynamic noise (such as memory addresses,
// temporary Swarming path prefixes, process/thread IDs, ports, timestamps,
// and IP addresses) from error messages to make failure classification more
// stable, preventing runaway FailureClass proliferation.
func SanitizeErrorText(errText string) string {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		return ""
	}

	errText = dateRegex.ReplaceAllString(errText, "<DATE>")
	errText = timeRegex.ReplaceAllString(errText, "<TIME>")
	errText = pidTidRegex.ReplaceAllString(errText, "${1}${2}<ID>")

	errText = logStampRegex.ReplaceAllStringFunc(errText, func(match string) string {
		if strings.HasPrefix(match, "[") {
			severity := string(match[1])
			return "[" + severity + "<DATE>T<TIME> <PID> <TID> "
		}
		severity := string(match[0])
		return severity + "<DATE> <TIME> <TID>"
	})

	errText = hexRegex.ReplaceAllString(errText, "0x...")
	errText = stackPointerRegex.ReplaceAllString(errText, "${1}0x...${2}")
	errText = funcOffsetRegex.ReplaceAllString(errText, "+0x...")
	errText = swarmingRegex.ReplaceAllString(errText, "/<workdir>/")
	errText = tmpPrefixRegex.ReplaceAllString(errText, "${1}/<workdir>/")
	errText = portRegex.ReplaceAllString(errText, "${1}:<port>")
	errText = ipRegex.ReplaceAllString(errText, "<IP>")
	errText = failuresRegex.ReplaceAllString(errText, "")
	errText = stdoutStderr.ReplaceAllString(errText, "\n")

	// Clean up any double-slashes or orphaned slashes resulting from replacement
	errText = strings.ReplaceAll(errText, "/<workdir>//", "/<workdir>/")

	// Clear leading newlines and trailing whitespace.
	errText = strings.TrimLeft(errText, "\r\n")
	errText = strings.TrimRight(errText, "\t\r\n ")

	return dedent(errText)
}

func dedent(text string) string {
	lines := strings.Split(text, "\n")
	var commonIndent string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var indent string
		for _, char := range line {
			if char == ' ' || char == '\t' {
				indent += string(char)
			} else {
				break
			}
		}
		if commonIndent == "" {
			commonIndent = indent
		} else {
			commonIndent = longestCommonPrefix(commonIndent, indent)
		}
		// If we have no common indent, we can exit without modifying anything.
		if commonIndent == "" {
			return text
		}
	}

	if len(commonIndent) > 0 {
		for i, line := range lines {
			if strings.HasPrefix(line, commonIndent) {
				lines[i] = strings.TrimPrefix(line, commonIndent)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func longestCommonPrefix(a, b string) string {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	i := 0
	for i < limit && a[i] == b[i] {
		i++
	}
	return a[:i]
}

var genericErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?i)(?:command\s+)?(?:exited|failed)?\s*(?:with\s+)?(?:exit\s+|status\s+|code\s+)*-?[0-9]*(?::\s*.*)?$`),
	regexp.MustCompile(`^(?i)failed to run command$`),
	regexp.MustCompile(`^(?i)task timed out$`),
	regexp.MustCompile(`^(?i)context deadline exceeded$`),
	regexp.MustCompile(`^(?i)recipe failed$`),
	regexp.MustCompile(`^(?i)step failed$`),
	regexp.MustCompile(`^(?i)unknown error$`),
	regexp.MustCompile(`^(?i)error$`),
	regexp.MustCompile(`^(?i)failed$`),
	regexp.MustCompile(`^(?i)segmentation fault$`),
	regexp.MustCompile(`^(?i)panic: exit status -?[0-9]+( \[recovered\])?$`),
	regexp.MustCompile(`^(?i)caught signal [0-9]+.*$`),
}

// ErrorIsGeneric returns true if the error text is too short, empty, or consists
// only of standard generic error boilerplates (like "exit status 1") that do not
// contain specific diagnostic details. Creating distinct FailureClasses for these
// would cause unrelated errors to be incorrectly grouped together.
func ErrorIsGeneric(errText string) bool {
	errText = strings.TrimSpace(errText)
	if len(errText) == 0 {
		return true
	}

	for _, pattern := range genericErrorPatterns {
		if pattern.MatchString(errText) {
			return true
		}
	}

	return false
}
