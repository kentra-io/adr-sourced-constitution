package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

func TestConsentGate(t *testing.T) {
	tests := []struct {
		name      string
		policy    string
		approve   bool
		isTTY     bool
		input     string
		wantErr   bool
		wantInErr string
	}{
		{name: "off policy proceeds", policy: config.ConsentOff, wantErr: false},
		{name: "off policy proceeds even non-tty", policy: config.ConsentOff, isTTY: false, wantErr: false},
		{name: "strict with approve proceeds", policy: config.ConsentStrict, approve: true, wantErr: false},
		{
			name: "strict non-tty without approve refuses", policy: config.ConsentStrict,
			isTTY: false, wantErr: true, wantInErr: "not a terminal",
		},
		{
			name: "strict tty with yes proceeds", policy: config.ConsentStrict,
			isTTY: true, input: "yes\n", wantErr: false,
		},
		{
			name: "strict tty with y proceeds", policy: config.ConsentStrict,
			isTTY: true, input: "y\n", wantErr: false,
		},
		{
			name: "strict tty with no refuses", policy: config.ConsentStrict,
			isTTY: true, input: "no\n", wantErr: true, wantInErr: "not confirmed",
		},
		{
			name: "strict tty with empty refuses", policy: config.ConsentStrict,
			isTTY: true, input: "\n", wantErr: true, wantInErr: "not confirmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			g := consentGate{
				policy:  tt.policy,
				approve: tt.approve,
				isTTY:   tt.isTTY,
				in:      strings.NewReader(tt.input),
				out:     &out,
			}
			err := g.confirm("create ADR-0001")
			if (err != nil) != tt.wantErr {
				t.Fatalf("confirm() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantInErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantInErr)) {
				t.Errorf("confirm() err = %v, want it to contain %q", err, tt.wantInErr)
			}
		})
	}
}
