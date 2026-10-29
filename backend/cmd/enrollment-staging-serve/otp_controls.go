package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// waitOTPDelivery observes only the requested challenge's committed delivery
// state. The sender publishes the local code file before the OTP service commits
// that state, so file existence alone is not a verification-readiness barrier.
// This disposable stdin control neither reads a verifier nor verifies an OTP.
func (s *staging) waitOTPDelivery(ctx context.Context, challengeID string) error {
	if !fixtureUUID(challengeID) {
		return errors.New("invalid challenge")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var delivery string
		if err := s.db.QueryRowContext(ctx, `SELECT delivery FROM email_otp_challenges WHERE id=?`, challengeID).Scan(&delivery); err != nil {
			return err
		}
		if delivery == "delivered" {
			return ctx.Err()
		}
		if delivery != "pending" {
			return errors.New("challenge delivery unavailable")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// readOTPControls is the deliberately narrow default-mode JSONL interface. Like
// the opt-in fixture readers, its bounded input and EOF own process lifetime;
// no HTTP control route or authentication-policy exception is introduced.
func (s *staging) readOTPControls(ctx context.Context, input io.Reader, output io.Writer, stop context.CancelFunc) {
	defer stop()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4096)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		var command struct {
			Command     string `json:"command"`
			ChallengeID string `json:"challenge_id"`
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&command)
		if err == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				err = errors.New("one object required")
			}
		}
		ack := map[string]any{"fixture_control": true}
		if command.Command == "otp.wait" {
			ack["command"] = "otp.wait"
			if err == nil {
				err = s.waitOTPDelivery(ctx, command.ChallengeID)
			}
		} else {
			err = errors.New("unknown control")
		}
		ack["ok"] = err == nil
		if err != nil {
			ack["error"] = "fixture control rejected"
		}
		if encoder.Encode(ack) != nil {
			return
		}
	}
}
