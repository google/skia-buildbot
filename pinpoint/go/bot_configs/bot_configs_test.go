package bot_configs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetBotConfig verifies if GetBotConfig will
// return an error or not if the config was not found
func TestGetBotConfig(t *testing.T) {
	for i, test := range []struct {
		name          string
		bot           string
		externalOnly  bool
		expectedError bool
	}{
		{
			name:          "external bot",
			bot:           "win-10-perf",
			externalOnly:  true,
			expectedError: false,
		},
		{
			name:          "internal bot not found with external only data",
			bot:           "android-go-perf-pgo",
			externalOnly:  true,
			expectedError: true,
		},
		{
			name:          "internal bot found with internal data",
			bot:           "android-go-perf-pgo",
			externalOnly:  false,
			expectedError: false,
		},
		{
			name:          "completely made up bot",
			bot:           "the-cake-is-a-lie",
			externalOnly:  false,
			expectedError: true,
		},
		{
			name:          "check alias",
			bot:           "Android Go",
			externalOnly:  true,
			expectedError: false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			bot, err := GetBotConfig(test.bot, test.externalOnly)
			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotEmpty(t, bot.Browser)
			}
		})
	}
}

func TestValidateBotConfigs(t *testing.T) {
	assert.NoError(t, validate())
}
