package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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
	d.logger.Debug("update hook called", "repo", repo, "arg", arg)

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
		d.logger.Error("error finding user")
		return
	}

	// Get repo
	r, err := d.Repository(ctx, repo)
	if err != nil {
		d.logger.Error("error finding repository", "repo", repo, "err", err)
		return
	}

	// TODO: run this async
	// This would probably need something like an RPC server to communicate with the hook process.
	if git.IsZeroHash(arg.OldSha) || git.IsZeroHash(arg.NewSha) {
		wh, err := webhook.NewBranchTagEvent(ctx, user, r, arg.RefName, arg.OldSha, arg.NewSha)
		if err != nil {
			d.logger.Error("error creating branch_tag webhook", "err", err)
		} else if err := webhook.SendEvent(ctx, wh); err != nil {
			d.logger.Error("error sending branch_tag webhook", "err", err)
		}
	}
	wh, err := webhook.NewPushEvent(ctx, user, r, arg.RefName, arg.OldSha, arg.NewSha)
	if err != nil {
		d.logger.Error("error creating push webhook", "err", err)
	} else if err := webhook.SendEvent(ctx, wh); err != nil {
		d.logger.Error("error sending push webhook", "err", err)
	}

	// Notify the GYT backend so it can dismiss stale reviews and push WS events.
	if hookURL := os.Getenv("GYT_BACKEND_HOOK_URL"); hookURL != "" && !git.IsZeroHash(arg.NewSha) {
		branch := strings.TrimPrefix(arg.RefName, "refs/heads/")
		if branch != arg.RefName {
			owner := user.Username()
			go notifyGytBackend(hookURL, owner, repo, branch)
		}
	}
}

// notifyGytBackend sends a push notification to the GYT backend gateway.
func notifyGytBackend(hookURL, owner, repo, branch string) {
	body, err := json.Marshal(map[string]string{"owner": owner, "repo": repo, "branch": branch})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hookURL+"/hooks/push", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
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
