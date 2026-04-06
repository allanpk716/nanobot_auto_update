package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTrigger_Positive(t *testing.T) {
	assert.True(t, IsTrigger("Starting Telegram bot..."))
}

func TestIsTrigger_Negative(t *testing.T) {
	assert.False(t, IsTrigger("Some other log line"))
}

func TestIsTrigger_EmptyString(t *testing.T) {
	assert.False(t, IsTrigger(""))
}

func TestIsSuccess_Positive(t *testing.T) {
	assert.True(t, IsSuccess("Telegram bot commands registered"))
}

func TestIsSuccess_Negative(t *testing.T) {
	assert.False(t, IsSuccess("Starting Telegram bot"))
}

func TestIsFailure_Positive(t *testing.T) {
	assert.True(t, IsFailure("httpx.ConnectError: connection refused"))
}

func TestIsFailure_Negative(t *testing.T) {
	assert.False(t, IsFailure("Telegram bot commands registered"))
}
