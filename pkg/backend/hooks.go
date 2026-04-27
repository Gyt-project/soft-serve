package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/Gyt-project/soft-serve/git"
	"github.com/Gyt-project/soft-serve/pkg/hooks"
	"github.com/Gyt-project/soft-serve/pkg/proto"
	"github.com/Gyt-project/soft-serve/pkg/sshutils"
	"github.com/Gyt-project/soft-serve/pkg/webhook"
)

var _ hooks.Hooks = (*Backend)(nil)

// PostReceive is called by the git post-receive hook.
//
// It implements Hooks.
func (d *Backend) PostReceive(_ context.Context, _ io.Writer, _ io.Writer, repo string, args []hooks.HookArg) {
	d.logger.Debug("post-receive hook called", "repo", repo, "args", args)
}

// PreReceive is called by the git pre-receive hook.
//
// It implements Hooks.
func (d *Backend) PreReceive(_ context.Context, _ io.Writer, _ io.Writer, repo string, args []hooks.HookArg) {
	d.logger.Debug("pre-receive hook called", "repo", repo, "args", args)
}

// Update is called by the git update hook.
//
// It implements Hooks.
func (d *Backend) Update(ctx context.Context, _ io.Writer, _ io.Writer, repo string, arg hooks.HookArg) {
	d.logger.Info("update hook called", "repo", repo, "ref", arg.RefName, "old", arg.OldSha, "new", arg.NewSha)

	// Find user
	var user proto.User
	if pubkey := os.Getenv("SOFT_SERVE_PUBLIC_KEY"); pubkey != "" {
		pk, _, err := sshutils.ParseAuthorizedKey(pubkey)
		if err != nil {
			d.logger.Error("error parsing public key", "err", err)
			return
		}

		user, err = d.UserByPublicKey(ctx, pk)
		if err != nil {
			d.logger.Error("error finding user from public key", "key", pubkey, "err", err)
			return
		}
	} else if username := os.Getenv("SOFT_SERVE_USERNAME"); username != "" {
		var err error
		user, err = d.User(ctx, username)
		if err != nil {
			d.logger.Error("error finding user from username", "username", username, "err", err)
			return
		}
	} else {
		d.logger.Error("update hook: cannot find user — neither SOFT_SERVE_PUBLIC_KEY nor SOFT_SERVE_USERNAME is set")
		return
	}

	d.logger.Info("update hook: user resolved", "username", user.Username())

	// Get repo
	r, err := d.Repository(ctx, repo)
	if err != nil {
		d.logger.Error("error finding repository", "repo", repo, "err", err)
		return
	}

	// TODO: run this async
	// This would probably need something like an RPC server to communicate with the hook process.
	if git.IsZeroHash(arg.OldSha) || git.IsZeroHash(arg.NewSha) {
		d.logger.Info("update hook: sending branch/tag webhook", "ref", arg.RefName)
		wh, err := webhook.NewBranchTagEvent(ctx, user, r, arg.RefName, arg.OldSha, arg.NewSha)
		if err != nil {
			d.logger.Error("error creating branch_tag webhook", "err", err)
		} else if err := webhook.SendEvent(ctx, wh); err != nil {
			d.logger.Error("error sending branch_tag webhook", "err", err)
		} else {
			d.logger.Info("update hook: branch/tag webhook sent ok", "ref", arg.RefName)
		}
	}

	d.logger.Info("update hook: sending push webhook", "ref", arg.RefName)
	wh, err := webhook.NewPushEvent(ctx, user, r, arg.RefName, arg.OldSha, arg.NewSha)
	if err != nil {
		d.logger.Error("error creating push webhook", "err", err)
	} else if err := webhook.SendEvent(ctx, wh); err != nil {
		d.logger.Error("error sending push webhook", "url", wh, "err", err)
	} else {
		d.logger.Info("update hook: push webhook sent ok", "ref", arg.RefName)
	}

	// Notify the GYT backend so it can dismiss stale reviews and push WS events.
	// NOTE: called synchronously — this is a hook subprocess that exits as soon as
	// Update() returns, so goroutines would be killed before the HTTP call completes.
	if hookURL := os.Getenv("GYT_BACKEND_HOOK_URL"); hookURL != "" && !git.IsZeroHash(arg.NewSha) {
		branch := strings.TrimPrefix(arg.RefName, "refs/heads/")
		if branch != arg.RefName {
			owner := user.Username()
			notifyGytBackend(d.logger, hookURL, owner, repo, branch)
		}
	} else if hookURL == "" {
		d.logger.Warn("GYT_BACKEND_HOOK_URL is not set — push events will not reach the gateway")
	}
}

// notifyGytBackend sends a push notification to the GYT backend gateway.
// Called synchronously from Update so it completes before the hook subprocess exits.
func notifyGytBackend(logger *log.Logger, hookURL, owner, repo, branch string) {
	url := hookURL + "/hooks/push"
	body, err := json.Marshal(map[string]string{"owner": owner, "repo": repo, "branch": branch})
	if err != nil {
		logger.Error("notifyGytBackend: failed to marshal payload", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logger.Error("notifyGytBackend: failed to create request", "url", url, "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	logger.Info("notifyGytBackend: calling gateway", "url", url, "owner", owner, "repo", repo, "branch", branch)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("notifyGytBackend: HTTP request failed", "url", url, "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Error("notifyGytBackend: unexpected status", "url", url, "status", fmt.Sprintf("%d %s", resp.StatusCode, resp.Status))
		return
	}
	logger.Info("notifyGytBackend: gateway notified ok", "url", url, "owner", owner, "repo", repo, "branch", branch)
}

// PostUpdate is called by the git post-update hook.
//
// It implements Hooks.
func (d *Backend) PostUpdate(ctx context.Context, _ io.Writer, _ io.Writer, repo string, args ...string) {
	d.logger.Debug("post-update hook called", "repo", repo, "args", args)

	var wg sync.WaitGroup

	// Populate last-modified file.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := populateLastModified(ctx, d, repo); err != nil {
			d.logger.Error("error populating last-modified", "repo", repo, "err", err)
			return
		}
	}()

	wg.Wait()
}

func populateLastModified(ctx context.Context, d *Backend, name string) error {
	var rr *repo
	_rr, err := d.Repository(ctx, name)
	if err != nil {
		return err
	}

	if r, ok := _rr.(*repo); ok {
		rr = r
	} else {
		return proto.ErrRepoNotFound
	}

	r, err := rr.Open()
	if err != nil {
		return err
	}

	c, err := r.LatestCommitTime()
	if err != nil {
		return err
	}

	return rr.writeLastModified(c)
}
