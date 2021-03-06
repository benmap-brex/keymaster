package okta

import (
	"sync"
	"time"

	"github.com/Cloud-Foundations/golib/pkg/log"
	"github.com/Cloud-Foundations/keymaster/lib/simplestorage"
)

// This module implements the PasswordAuthenticator interface and will implement
// a unified 2fa backend interface in some future

type authCacheData struct {
	response OktaApiPrimaryResponseType
	expires  time.Time
}

type PasswordAuthenticator struct {
	authnURL   string
	logger     log.DebugLogger
	mutex      sync.Mutex
	recentAuth map[string]authCacheData
}

type PushResponse int

const (
	PushResponseRejected PushResponse = iota
	PushResponseApproved
	PushResponseWaiting
	PushResonseTimeout
)

// New creates a new PasswordAuthenticator using Okta as the backend. The Okta
// Public Application API is used, so rate limits apply.
// The Okta domain to check must be given by oktaDomain.
// Log messages are written to logger. A new *PasswordAuthenticator is returned.
func NewPublic(oktaDomain string, logger log.DebugLogger) (
	*PasswordAuthenticator, error) {
	return newPublicAuthenticator(oktaDomain, logger)
}

// NewPublicTesting creates a new public authenticator, but
// pointing to an explicit authenticator url intead of okta urls.
// Log messages are written to logger. A new *PasswordAuthenticator is returned.
func NewPublicTesting(authnURL string, logger log.DebugLogger) (
	*PasswordAuthenticator, error) {
	pa, err := newPublicAuthenticator("example.com", logger)
	if err != nil {
		return pa, err
	}
	pa.authnURL = authnURL
	return pa, nil
}

// PasswordAuthenticate will authenticate a user using the provided username and
// password.
// It returns true if the user is authenticated, else false (due to either
// invalid username or incorrect password), and an error.
func (pa *PasswordAuthenticator) PasswordAuthenticate(username string,
	password []byte) (bool, error) {
	return pa.passwordAuthenticate(username, password)
}

func (pa *PasswordAuthenticator) UpdateStorage(storage simplestorage.SimpleStore) error {
	return nil
}

// ValidateUserOTP validates the otp value for an authenticated user.
// Assumes the user has a recent password authentication transaction.
// Returns true if the OTP value is valid according to okta, false otherwise.
func (pa *PasswordAuthenticator) ValidateUserOTP(username string, otpValue int) (bool, error) {
	return pa.validateUserOTP(username, otpValue)
}

// ValidateUserPush initializes or checks if a user MFA push has succeed for
// a specific user. Returns one of PushRessponse.
func (pa *PasswordAuthenticator) ValidateUserPush(username string) (PushResponse, error) {
	return pa.validateUserPush(username)
}
