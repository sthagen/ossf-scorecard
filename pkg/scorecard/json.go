// Copyright 2021 OpenSSF Scorecard Authors
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

package scorecard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ossf/scorecard/v5/checker"
	docs "github.com/ossf/scorecard/v5/docs/checks"
	sce "github.com/ossf/scorecard/v5/errors"
	"github.com/ossf/scorecard/v5/log"
)

type jsonCheckResult struct {
	Name       string
	Details    []string
	Confidence int
	Pass       bool
}

type jsonScorecardResult struct {
	Repo     string
	Date     string
	Checks   []jsonCheckResult
	Metadata []string
}

type jsonCheckDocumentationV2 struct {
	URL   string `json:"url"`
	Short string `json:"short"`
	// Can be extended if needed.
}

//nolint:govet
type jsonCheckResultV2 struct {
	Details     []string                 `json:"details"`
	Score       int                      `json:"score"`
	Reason      string                   `json:"reason"`
	Name        string                   `json:"name"`
	Doc         jsonCheckDocumentationV2 `json:"documentation"`
	Annotations []string                 `json:"annotations,omitempty"`
}

type jsonRepoV2 struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

type jsonScorecardV2 struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

type jsonFloatScore float64

var errNoDoc = errors.New("doc is nil")

func (s jsonFloatScore) MarshalJSON() ([]byte, error) {
	// Note: for integers, this will show as X.0.
	return []byte(fmt.Sprintf("%.1f", s)), nil
}

// JSONScorecardResultV2 exports results as JSON for new detail format.
//
//nolint:govet
type JSONScorecardResultV2 struct {
	Date           string              `json:"date"`
	Repo           jsonRepoV2          `json:"repo"`
	Scorecard      jsonScorecardV2     `json:"scorecard"`
	AggregateScore jsonFloatScore      `json:"score"`
	Checks         []jsonCheckResultV2 `json:"checks"`
	Metadata       []string            `json:"metadata"`
}

// AsJSON2ResultOption provides configuration options for JSON2 Scorecard results.
type AsJSON2ResultOption struct {
	LogLevel    log.Level
	Details     bool
	Annotations bool
}

// AsJSON exports results as JSON for new detail format.
func (r *Result) AsJSON(showDetails bool, logLevel log.Level, writer io.Writer) error {
	encoder := json.NewEncoder(writer)

	out := jsonScorecardResult{
		Repo:     r.Repo.Name,
		Date:     r.Date.Format("2006-01-02"),
		Metadata: r.Metadata,
	}

	for _, checkResult := range r.Checks {
		tmpResult := jsonCheckResult{
			Name: checkResult.Name,
		}
		if showDetails {
			for i := range checkResult.Details {
				d := checkResult.Details[i]
				m := DetailToString(&d, logLevel)
				if m == "" {
					continue
				}
				tmpResult.Details = append(tmpResult.Details, m)
			}
		}
		out.Checks = append(out.Checks, tmpResult)
	}
	//nolint:musttag
	if err := encoder.Encode(out); err != nil {
		return sce.WithMessage(sce.ErrScorecardInternal, fmt.Sprintf("encoder.Encode: %v", err))
	}
	return nil
}

func (r *Result) resultsToJSON2(checkDocs docs.Doc, opt *AsJSON2ResultOption) (JSONScorecardResultV2, error) {
	if opt == nil {
		opt = &AsJSON2ResultOption{
			LogLevel:    log.DefaultLevel,
			Details:     false,
			Annotations: false,
		}
	}

	score, err := r.GetAggregateScore(checkDocs)
	if err != nil {
		return JSONScorecardResultV2{}, err
	}

	out := JSONScorecardResultV2{
		Repo: jsonRepoV2{
			Name:   r.Repo.Name,
			Commit: r.Repo.CommitSHA,
		},
		Scorecard: jsonScorecardV2{
			Version: r.Scorecard.Version,
			Commit:  r.Scorecard.CommitSHA,
		},
		Date:           r.Date.Format(time.RFC3339),
		Metadata:       r.Metadata,
		AggregateScore: jsonFloatScore(score),
	}

	for _, checkResult := range r.Checks {
		doc, e := checkDocs.GetCheck(checkResult.Name)
		if e != nil {
			return out, fmt.Errorf("GetCheck: %s: %w", checkResult.Name, e)
		}
		if doc == nil {
			return out, fmt.Errorf("GetCheck: %s: %w", checkResult.Name, errNoDoc)
		}

		tmpResult := jsonCheckResultV2{
			Name: checkResult.Name,
			Doc: jsonCheckDocumentationV2{
				URL:   doc.GetDocumentationURL(r.Scorecard.CommitSHA),
				Short: doc.GetShort(),
			},
			Reason: checkResult.Reason,
			Score:  checkResult.Score,
		}
		if opt.Details {
			for i := range checkResult.Details {
				d := checkResult.Details[i]
				m := DetailToString(&d, opt.LogLevel)
				if m == "" {
					continue
				}
				tmpResult.Details = append(tmpResult.Details, m)
			}
		}
		if opt.Annotations {
			tmpResult.Annotations = append(tmpResult.Annotations, checkResult.Annotations(r.Config)...)
		}
		out.Checks = append(out.Checks, tmpResult)
	}
	return out, nil
}

// AsJSON2 exports results as JSON for new detail format.
func (r *Result) AsJSON2(writer io.Writer, checkDocs docs.Doc, opt *AsJSON2ResultOption) error {
	encoder := json.NewEncoder(writer)
	out, err := r.resultsToJSON2(checkDocs, opt)
	if err != nil {
		return sce.WithMessage(sce.ErrScorecardInternal, err.Error())
	}

	if err := encoder.Encode(out); err != nil {
		return sce.WithMessage(sce.ErrScorecardInternal, fmt.Sprintf("encoder.Encode: %v", err))
	}

	return nil
}

// ExperimentalFromJSON2 is experimental. Do not depend on it, it may be removed at any point.
// Also returns the aggregate score, as the ScorecardResult field does not contain it.
func ExperimentalFromJSON2(r io.Reader) (result Result, score float64, err error) {
	var jsr JSONScorecardResultV2
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&jsr); err != nil {
		return Result{}, 0, fmt.Errorf("decode json: %w", err)
	}

	var parseErr *time.ParseError
	date, err := time.Parse(time.RFC3339, jsr.Date)
	if errors.As(err, &parseErr) {
		date, err = time.Parse("2006-01-02", jsr.Date)
	}
	if err != nil {
		return Result{}, 0, fmt.Errorf("parse scorecard analysis time: %w", err)
	}

	sr := Result{
		Repo: RepoInfo{
			Name:      jsr.Repo.Name,
			CommitSHA: jsr.Repo.Commit,
		},
		Scorecard: ScorecardInfo{
			Version:   jsr.Scorecard.Version,
			CommitSHA: jsr.Scorecard.Commit,
		},
		Date:     date,
		Metadata: jsr.Metadata,
		Checks:   make([]checker.CheckResult, 0, len(jsr.Checks)),
	}

	for _, check := range jsr.Checks {
		cr := checker.CheckResult{
			Name:   check.Name,
			Score:  check.Score,
			Reason: check.Reason,
		}
		cr.Details = make([]checker.CheckDetail, 0, len(check.Details))
		for _, detail := range check.Details {
			cr.Details = append(cr.Details, stringToDetail(detail))
		}
		sr.Checks = append(sr.Checks, cr)
	}

	return sr, float64(jsr.AggregateScore), nil
}
