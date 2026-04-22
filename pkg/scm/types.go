package scm

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pluralsh/deployment-operator/internal/utils"
)

// PRDetails holds live PR state fetched directly from the SCM provider.
type PRDetails struct {
	Title    string
	Body     string
	Comments []PRComment
	CIChecks []CICheck
}

type PRComment struct {
	ID        string
	Author    string
	Body      string
	CreatedAt time.Time
}

type CICheck struct {
	Name       string
	Status     string
	Conclusion string // "success", "failure", "neutral", "cancelled", "skipped", "timed_out", ""
}

// Client fetches live PR state from an SCM provider.
type Client interface {
	GetPRDetails(ctx context.Context, prURL string) (*PRDetails, error)
}

// NewClient returns a provider-dispatching SCM client using token auth.
func NewClient(token string) Client {
	return &dispatchClient{token: token}
}

type dispatchClient struct {
	token string
}

func (d *dispatchClient) GetPRDetails(ctx context.Context, prURL string) (*PRDetails, error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return nil, fmt.Errorf("invalid PR URL %q: %w", prURL, err)
	}
	host := strings.ToLower(u.Host)

	switch {
	case strings.Contains(host, "github"):
		return newGitHubClient(d.token, host).GetPRDetails(ctx, prURL)
	default:
		return nil, fmt.Errorf("unsupported SCM host %q: only GitHub is supported", host)
	}
}

// PRStateHash produces a stable dedup hash over a slice of PRDetails.
// Comments are keyed by "id:body" so edits are detected; CI checks by "name:conclusion".
// Both are sorted before hashing so insertion order never causes false positives.
func PRStateHash(details ...*PRDetails) (string, error) {
	type hashable struct {
		Title    string
		Body     string
		Comments []string
		CIChecks []string
	}
	all := make([]hashable, 0, len(details))
	for _, d := range details {
		h := hashable{Title: d.Title, Body: d.Body}
		for _, c := range d.Comments {
			h.Comments = append(h.Comments, c.ID+":"+c.Body)
		}
		for _, ci := range d.CIChecks {
			h.CIChecks = append(h.CIChecks, ci.Name+":"+ci.Conclusion)
		}
		sort.Strings(h.Comments)
		sort.Strings(h.CIChecks)
		all = append(all, h)
	}
	return utils.HashObject(all)
}
