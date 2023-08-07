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

package client

// PromptMFAChallenge is used in tests to replace the standard MFA prompt.
var PromptMFAChallenge = &promptMFAChallenge

// HasTouchIDCredentials is used in tests to replace the standard Touch ID platform support check.
var HasTouchIDCredentials = &hasTouchIDCredentials

func (tc *TeleportClient) SetDTAttemptLoginIgnorePing(val bool) {
	tc.dtAttemptLoginIgnorePing = val
}

func (tc *TeleportClient) SetDTAutoEnrollIgnorePing(val bool) {
	tc.dtAutoEnrollIgnorePing = val
}

func (tc *TeleportClient) SetDTAuthnRunCeremony(fn DTAuthnRunCeremonyFunc) {
	tc.DTAuthnRunCeremony = fn
}

func (tc *TeleportClient) SetDTAutoEnroll(fn dtAutoEnrollFunc) {
	tc.dtAutoEnroll = fn
}
