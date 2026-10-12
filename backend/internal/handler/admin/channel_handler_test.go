package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelToResponseDefaultsBlankStatusToActive(t *testing.T) {
	response := channelToResponse(&service.Channel{Status: "   "})

	require.NotNil(t, response)
	require.Equal(t, service.StatusActive, response.Status)
}

func TestChannelToResponsePreservesKnownStatus(t *testing.T) {
	response := channelToResponse(&service.Channel{Status: service.StatusDisabled})

	require.NotNil(t, response)
	require.Equal(t, service.StatusDisabled, response.Status)
}
