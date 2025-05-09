// Copyright 2022 OpenSSF Scorecard Authors
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

package evaluation

import (
	"testing"

	"github.com/ossf/scorecard/v5/checker"
	sce "github.com/ossf/scorecard/v5/errors"
	"github.com/ossf/scorecard/v5/finding"
	"github.com/ossf/scorecard/v5/probes/hasUnverifiedBinaryArtifacts"
	scut "github.com/ossf/scorecard/v5/utests"
)

// TestBinaryArtifacts tests the binary artifacts check.
func TestBinaryArtifacts(t *testing.T) {
	t.Parallel()
	lineStart := uint(123)
	unverifiedBinaryFinding := finding.Finding{
		Probe:   hasUnverifiedBinaryArtifacts.Probe,
		Outcome: finding.OutcomeTrue,

		Location: &finding.Location{
			Path:      "path",
			Type:      finding.FileTypeBinary,
			LineStart: &lineStart,
		},
	}

	tests := []struct {
		name     string
		findings []finding.Finding
		result   scut.TestReturn
	}{
		{
			name: "no binary artifacts",
			findings: []finding.Finding{
				{
					Probe:   hasUnverifiedBinaryArtifacts.Probe,
					Outcome: finding.OutcomeFalse,
				},
			},
			result: scut.TestReturn{
				Score: checker.MaxResultScore,
			},
		},
		{
			name: "one binary artifact",
			findings: []finding.Finding{
				unverifiedBinaryFinding,
			},
			result: scut.TestReturn{
				Score:        9,
				NumberOfWarn: 1,
			},
		},
		{
			name: "two binary artifact",
			findings: []finding.Finding{
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
			},
			result: scut.TestReturn{
				Score:        8,
				NumberOfWarn: 2,
			},
		},
		{
			name: "five binary artifact",
			findings: []finding.Finding{
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
			},
			result: scut.TestReturn{
				Score:        5,
				NumberOfWarn: 5,
			},
		},
		{
			name: "twelve binary artifact - ensure score doesn't drop below min",
			findings: []finding.Finding{
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
				unverifiedBinaryFinding,
			},
			result: scut.TestReturn{
				Score:        checker.MinResultScore,
				NumberOfWarn: 12,
			},
		},
		{
			name:     "invalid findings",
			findings: []finding.Finding{},
			result: scut.TestReturn{
				Score: checker.InconclusiveResultScore,
				Error: sce.ErrScorecardInternal,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dl := scut.TestDetailLogger{}
			got := BinaryArtifacts(tt.name, tt.findings, &dl)
			scut.ValidateTestReturn(t, tt.name, &tt.result, &got, &dl)
		})
	}
}
