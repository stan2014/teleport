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
	"fmt"
	"testing"

	"github.com/gravitational/trace"
)

type fakeT struct {
	Failed bool
}

func (t *fakeT) Errorf(format string, args ...any) {
	t.Failed = true
}

func (t *fakeT) FailNow() {}

func (*fakeT) Helper() {}

func TestAssertTypeAndMessage(t *testing.T) {
	notFoundErr := trace.NotFound("llama not found")
	wrappedNotFoundErr := trace.Wrap(notFoundErr)
	wrappedAgainNotFoundErr := fmt.Errorf("bad: %w", wrappedNotFoundErr)

	AssertTypeAndMessage(t, &trace.NotFoundError{Message: "not found"}, notFoundErr)
	AssertTypeAndMessage(t, &trace.NotFoundError{Message: "not found"}, wrappedNotFoundErr)
	AssertTypeAndMessage(t, &trace.NotFoundError{Message: "not found"}, wrappedAgainNotFoundErr)

	assertFails := func(t *testing.T, message string, fn func(tt *fakeT)) {
		t.Helper()
		tt := &fakeT{}
		fn(tt)
		if !tt.Failed {
			t.Error(message)
		}
	}

	assertFails(t, "want cannot be trace.Error", func(tt *fakeT) {
		AssertTypeAndMessage(tt, trace.NotFound("not llama"), notFoundErr)
	})
	assertFails(t, "Different strings didn't fail", func(tt *fakeT) {
		AssertTypeAndMessage(tt, &trace.NotFoundError{Message: "not llama"}, notFoundErr)
	})
	assertFails(t, "Different types didn't fail", func(tt *fakeT) {
		AssertTypeAndMessage(tt, &trace.BadParameterError{Message: "not found"}, notFoundErr)
	})
}
