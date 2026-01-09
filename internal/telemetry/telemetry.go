/*
Copyright © 2025 Sun Asterisk Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

// Package telemetry provides opt-in analytics and telemetry collection.
// Currently this is a stub implementation that does nothing (no-op).
// Future versions may implement opt-in telemetry to improve the CLI experience.
package telemetry

import (
	"context"
)

// Event represents a telemetry event
type Event struct {
	Name       string
	Properties map[string]interface{}
}

// Client represents a telemetry client
type Client struct {
	enabled bool
}

// NewClient creates a new telemetry client
// Currently returns a no-op client
func NewClient() *Client {
	return &Client{
		enabled: false, // Telemetry is disabled by default
	}
}

// IsEnabled returns whether telemetry is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// Enable enables telemetry collection (opt-in)
func (c *Client) Enable() {
	// No-op for now - future implementation would enable telemetry
	c.enabled = false // Keep disabled until properly implemented
}

// Disable disables telemetry collection
func (c *Client) Disable() {
	c.enabled = false
}

// Track records a telemetry event
// This is a no-op implementation
func (c *Client) Track(ctx context.Context, event Event) error {
	if !c.enabled {
		return nil
	}

	// Future implementation would send the event to a telemetry service
	// For now, this is a no-op

	return nil
}

// TrackCommand records a command execution event
// This is a no-op implementation
func (c *Client) TrackCommand(ctx context.Context, command string, duration int64, success bool) error {
	return c.Track(ctx, Event{
		Name: "command_executed",
		Properties: map[string]interface{}{
			"command":  command,
			"duration": duration,
			"success":  success,
		},
	})
}

// TrackError records an error event
// This is a no-op implementation
func (c *Client) TrackError(ctx context.Context, command string, errType string) error {
	return c.Track(ctx, Event{
		Name: "error_occurred",
		Properties: map[string]interface{}{
			"command":    command,
			"error_type": errType,
		},
	})
}

// Flush sends any pending events
// This is a no-op implementation
func (c *Client) Flush(ctx context.Context) error {
	// No-op
	return nil
}

// Close closes the telemetry client and flushes any pending events
func (c *Client) Close() error {
	// No-op
	return nil
}

// Global singleton client
var defaultClient = NewClient()

// Track records a telemetry event using the default client
func Track(ctx context.Context, event Event) error {
	return defaultClient.Track(ctx, event)
}

// TrackCommand records a command execution using the default client
func TrackCommand(ctx context.Context, command string, duration int64, success bool) error {
	return defaultClient.TrackCommand(ctx, command, duration, success)
}

// TrackError records an error using the default client
func TrackError(ctx context.Context, command string, errType string) error {
	return defaultClient.TrackError(ctx, command, errType)
}
