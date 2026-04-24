package scm

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	gogithub "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

var githubPRPattern = regexp.MustCompile(`github(?:\.[^/]+)?/([^/]+)/([^/]+)/pull/(\d+)`)

type gitHubClient struct {
	gh *gogithub.Client
}

func newGitHubClient(token, host string) *gitHubClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	gh := gogithub.NewClient(tc)

	// GitHub Enterprise: upload/download URLs differ from api.github.com
	if host != "github.com" {
		baseURL := fmt.Sprintf("https://%s/api/v3/", host)
		uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)
		gh, _ = gh.WithEnterpriseURLs(baseURL, uploadURL)
	}
	return &gitHubClient{gh: gh}
}

func (c *gitHubClient) GetPRDetails(ctx context.Context, prURL string) (*PRDetails, error) {
	m := githubPRPattern.FindStringSubmatch(prURL)
	if m == nil {
		return nil, fmt.Errorf("cannot parse GitHub PR URL: %s", prURL)
	}
	owner, repo := m[1], m[2]
	number, _ := strconv.Atoi(m[3])

	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("get PR: %w", err)
	}

	comments, err := c.allComments(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	checks, err := c.checkRuns(ctx, owner, repo, pr.GetHead().GetSHA())
	if err != nil {
		return nil, err
	}

	return &PRDetails{
		Title:    pr.GetTitle(),
		Body:     pr.GetBody(),
		HeadRef:  pr.GetHead().GetRef(),
		Comments: comments,
		CIChecks: checks,
	}, nil
}

func (c *gitHubClient) allComments(ctx context.Context, owner, repo string, number int) ([]PRComment, error) {
	opts := &gogithub.IssueListCommentsOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	var all []PRComment
	for {
		batch, resp, err := c.gh.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list issue comments: %w", err)
		}
		for _, c := range batch {
			all = append(all, PRComment{
				ID:        strconv.FormatInt(c.GetID(), 10),
				Author:    c.GetUser().GetLogin(),
				Body:      c.GetBody(),
				CreatedAt: c.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Also fetch inline review comments
	ropts := &gogithub.PullRequestListCommentsOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	for {
		batch, resp, err := c.gh.PullRequests.ListComments(ctx, owner, repo, number, ropts)
		if err != nil {
			return nil, fmt.Errorf("list review comments: %w", err)
		}
		for _, c := range batch {
			all = append(all, PRComment{
				ID:        strconv.FormatInt(c.GetID(), 10),
				Author:    c.GetUser().GetLogin(),
				Body:      c.GetBody(),
				CreatedAt: c.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		ropts.Page = resp.NextPage
	}

	return all, nil
}

func (c *gitHubClient) checkRuns(ctx context.Context, owner, repo, sha string) ([]CICheck, error) {
	opts := &gogithub.ListCheckRunsOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	var all []CICheck
	for {
		result, resp, err := c.gh.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, opts)
		if err != nil {
			return nil, fmt.Errorf("list check runs: %w", err)
		}
		for _, cr := range result.CheckRuns {
			all = append(all, CICheck{
				Name:       cr.GetName(),
				Status:     cr.GetStatus(),
				Conclusion: cr.GetConclusion(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}
