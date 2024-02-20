// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errtest

import (
	"errors"
	"strings"

	"github.com/gravitational/trace"
)

type T interface {
	Errorf(format string, args ...any)
	FailNow()
	Helper()
}

// AssertTypeAndMessage asserts that got contains an error of type want and that
// its message contains a substring of want's message.
//
// Returns true if the assertion passes, false otherwise.
//
// Example:
//
//	errtest.AssertTypeAndMessage(t, &trace.NotFoundError{Message: "not found"}, trace.NotFound("llama not found")) // true
//	errtest.AssertTypeAndMessage(t, &trace.BadParameterError{Message: "not found"}, trace.NotFound("llama not found")) // false
func AssertTypeAndMessage[E error](t T, want E, got error) bool {
	t.Helper()

	// Guard against trace.Error, otherwise all trace errors will match each
	// other.
	if errors.As(want, new(trace.Error)) {
		t.Errorf("want cannot be a trace.Error. Use &trace.BadParameterError{} instead of trace.BadParameter().")
		return false
	}

	ok := true
	if !errors.As(got, new(E)) {
		t.Errorf("error doesn't match type=%T: %v", want, got)
		ok = false
	}
	if g, w := got.Error(), want.Error(); !strings.Contains(g, w) {
		t.Errorf("error doesn't contain substring %q: %q", g, w)
		ok = false
	}
	return ok
}

// RequireTypeAndMessage is a fatal version of [AssertTypeAndMessage].
func RequireTypeAndMessage[E error](t T, want E, got error) {
	if !AssertTypeAndMessage(t, want, got) {
		t.FailNow()
	}
}
