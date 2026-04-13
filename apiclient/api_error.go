package apiclient

import (
	"fmt"
	"strings"

	"github.com/cmcoffee/snugforge/nfo"
)

// APIError represents one or more errors returned by an API response.
type APIError struct {
	prefix  string
	message []string
	codes   []string
	err     map[string]struct{}
}

// noError returns true if no error codes have been registered.
func (e APIError) noError() bool {
	return e.err == nil
}

// Register adds an error code and message to the APIError.
func (e *APIError) Register(code, message string) {
	code = strings.ToUpper(code)

	if e.err == nil {
		e.err = make(map[string]struct{})
	}

	e.err[code] = struct{}{}

	if message != "" {
		e.codes = append(e.codes, code)
		e.message = append(e.message, message)
	}
}

// Error returns the formatted error string.
func (e APIError) Error() string {
	str := make([]string, 0)
	e_len := len(e.message)
	for i := 0; i < e_len; i++ {
		if e_len == 1 {
			if e.prefix == "" {
				return fmt.Sprintf("%s (%s)", e.message[i], e.codes[i])
			} else {
				return fmt.Sprintf("%s => %s (%s)", e.prefix, e.message[i], e.codes[i])
			}
		} else {
			if i == 0 && e.prefix != "" {
				str = append(str, fmt.Sprintf("%s -", e.prefix))
			}
			str = append(str, fmt.Sprintf("[%d] %s (%s)", i, e.message[i], e.codes[i]))
		}
	}
	return strings.Join(str, "\n")
}

// PrefixAPIError adds a prefix to an APIError. Returns the original error
// unchanged if it is not an APIError.
func PrefixAPIError(prefix string, err error) error {
	if e, ok := err.(APIError); !ok {
		return err
	} else {
		e.prefix = prefix
		return e
	}
}

// IsAPIError reports whether err is an APIError. If codes are provided,
// it also checks whether the error contains any of the specified codes.
func IsAPIError(err error, code ...string) bool {
	e, ok := err.(APIError)
	if !ok {
		return false
	}
	if len(code) == 0 {
		return true
	}
	for _, v := range code {
		if _, ok := e.err[strings.ToUpper(v)]; ok {
			return true
		}
	}
	return false
}

// clear_token removes or refreshes the token for the given username.
func (s *APIClient) clear_token(username string) {
	nfo.Debug("[%s]: Clearing token (running=%v).", username, s.running)
	if s.running {
		token, err := s.TokenStore.Load(username)
		if err != nil {
			nfo.Fatal(err)
		}
		if err := s.refreshToken(username, token); err == nil {
			nfo.Debug("[%s]: Token refreshed successfully during clear_token.", username)
			if err := s.TokenStore.Save(username, token); err != nil {
				nfo.Fatal(err)
			}
		} else {
			nfo.Debug("[%s]: Refresh failed during clear_token (%v), deleting token.", username, err)
			s.TokenStore.Delete(username)
		}
	} else {
		nfo.Debug("[%s]: Deleting token (not running).", username)
		s.TokenStore.Delete(username)
	}
}

// isTokenError checks if the error matches any configured TokenErrorCodes
// and clears the token if so.
func (s *APIClient) isTokenError(username string, err error) bool {
	if s.TokenErrorCodes == nil {
		return false
	}
	if IsAPIError(err, s.TokenErrorCodes[0:]...) {
		nfo.Err(err)
		s.clear_token(username)
		return true
	}
	return false
}

// isRetryError checks if the error matches any configured RetryErrorCodes.
func (s *APIClient) isRetryError(err error) bool {
	if s.RetryErrorCodes == nil {
		return false
	}
	return IsAPIError(err, s.RetryErrorCodes[0:]...)
}
