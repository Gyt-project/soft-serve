package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Gyt-project/soft-serve/git"
	"github.com/Gyt-project/soft-serve/pkg/access"
	"github.com/Gyt-project/soft-serve/pkg/backend"
	"github.com/Gyt-project/soft-serve/pkg/config"
	"github.com/Gyt-project/soft-serve/pkg/proto"
	"github.com/Gyt-project/soft-serve/pkg/version"
	"github.com/Gyt-project/soft-serve/pkg/webhook"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the GitServerManagement gRPC service
type Server struct {
	UnimplementedGitServerManagementServer
	backend *backend.Backend
	config  *config.Config
	ctx     context.Context
}

// NewServer creates a new gRPC server instance
func NewServer(ctx context.Context, be *backend.Backend, cfg *config.Config) *Server {
	return &Server{
		backend: be,
		config:  cfg,
		ctx:     ctx,
	}
}

// Repository Management

func (s *Server) CreateRepository(ctx context.Context, req *CreateRepositoryRequest) (*Repository, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	var user proto.User
	var err error
	if req.Username != "" {
		user, err = s.backend.User(ctx, req.Username)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
	}

	opts := proto.RepositoryOptions{
		Private:     req.Private,
		Description: req.Description,
		ProjectName: req.ProjectName,
		Mirror:      req.Mirror,
		Hidden:      req.Hidden,
	}

	repo, err := s.backend.CreateRepository(ctx, req.Name, user, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create repository: %v", err)
	}

	return toProtoRepository(repo), nil
}

func (s *Server) DeleteRepository(ctx context.Context, req *DeleteRepositoryRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	err := s.backend.DeleteRepository(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete repository: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetRepository(ctx context.Context, req *GetRepositoryRequest) (*Repository, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	return toProtoRepository(repo), nil
}

func (s *Server) ListRepositories(ctx context.Context, req *ListRepositoriesRequest) (*ListRepositoriesResponse, error) {
	repos, err := s.backend.Repositories(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list repositories: %v", err)
	}

	protoRepos := make([]*Repository, len(repos))
	for i, repo := range repos {
		protoRepos[i] = toProtoRepository(repo)
	}

	return &ListRepositoriesResponse{
		Repositories: protoRepos,
	}, nil
}

func (s *Server) RenameRepository(ctx context.Context, req *RenameRepositoryRequest) (*Repository, error) {
	if req.OldName == "" || req.NewName == "" {
		return nil, status.Error(codes.InvalidArgument, "old name and new name are required")
	}

	err := s.backend.RenameRepository(ctx, req.OldName, req.NewName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to rename repository: %v", err)
	}

	repo, err := s.backend.Repository(ctx, req.NewName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get renamed repository: %v", err)
	}

	return toProtoRepository(repo), nil
}

func (s *Server) UpdateRepository(ctx context.Context, req *UpdateRepositoryRequest) (*Repository, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	if req.Description != nil {
		if err := s.backend.SetDescription(ctx, req.Name, *req.Description); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update description: %v", err)
		}
	}

	if req.IsPrivate != nil {
		if err := s.backend.SetPrivate(ctx, req.Name, *req.IsPrivate); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update private status: %v", err)
		}
	}

	if req.IsHidden != nil {
		if err := s.backend.SetHidden(ctx, req.Name, *req.IsHidden); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update hidden status: %v", err)
		}
	}

	if req.ProjectName != nil {
		if err := s.backend.SetProjectName(ctx, req.Name, *req.ProjectName); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update project name: %v", err)
		}
	}

	repo, err := s.backend.Repository(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated repository: %v", err)
	}

	return toProtoRepository(repo), nil
}

func (s *Server) ImportRepository(ctx context.Context, req *ImportRepositoryRequest) (*Repository, error) {
	if req.Name == "" || req.RemoteUrl == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and remote URL are required")
	}

	var user proto.User
	var err error
	if req.Username != "" {
		user, err = s.backend.User(ctx, req.Username)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
	}

	opts := proto.RepositoryOptions{
		Private:     req.Private,
		Description: req.Description,
		ProjectName: req.ProjectName,
		Hidden:      req.Hidden,
	}

	repo, err := s.backend.ImportRepository(ctx, req.Name, user, req.RemoteUrl, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to import repository: %v", err)
	}

	return toProtoRepository(repo), nil
}

// User Management

func (s *Server) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	opts := proto.UserOptions{
		Admin: req.Admin,
	}

	// Parse public keys
	if len(req.PublicKeys) > 0 {
		pks := make([]ssh.PublicKey, 0, len(req.PublicKeys))
		for _, keyStr := range req.PublicKeys {
			pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
			}
			pks = append(pks, pk)
		}
		opts.PublicKeys = pks
	}

	user, err := s.backend.CreateUser(ctx, req.Username, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	// Set password if provided
	if req.Password != nil && *req.Password != "" {
		if err := s.backend.SetPassword(ctx, req.Username, *req.Password); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to set password: %v", err)
		}
	}

	return toProtoUser(user), nil
}

func (s *Server) DeleteUser(ctx context.Context, req *DeleteUserRequest) (*emptypb.Empty, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	err := s.backend.DeleteUser(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete user: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	user, err := s.backend.User(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return toProtoUser(user), nil
}

func (s *Server) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	usernames, err := s.backend.Users(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users: %v", err)
	}

	users := make([]*User, 0, len(usernames))
	for _, username := range usernames {
		user, err := s.backend.User(ctx, username)
		if err != nil {
			continue // Skip users that can't be retrieved
		}
		users = append(users, toProtoUser(user))
	}

	return &ListUsersResponse{
		Users: users,
	}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*User, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	currentUsername := req.Username

	if req.NewUsername != nil && *req.NewUsername != "" {
		if err := s.backend.SetUsername(ctx, req.Username, *req.NewUsername); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update username: %v", err)
		}
		currentUsername = *req.NewUsername
	}

	if req.Admin != nil {
		if err := s.backend.SetAdmin(ctx, currentUsername, *req.Admin); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update admin status: %v", err)
		}
	}

	if req.Password != nil && *req.Password != "" {
		if err := s.backend.SetPassword(ctx, currentUsername, *req.Password); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update password: %v", err)
		}
	}

	user, err := s.backend.User(ctx, currentUsername)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated user: %v", err)
	}

	return toProtoUser(user), nil
}

// Public Key Management

func (s *Server) AddPublicKey(ctx context.Context, req *AddPublicKeyRequest) (*emptypb.Empty, error) {
	if req.Username == "" || req.PublicKey == "" {
		return nil, status.Error(codes.InvalidArgument, "username and public key are required")
	}

	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.PublicKey))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	err = s.backend.AddPublicKey(ctx, req.Username, pk)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add public key: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) RemovePublicKey(ctx context.Context, req *RemovePublicKeyRequest) (*emptypb.Empty, error) {
	if req.Username == "" || req.PublicKey == "" {
		return nil, status.Error(codes.InvalidArgument, "username and public key are required")
	}

	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.PublicKey))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	err = s.backend.RemovePublicKey(ctx, req.Username, pk)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove public key: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ListPublicKeys(ctx context.Context, req *ListPublicKeysRequest) (*ListPublicKeysResponse, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	pks, err := s.backend.ListPublicKeys(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list public keys: %v", err)
	}

	keyStrs := make([]string, len(pks))
	for i, pk := range pks {
		keyStrs[i] = string(ssh.MarshalAuthorizedKey(pk))
	}

	return &ListPublicKeysResponse{
		PublicKeys: keyStrs,
	}, nil
}

// Collaborator Management

func (s *Server) AddCollaborator(ctx context.Context, req *AddCollaboratorRequest) (*emptypb.Empty, error) {
	if req.RepoName == "" || req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and username are required")
	}

	accessLevel := toBackendAccessLevel(req.AccessLevel)
	err := s.backend.AddCollaborator(ctx, req.RepoName, req.Username, accessLevel)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add collaborator: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) RemoveCollaborator(ctx context.Context, req *RemoveCollaboratorRequest) (*emptypb.Empty, error) {
	if req.RepoName == "" || req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and username are required")
	}

	err := s.backend.RemoveCollaborator(ctx, req.RepoName, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove collaborator: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ListCollaborators(ctx context.Context, req *ListCollaboratorsRequest) (*ListCollaboratorsResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	usernames, err := s.backend.Collaborators(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list collaborators: %v", err)
	}

	protoCollabs := make([]*Collaborator, 0, len(usernames))
	for _, username := range usernames {
		level, isCollab, err := s.backend.IsCollaborator(ctx, req.RepoName, username)
		if err != nil || !isCollab {
			continue
		}
		protoCollabs = append(protoCollabs, &Collaborator{
			Username:    username,
			AccessLevel: toProtoAccessLevel(level),
		})
	}

	return &ListCollaboratorsResponse{
		Collaborators: protoCollabs,
	}, nil
}

// Access Token Management

func (s *Server) CreateAccessToken(ctx context.Context, req *CreateAccessTokenRequest) (*AccessToken, error) {
	if req.Username == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "username and token name are required")
	}

	user, err := s.backend.User(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.AsTime()
		expiresAt = &t
	}

	var expiresAtTime time.Time
	if expiresAt != nil {
		expiresAtTime = *expiresAt
	}

	tokenStr, err := s.backend.CreateAccessToken(ctx, user, req.Name, expiresAtTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create access token: %v", err)
	}

	// Return the token with basic info
	return &AccessToken{
		Id:        0, // ID not returned by CreateAccessToken
		Name:      req.Name,
		Token:     tokenStr,
		CreatedAt: timestamppb.Now(),
		ExpiresAt: req.ExpiresAt,
	}, nil
}

func (s *Server) DeleteAccessToken(ctx context.Context, req *DeleteAccessTokenRequest) (*emptypb.Empty, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	user, err := s.backend.User(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	err = s.backend.DeleteAccessToken(ctx, user, req.TokenId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete access token: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ListAccessTokens(ctx context.Context, req *ListAccessTokensRequest) (*ListAccessTokensResponse, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	user, err := s.backend.User(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	tokens, err := s.backend.ListAccessTokens(ctx, user)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list access tokens: %v", err)
	}

	protoTokens := make([]*AccessToken, len(tokens))
	for i, token := range tokens {
		protoTokens[i] = &AccessToken{
			Id:        token.ID,
			Name:      token.Name,
			Token:     "", // Don't return token value in list
			CreatedAt: timestamppb.New(token.CreatedAt),
			ExpiresAt: timestamppb.New(token.ExpiresAt),
		}
	}

	return &ListAccessTokensResponse{
		Tokens: protoTokens,
	}, nil
}

// Webhook Management

func (s *Server) CreateWebhook(ctx context.Context, req *CreateWebhookRequest) (*Webhook, error) {
	if req.RepoName == "" || req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and URL are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	contentType := webhook.ContentTypeJSON
	if req.ContentType == "form" {
		contentType = webhook.ContentTypeForm
	}

	events := make([]webhook.Event, len(req.Events))
	for i, e := range req.Events {
		parsedEvent, err := webhook.ParseEvent(e)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid event: %v", err)
		}
		events[i] = parsedEvent
	}

	secret := ""
	if req.Secret != nil {
		secret = *req.Secret
	}

	err = s.backend.CreateWebhook(ctx, repo, req.Url, contentType, secret, events, req.Active)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create webhook: %v", err)
	}

	// Get the newly created webhook (list and return the last one)
	hooks, err := s.backend.ListWebhooks(ctx, repo)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve created webhook: %v", err)
	}

	if len(hooks) == 0 {
		return nil, status.Error(codes.Internal, "webhook created but not found")
	}

	hook := hooks[len(hooks)-1]
	return toProtoWebhook(hook), nil
}

func (s *Server) DeleteWebhook(ctx context.Context, req *DeleteWebhookRequest) (*emptypb.Empty, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	err = s.backend.DeleteWebhook(ctx, repo, req.WebhookId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete webhook: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetWebhook(ctx context.Context, req *GetWebhookRequest) (*Webhook, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	hook, err := s.backend.Webhook(ctx, repo, req.WebhookId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "webhook not found: %v", err)
	}

	return toProtoWebhook(hook), nil
}

func (s *Server) ListWebhooks(ctx context.Context, req *ListWebhooksRequest) (*ListWebhooksResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	hooks, err := s.backend.ListWebhooks(ctx, repo)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list webhooks: %v", err)
	}

	protoHooks := make([]*Webhook, len(hooks))
	for i, hook := range hooks {
		protoHooks[i] = toProtoWebhook(hook)
	}

	return &ListWebhooksResponse{
		Webhooks: protoHooks,
	}, nil
}

func (s *Server) UpdateWebhook(ctx context.Context, req *UpdateWebhookRequest) (*Webhook, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	// Get current webhook to fill in non-updated fields
	currentHook, err := s.backend.Webhook(ctx, repo, req.WebhookId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "webhook not found: %v", err)
	}

	url := currentHook.URL
	if req.Url != nil {
		url = *req.Url
	}

	contentType := currentHook.ContentType
	if req.ContentType != nil {
		if *req.ContentType == "form" {
			contentType = webhook.ContentTypeForm
		} else {
			contentType = webhook.ContentTypeJSON
		}
	}

	secret := currentHook.Secret
	if req.Secret != nil {
		secret = *req.Secret
	}

	events := currentHook.Events
	if len(req.Events) > 0 {
		events = make([]webhook.Event, len(req.Events))
		for i, e := range req.Events {
			parsedEvent, err := webhook.ParseEvent(e)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid event: %v", err)
			}
			events[i] = parsedEvent
		}
	}

	active := currentHook.Active
	if req.Active != nil {
		active = *req.Active
	}

	err = s.backend.UpdateWebhook(ctx, repo, req.WebhookId, url, contentType, secret, events, active)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update webhook: %v", err)
	}

	hook, err := s.backend.Webhook(ctx, repo, req.WebhookId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve updated webhook: %v", err)
	}

	return toProtoWebhook(hook), nil
}

// Server Settings

func (s *Server) GetSettings(ctx context.Context, req *emptypb.Empty) (*ServerSettings, error) {
	allowKeyless := s.backend.AllowKeyless(ctx)
	anonAccess := s.backend.AnonAccess(ctx)

	return &ServerSettings{
		AllowKeyless: allowKeyless,
		AnonAccess:   toProtoAccessLevel(anonAccess),
	}, nil
}

func (s *Server) UpdateSettings(ctx context.Context, req *UpdateSettingsRequest) (*ServerSettings, error) {
	if req.AllowKeyless != nil {
		if err := s.backend.SetAllowKeyless(ctx, *req.AllowKeyless); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update allow keyless: %v", err)
		}
	}

	if req.AnonAccess != nil {
		accessLevel := toBackendAccessLevel(*req.AnonAccess)
		if err := s.backend.SetAnonAccess(ctx, accessLevel); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update anon access: %v", err)
		}
	}

	return s.GetSettings(ctx, &emptypb.Empty{})
}

// Health Check

func (s *Server) HealthCheck(ctx context.Context, req *emptypb.Empty) (*HealthCheckResponse, error) {
	return &HealthCheckResponse{
		Status:  "ok",
		Version: version.Version,
	}, nil
}

// Repository Content Browsing

func (s *Server) GetTree(ctx context.Context, req *GetTreeRequest) (*GetTreeResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Resolve reference (branch, tag, or commit)
	ref := ""
	refName := ""
	if req.Ref != nil && *req.Ref != "" {
		ref = *req.Ref
		refName = *req.Ref
	} else {
		head, err := gitRepo.HEAD()
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "HEAD not found: %v", err)
		}
		ref = head.ID
		refName = head.Name().Short()
	}

	// Get tree at ref
	tree, err := gitRepo.LsTree(ref)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "tree not found: %v", err)
	}

	// Navigate to path if specified
	path := ""
	if req.Path != nil && *req.Path != "" {
		path = *req.Path
		if path != "/" {
			te, err := tree.TreeEntry(path)
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "path not found: %v", err)
			}
			if te.Type() != "tree" {
				return nil, status.Error(codes.InvalidArgument, "path is not a directory")
			}
			tree, err = tree.SubTree(path)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get subtree: %v", err)
			}
		}
	}

	entries, err := tree.Entries()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read tree entries: %v", err)
	}

	entries.Sort()

	protoEntries := make([]*TreeEntry, len(entries))
	for i, entry := range entries {
		size := int64(entry.Size())
		isDir := entry.Type() == "tree"
		isSubmodule := entry.Type() == "commit"

		protoEntries[i] = &TreeEntry{
			Name:        entry.Name(),
			Path:        fmt.Sprintf("%s", entry.Name()), // Just the name, not full path
			Mode:        fmt.Sprintf("%o", entry.Mode()),
			Size:        size,
			IsDir:       isDir,
			IsSubmodule: isSubmodule,
		}
	}

	return &GetTreeResponse{
		Entries: protoEntries,
		Ref:     refName,
	}, nil
}

func (s *Server) GetBlob(ctx context.Context, req *GetBlobRequest) (*GetBlobResponse, error) {
	if req.RepoName == "" || req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and path are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Resolve reference
	ref := ""
	if req.Ref != nil && *req.Ref != "" {
		ref = *req.Ref
	} else {
		head, err := gitRepo.HEAD()
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "HEAD not found: %v", err)
		}
		ref = head.ID
	}

	// Get tree at ref
	tree, err := gitRepo.LsTree(ref)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "tree not found: %v", err)
	}

	// Get the file entry
	entry, err := tree.TreeEntry(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
	}

	if entry.Type() == "tree" {
		return nil, status.Error(codes.InvalidArgument, "path is a directory, use GetTree instead")
	}

	if entry.Type() == "commit" {
		return nil, status.Error(codes.InvalidArgument, "path is a submodule")
	}

	// Get file contents
	contents, err := entry.Contents()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read file: %v", err)
	}

	// Check if binary
	file := entry.File()
	isBinary, err := file.IsBinary()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if binary: %v", err)
	}

	return &GetBlobResponse{
		Content:  contents,
		Size:     int64(len(contents)),
		IsBinary: isBinary,
		Path:     req.Path,
	}, nil
}

// GetBranches returns all branches for a repository
func (s *Server) GetBranches(ctx context.Context, req *GetBranchesRequest) (*GetBranchesResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	// Get repository
	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	// Get git repository
	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Get all references
	refs, err := gitRepo.References()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
	}

	// Filter for branches only
	branches := make([]*Branch, 0)
	for _, ref := range refs {
		if ref.IsBranch() {
			branches = append(branches, &Branch{
				Name:      ref.Name().Short(),
				FullName:  ref.Name().String(),
				CommitSha: ref.ID,
			})
		}
	}

	return &GetBranchesResponse{
		Branches: branches,
	}, nil
}

// CreateBranch creates a new branch from a source branch or commit SHA
func (s *Server) CreateBranch(ctx context.Context, req *CreateBranchRequest) (*Branch, error) {
	if req.RepoName == "" || req.BranchName == "" || req.Source == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_name, branch_name, and source are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	_, err = git.NewCommand("branch", req.BranchName, req.Source).RunInDir(gitRepo.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create branch: %v", err)
	}

	sha, err := gitRepo.BranchCommitID(req.BranchName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read new branch: %v", err)
	}

	return &Branch{
		Name:      req.BranchName,
		FullName:  "refs/heads/" + req.BranchName,
		CommitSha: sha,
	}, nil
}

// DeleteBranch deletes a branch
func (s *Server) DeleteBranch(ctx context.Context, req *DeleteBranchRequest) (*emptypb.Empty, error) {
	if req.RepoName == "" || req.BranchName == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_name and branch_name are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	if err := gitRepo.DeleteBranch(req.BranchName); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete branch: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ListUserRepositories lists all repositories for a specific user
func (s *Server) ListUserRepositories(ctx context.Context, req *ListUserRepositoriesRequest) (*ListRepositoriesResponse, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	// Get user
	user, err := s.backend.User(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	// Get all repositories and filter by user ID
	allRepos, err := s.backend.Repositories(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get repositories: %v", err)
	}

	// Filter repositories owned by this user
	userRepos := make([]proto.Repository, 0)
	for _, repo := range allRepos {
		if repo.UserID() == user.ID() {
			userRepos = append(userRepos, repo)
		}
	}

	// Convert to proto
	protoRepos := make([]*Repository, len(userRepos))
	for i, repo := range userRepos {
		protoRepos[i] = toProtoRepository(repo)
	}

	return &ListRepositoriesResponse{
		Repositories: protoRepos,
	}, nil
}

// ListCommits returns commit history for a repository with pagination
func (s *Server) ListCommits(ctx context.Context, req *ListCommitsRequest) (*ListCommitsResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	// Get repository
	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	// Open git repository
	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Determine reference to use (default to HEAD)
	refName := "HEAD"
	if req.Ref != nil && *req.Ref != "" {
		refName = *req.Ref
	}

	// Resolve the ref to get a Reference object
	var targetRef *git.Reference

	// Try to get HEAD first
	if refName == "HEAD" {
		targetRef, err = gitRepo.HEAD()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get HEAD: %v", err)
		}
	} else {
		// Get all references to find the matching one
		refs, err := gitRepo.References()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
		}

		// Look for matching branch or tag
		for _, ref := range refs {
			if ref.Name().Short() == refName || ref.Name().String() == refName || ref.ID == refName {
				targetRef = ref
				break
			}
		}

		// If still not found, try to resolve it as a commit SHA directly
		if targetRef == nil {
			// Try to get reference by checking if it exists as a commit
			_, err := gitRepo.CommitByRevision(refName)
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "reference not found: %v", err)
			}
			// Use HEAD but we'll get commits from this SHA
			targetRef, err = gitRepo.HEAD()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get reference: %v", err)
			}
			// Override the ref name to use the commit SHA
			targetRef.ID = refName
		}
	}

	// Set defaults for pagination
	limit := int32(30)
	if req.Limit != nil && *req.Limit > 0 {
		limit = *req.Limit
		if limit > 100 {
			limit = 100 // Cap at 100
		}
	}

	page := int32(1)
	if req.Page != nil && *req.Page > 0 {
		page = *req.Page
	}

	// Get commits using CommitsByPage
	commits, err := gitRepo.CommitsByPage(targetRef, int(page), int(limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get commits: %v", err)
	}

	// Convert to proto commits
	protoCommits := make([]*Commit, len(commits))
	for i, commit := range commits {
		// Get parent IDs
		parentSHAs := make([]string, commit.ParentsCount())
		for j := 0; j < commit.ParentsCount(); j++ {
			parentID, err := commit.ParentID(j)
			if err == nil && parentID != nil {
				parentSHAs[j] = parentID.String()
			}
		}

		protoCommits[i] = &Commit{
			Sha:     commit.ID.String(),
			Message: commit.Message,
			Author: &Author{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  timestamppb.New(commit.Author.When),
			},
			Committer: &Author{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
				When:  timestamppb.New(commit.Committer.When),
			},
			ParentShas: parentSHAs,
		}
	}

	// Check if there are more commits
	hasMore := len(commits) == int(limit)

	return &ListCommitsResponse{
		Commits: protoCommits,
		Page:    page,
		PerPage: limit,
		HasMore: hasMore,
	}, nil
}

// GetCommit returns detailed information about a single commit including diff
func (s *Server) GetCommit(ctx context.Context, req *GetCommitRequest) (*CommitDetail, error) {
	if req.RepoName == "" || req.Sha == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and SHA are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Get the commit
	commit, err := gitRepo.CommitByRevision(req.Sha)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "commit not found: %v", err)
	}

	// Get diff for the commit
	diff, err := gitRepo.Diff(commit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get diff: %v", err)
	}

	// Get patch
	patch, err := gitRepo.Patch(commit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get patch: %v", err)
	}

	// Convert parent IDs
	parentSHAs := make([]string, commit.ParentsCount())
	for j := 0; j < commit.ParentsCount(); j++ {
		parentID, err := commit.ParentID(j)
		if err == nil && parentID != nil {
			parentSHAs[j] = parentID.String()
		}
	}

	// Build file diffs
	fileDiffs := make([]*FileDiff, 0)
	totalAdd := int32(0)
	totalDel := int32(0)

	for _, file := range diff.Files {
		// Determine status based on file names
		status := "modified"
		oldName := file.OldName()
		if oldName == "" || oldName == "/dev/null" {
			status = "added"
		} else if file.Name == "/dev/null" {
			status = "deleted"
		} else if oldName != file.Name {
			status = "renamed"
		}

		fileDiff := &FileDiff{
			Path:      file.Name,
			Additions: int32(file.NumAdditions()),
			Deletions: int32(file.NumDeletions()),
			Status:    status,
		}

		if oldName != "" && oldName != file.Name && oldName != "/dev/null" {
			fileDiff.OldPath = &oldName
		}

		fileDiffs = append(fileDiffs, fileDiff)
		totalAdd += int32(file.NumAdditions())
		totalDel += int32(file.NumDeletions())
	}

	return &CommitDetail{
		Commit: &Commit{
			Sha:     commit.ID.String(),
			Message: commit.Message,
			Author: &Author{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  timestamppb.New(commit.Author.When),
			},
			Committer: &Author{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
				When:  timestamppb.New(commit.Committer.When),
			},
			ParentShas: parentSHAs,
		},
		Files:          fileDiffs,
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
		FilesChanged:   int32(len(fileDiffs)),
		Patch:          patch,
	}, nil
}

// ListTags returns all tags in a repository
func (s *Server) ListTags(ctx context.Context, req *ListTagsRequest) (*ListTagsResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	refs, err := gitRepo.References()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
	}

	tags := make([]*Tag, 0)
	for _, ref := range refs {
		if ref.IsTag() {
			tag := &Tag{
				Name:      ref.Name().Short(),
				FullName:  ref.Name().String(),
				CommitSha: ref.ID,
			}
			tags = append(tags, tag)
		}
	}

	return &ListTagsResponse{Tags: tags}, nil
}

// GetTag returns detailed information about a specific tag
func (s *Server) GetTag(ctx context.Context, req *GetTagRequest) (*TagDetail, error) {
	if req.RepoName == "" || req.TagName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and tag name are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	refs, err := gitRepo.References()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
	}

	var targetRef *git.Reference
	for _, ref := range refs {
		if ref.IsTag() && (ref.Name().Short() == req.TagName || ref.Name().String() == req.TagName) {
			targetRef = ref
			break
		}
	}

	if targetRef == nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}

	commit, err := gitRepo.CommitByRevision(targetRef.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get commit: %v", err)
	}

	parentSHAs := make([]string, commit.ParentsCount())
	for j := 0; j < commit.ParentsCount(); j++ {
		parentID, err := commit.ParentID(j)
		if err == nil && parentID != nil {
			parentSHAs[j] = parentID.String()
		}
	}

	return &TagDetail{
		Tag: &Tag{
			Name:      targetRef.Name().Short(),
			FullName:  targetRef.Name().String(),
			CommitSha: targetRef.ID,
		},
		Commit: &Commit{
			Sha:     commit.ID.String(),
			Message: commit.Message,
			Author: &Author{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  timestamppb.New(commit.Author.When),
			},
			Committer: &Author{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
				When:  timestamppb.New(commit.Committer.When),
			},
			ParentShas: parentSHAs,
		},
	}, nil
}

// CreateTag creates a new tag
func (s *Server) CreateTag(ctx context.Context, req *CreateTagRequest) (*TagDetail, error) {
	if req.RepoName == "" || req.TagName == "" || req.Target == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name, tag name, and target are required")
	}

	// Note: Tag creation via gRPC is not supported in this version
	// Tags should be created via git push
	return nil, status.Error(codes.Unimplemented, "tag creation is not supported via gRPC - use git push instead")
}

// DeleteTag deletes a tag
func (s *Server) DeleteTag(ctx context.Context, req *DeleteTagRequest) (*emptypb.Empty, error) {
	if req.RepoName == "" || req.TagName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and tag name are required")
	}

	// Note: Tag deletion via gRPC is not supported in this version
	// Tags should be deleted via git push
	return nil, status.Error(codes.Unimplemented, "tag deletion is not supported via gRPC - use git push instead")
}

// CompareBranches compares two branches
func (s *Server) CompareBranches(ctx context.Context, req *CompareBranchesRequest) (*CompareResponse, error) {
	if req.RepoName == "" || req.BaseBranch == "" || req.HeadBranch == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name, base branch, and head branch are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Resolve branches to commit SHAs
	baseCommit, err := gitRepo.CommitByRevision(req.BaseBranch)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "base branch not found: %v", err)
	}

	headCommit, err := gitRepo.CommitByRevision(req.HeadBranch)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "head branch not found: %v", err)
	}

	return s.compareCommits(gitRepo, baseCommit.ID.String(), headCommit.ID.String())
}

// CompareCommits compares two commits
func (s *Server) CompareCommits(ctx context.Context, req *CompareCommitsRequest) (*CompareResponse, error) {
	if req.RepoName == "" || req.BaseSha == "" || req.HeadSha == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name, base SHA, and head SHA are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	return s.compareCommits(gitRepo, req.BaseSha, req.HeadSha)
}

// GetDefaultBranch returns the default branch of a repository
func (s *Server) GetDefaultBranch(ctx context.Context, req *GetDefaultBranchRequest) (*DefaultBranchResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	head, err := gitRepo.HEAD()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get HEAD: %v", err)
	}

	return &DefaultBranchResponse{
		BranchName: head.Name().Short(),
	}, nil
}

// SetDefaultBranch sets the default branch of a repository
func (s *Server) SetDefaultBranch(ctx context.Context, req *SetDefaultBranchRequest) (*DefaultBranchResponse, error) {
	if req.RepoName == "" || req.BranchName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and branch name are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Set symbolic ref
	branchRef := "refs/heads/" + req.BranchName
	_, err = gitRepo.SymbolicRef("HEAD", branchRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set default branch: %v", err)
	}

	return &DefaultBranchResponse{
		BranchName: req.BranchName,
	}, nil
}

// GetCloneURLs returns clone URLs for a repository
func (s *Server) GetCloneURLs(ctx context.Context, req *GetCloneURLsRequest) (*CloneURLsResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	// Build URLs from configuration
	sshURL := ""
	if s.config.SSH.PublicURL != "" {
		sshURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.config.SSH.PublicURL, "/"), req.RepoName)
	} else if s.config.SSH.Enabled {
		sshURL = fmt.Sprintf("ssh://%s/%s", s.config.SSH.ListenAddr, req.RepoName)
	}

	httpURL := ""
	if s.config.HTTP.PublicURL != "" {
		httpURL = fmt.Sprintf("%s/%s.git", strings.TrimSuffix(s.config.HTTP.PublicURL, "/"), req.RepoName)
	} else if s.config.HTTP.Enabled {
		scheme := "http"
		if s.config.HTTP.TLSCertPath != "" && s.config.HTTP.TLSKeyPath != "" {
			scheme = "https"
		}
		httpURL = fmt.Sprintf("%s://%s/%s.git", scheme, s.config.HTTP.ListenAddr, req.RepoName)
	}

	gitURL := ""
	if s.config.Git.Enabled {
		gitURL = fmt.Sprintf("git://%s/%s", s.config.Git.ListenAddr, req.RepoName)
	}

	return &CloneURLsResponse{
		SshUrl:  sshURL,
		HttpUrl: httpURL,
		GitUrl:  gitURL,
	}, nil
}

// GetRepositoryStats returns statistics about a repository
func (s *Server) GetRepositoryStats(ctx context.Context, req *GetRepositoryStatsRequest) (*RepositoryStatsResponse, error) {
	if req.RepoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name is required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	// Get HEAD for commit count
	head, err := gitRepo.HEAD()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get HEAD: %v", err)
	}

	// Count commits
	commitCount, err := gitRepo.CountCommits(head)
	if err != nil {
		commitCount = 0
	}

	// Get all references
	refs, err := gitRepo.References()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
	}

	branchCount := 0
	tagCount := 0
	for _, ref := range refs {
		if ref.IsBranch() {
			branchCount++
		} else if ref.IsTag() {
			tagCount++
		}
	}

	// Get latest commit for last commit time
	commits, err := gitRepo.CommitsByPage(head, 1, 1)
	var lastCommit *timestamppb.Timestamp
	if err == nil && len(commits) > 0 {
		lastCommit = timestamppb.New(commits[0].Committer.When)
	}

	return &RepositoryStatsResponse{
		SizeBytes:        0, // Would need to walk the repo directory
		CommitCount:      commitCount,
		BranchCount:      int32(branchCount),
		TagCount:         int32(tagCount),
		ContributorCount: 0, // Would need to analyze all commits
		LastCommit:       lastCommit,
	}, nil
}

// GetFileHistory returns commit history for a specific file
func (s *Server) GetFileHistory(ctx context.Context, req *GetFileHistoryRequest) (*GetFileHistoryResponse, error) {
	if req.RepoName == "" || req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and path are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	refName := "HEAD"
	if req.Ref != nil && *req.Ref != "" {
		refName = *req.Ref
	}

	limit := int32(50)
	if req.Limit != nil && *req.Limit > 0 {
		limit = *req.Limit
	}

	// Get all commits and filter manually (simplified implementation)
	// In a production system, this should use git log --follow -- path
	var targetRef *git.Reference
	if refName == "HEAD" {
		targetRef, err = gitRepo.HEAD()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get HEAD: %v", err)
		}
	} else {
		refs, err := gitRepo.References()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
		}
		for _, ref := range refs {
			if ref.Name().Short() == refName {
				targetRef = ref
				break
			}
		}
		if targetRef == nil {
			return nil, status.Error(codes.NotFound, "reference not found")
		}
	}

	// Get all commits (simplified - doesn't filter by file)
	commits, err := gitRepo.CommitsByPage(targetRef, 1, int(limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get commits: %v", err)
	}

	protoCommits := make([]*Commit, len(commits))
	for i, commit := range commits {
		parentSHAs := make([]string, commit.ParentsCount())
		for j := 0; j < commit.ParentsCount(); j++ {
			parentID, err := commit.ParentID(j)
			if err == nil && parentID != nil {
				parentSHAs[j] = parentID.String()
			}
		}

		protoCommits[i] = &Commit{
			Sha:     commit.ID.String(),
			Message: commit.Message,
			Author: &Author{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  timestamppb.New(commit.Author.When),
			},
			Committer: &Author{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
				When:  timestamppb.New(commit.Committer.When),
			},
			ParentShas: parentSHAs,
		}
	}

	return &GetFileHistoryResponse{
		Commits: protoCommits,
	}, nil
}

// SearchCommits searches for commits by message or author
func (s *Server) SearchCommits(ctx context.Context, req *SearchCommitsRequest) (*ListCommitsResponse, error) {
	if req.RepoName == "" || req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and query are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	refName := "HEAD"
	if req.Ref != nil && *req.Ref != "" {
		refName = *req.Ref
	}

	limit := int32(30)
	if req.Limit != nil && *req.Limit > 0 {
		limit = *req.Limit
		if limit > 100 {
			limit = 100
		}
	}

	// Get reference
	var targetRef *git.Reference
	if refName == "HEAD" {
		targetRef, err = gitRepo.HEAD()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get HEAD: %v", err)
		}
	} else {
		refs, err := gitRepo.References()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get references: %v", err)
		}

		for _, ref := range refs {
			if ref.Name().Short() == refName || ref.Name().String() == refName {
				targetRef = ref
				break
			}
		}

		if targetRef == nil {
			return nil, status.Error(codes.NotFound, "reference not found")
		}
	}

	// Get commits and filter
	commits, err := gitRepo.CommitsByPage(targetRef, 1, int(limit)*5) // Get more to filter
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get commits: %v", err)
	}

	// Filter commits by query
	filteredCommits := make([]*Commit, 0)
	query := req.Query
	author := ""
	if req.Author != nil {
		author = *req.Author
	}

	for _, commit := range commits {
		if len(filteredCommits) >= int(limit) {
			break
		}

		// Check if commit matches query
		matchesQuery := strings.Contains(strings.ToLower(commit.Message), strings.ToLower(query))
		matchesAuthor := author == "" || strings.Contains(strings.ToLower(commit.Author.Name), strings.ToLower(author))

		if matchesQuery && matchesAuthor {
			parentSHAs := make([]string, commit.ParentsCount())
			for j := 0; j < commit.ParentsCount(); j++ {
				parentID, err := commit.ParentID(j)
				if err == nil && parentID != nil {
					parentSHAs[j] = parentID.String()
				}
			}

			filteredCommits = append(filteredCommits, &Commit{
				Sha:     commit.ID.String(),
				Message: commit.Message,
				Author: &Author{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
					When:  timestamppb.New(commit.Author.When),
				},
				Committer: &Author{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
					When:  timestamppb.New(commit.Committer.When),
				},
				ParentShas: parentSHAs,
			})
		}
	}

	return &ListCommitsResponse{
		Commits: filteredCommits,
		Page:    1,
		PerPage: limit,
		HasMore: len(filteredCommits) == int(limit),
	}, nil
}

// CheckPath checks if a path exists in a repository
func (s *Server) CheckPath(ctx context.Context, req *CheckPathRequest) (*CheckPathResponse, error) {
	if req.RepoName == "" || req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "repository name and path are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	refName := "HEAD"
	if req.Ref != nil && *req.Ref != "" {
		refName = *req.Ref
	}

	tree, err := gitRepo.LsTree(refName)
	if err != nil {
		return &CheckPathResponse{Exists: false}, nil
	}

	// Navigate to the path
	if req.Path != "" && req.Path != "." && req.Path != "/" {
		tree, err = tree.SubTree(req.Path)
		if err != nil {
			// Try as a file
			parentTree := tree
			entry, _ := parentTree.TreeEntry(req.Path)
			if entry == nil {
				return &CheckPathResponse{Exists: false}, nil
			}

			isFile := entry.Type() == "blob"
			size := int64(0)
			if isFile {
				file := entry.File()
				size = file.Size()
			}

			return &CheckPathResponse{
				Exists: true,
				IsDir:  false,
				IsFile: isFile,
				Size:   &size,
			}, nil
		}
	}

	// It's a directory
	return &CheckPathResponse{
		Exists: true,
		IsDir:  true,
		IsFile: false,
	}, nil
}

// MergeBranches merges headBranch into baseBranch in the given repository.
func (s *Server) MergeBranches(ctx context.Context, req *MergeBranchesRequest) (*MergeBranchesResponse, error) {
	if req.RepoName == "" || req.BaseBranch == "" || req.HeadBranch == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_name, base_branch, and head_branch are required")
	}

	repo, err := s.backend.Repository(ctx, req.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "repository not found: %v", err)
	}

	gitRepo, err := repo.Open()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open repository: %v", err)
	}

	baseSHA, err := gitRepo.BranchCommitID(req.BaseBranch)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "base branch %q not found: %v", req.BaseBranch, err)
	}
	headSHA, err := gitRepo.BranchCommitID(req.HeadBranch)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "head branch %q not found: %v", req.HeadBranch, err)
	}

	repoPath := gitRepo.Path

	// Find the merge base to check fast-forward / already-merged cases.
	mergeBaseOut, err := git.NewCommand("merge-base", baseSHA, headSHA).RunInDir(repoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find merge base: %v", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	// headSHA is already an ancestor of baseSHA – nothing to do.
	if mergeBase == headSHA {
		return &MergeBranchesResponse{Merged: false, Sha: baseSHA, Message: "Already up to date"}, nil
	}

	// baseSHA is an ancestor of headSHA – simple fast-forward.
	if mergeBase == baseSHA {
		if _, err := git.NewCommand("update-ref", "refs/heads/"+req.BaseBranch, headSHA).RunInDir(repoPath); err != nil {
			return nil, status.Errorf(codes.Internal, "fast-forward update-ref failed: %v", err)
		}
		return &MergeBranchesResponse{Merged: true, Sha: headSHA, Message: "Fast-forward"}, nil
	}

	committerName := req.CommitterName
	if committerName == "" {
		committerName = "Gyt"
	}
	committerEmail := req.CommitterEmail
	if committerEmail == "" {
		committerEmail = "noreply@gyt.local"
	}
	commitTitle := req.CommitTitle
	if commitTitle == "" {
		commitTitle = "Merge branch '" + req.HeadBranch + "'"
	}
	mergeMethod := req.MergeMethod
	if mergeMethod == "" {
		mergeMethod = "merge"
	}

	envs := []string{
		"GIT_COMMITTER_NAME=" + committerName,
		"GIT_COMMITTER_EMAIL=" + committerEmail,
		"GIT_AUTHOR_NAME=" + committerName,
		"GIT_AUTHOR_EMAIL=" + committerEmail,
	}

	// Compute the merged tree (git 2.38+). Exit code 1 means conflicts.
	mergeTreeOut, mergeTreeErr := git.NewCommand("merge-tree", "--write-tree", baseSHA, headSHA).
		AddEnvs(envs...).RunInDir(repoPath)
	if mergeTreeErr != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "merge conflicts detected: %v", mergeTreeErr)
	}
	treeSHA := strings.TrimSpace(strings.SplitN(string(mergeTreeOut), "\n", 2)[0])

	// Build the merge (or squash) commit.
	var commitArgs []string
	commitArgs = append(commitArgs, "commit-tree", treeSHA, "-p", baseSHA)
	if mergeMethod != "squash" {
		// merge or rebase: include head as a second parent.
		commitArgs = append(commitArgs, "-p", headSHA)
	}
	commitArgs = append(commitArgs, "-m", commitTitle)

	commitOut, err := git.NewCommand(commitArgs...).AddEnvs(envs...).RunInDir(repoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "commit-tree failed: %v", err)
	}
	newSHA := strings.TrimSpace(string(commitOut))

	// Advance the base branch ref to the new commit.
	if _, err := git.NewCommand("update-ref", "refs/heads/"+req.BaseBranch, newSHA).RunInDir(repoPath); err != nil {
		return nil, status.Errorf(codes.Internal, "update-ref failed: %v", err)
	}

	return &MergeBranchesResponse{Merged: true, Sha: newSHA, Message: commitTitle}, nil
}

// Helper function for comparing commits
func (s *Server) compareCommits(gitRepo *git.Repository, baseSHA, headSHA string) (*CompareResponse, error) {
	// Get the full diff between base and head (all changes introduced by head relative to base)
	diff, err := gitRepo.DiffBetween(baseSHA, headSHA)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get diff: %v", err)
	}

	// Build file diffs
	fileDiffs := make([]*FileDiff, 0)
	totalAdd := int32(0)
	totalDel := int32(0)

	for _, file := range diff.Files {
		fileStatus := "modified"
		oldName := file.OldName()
		if oldName == "" || oldName == "/dev/null" {
			fileStatus = "added"
		} else if file.Name == "/dev/null" {
			fileStatus = "deleted"
		} else if oldName != file.Name {
			fileStatus = "renamed"
		}

		fileDiff := &FileDiff{
			Path:      file.Name,
			Additions: int32(file.NumAdditions()),
			Deletions: int32(file.NumDeletions()),
			Status:    fileStatus,
		}

		if oldName != "" && oldName != file.Name && oldName != "/dev/null" {
			fileDiff.OldPath = &oldName
		}

		fileDiffs = append(fileDiffs, fileDiff)
		totalAdd += int32(file.NumAdditions())
		totalDel += int32(file.NumDeletions())
	}

	// Get commits in head that are not in base
	betweenCommits, err := gitRepo.CommitsBetween(baseSHA, headSHA)
	if err != nil {
		// Non-fatal: return empty list if the rev-list fails
		betweenCommits = nil
	}

	protoCommits := make([]*Commit, 0, len(betweenCommits))
	for _, c := range betweenCommits {
		parentSHAs := make([]string, c.ParentsCount())
		for j := 0; j < c.ParentsCount(); j++ {
			pid, perr := c.ParentID(j)
			if perr == nil && pid != nil {
				parentSHAs[j] = pid.String()
			}
		}
		protoCommits = append(protoCommits, &Commit{
			Sha:     c.ID.String(),
			Message: c.Message,
			Author: &Author{
				Name:  c.Author.Name,
				Email: c.Author.Email,
				When:  timestamppb.New(c.Author.When),
			},
			Committer: &Author{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
				When:  timestamppb.New(c.Committer.When),
			},
			ParentShas: parentSHAs,
		})
	}

	return &CompareResponse{
		Commits:        protoCommits,
		Files:          fileDiffs,
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
		FilesChanged:   int32(len(fileDiffs)),
		CommitsAhead:   int32(len(protoCommits)),
		Patch:          diff.Patch(),
	}, nil
}

// Helper functions

func toProtoRepository(repo proto.Repository) *Repository {
	return &Repository{
		Id:          repo.ID(),
		Name:        repo.Name(),
		ProjectName: repo.ProjectName(),
		Description: repo.Description(),
		IsPrivate:   repo.IsPrivate(),
		IsMirror:    repo.IsMirror(),
		IsHidden:    repo.IsHidden(),
		UserId:      repo.UserID(),
		CreatedAt:   timestamppb.New(repo.CreatedAt()),
		UpdatedAt:   timestamppb.New(repo.UpdatedAt()),
	}
}

func toProtoUser(user proto.User) *User {
	pks := user.PublicKeys()
	keyStrs := make([]string, len(pks))
	for i, pk := range pks {
		keyStrs[i] = string(ssh.MarshalAuthorizedKey(pk))
	}

	return &User{
		Id:         user.ID(),
		Username:   user.Username(),
		IsAdmin:    user.IsAdmin(),
		PublicKeys: keyStrs,
	}
}

func toProtoAccessLevel(level access.AccessLevel) AccessLevel {
	switch level {
	case access.NoAccess:
		return AccessLevel_NO_ACCESS
	case access.ReadOnlyAccess:
		return AccessLevel_READ_ONLY
	case access.ReadWriteAccess:
		return AccessLevel_READ_WRITE
	case access.AdminAccess:
		return AccessLevel_ADMIN_ACCESS
	default:
		return AccessLevel_ACCESS_LEVEL_UNSPECIFIED
	}
}

func toBackendAccessLevel(level AccessLevel) access.AccessLevel {
	switch level {
	case AccessLevel_NO_ACCESS:
		return access.NoAccess
	case AccessLevel_READ_ONLY:
		return access.ReadOnlyAccess
	case AccessLevel_READ_WRITE:
		return access.ReadWriteAccess
	case AccessLevel_ADMIN_ACCESS:
		return access.AdminAccess
	default:
		return access.NoAccess
	}
}

func toProtoWebhook(hook webhook.Hook) *Webhook {
	events := hook.Events
	eventStrs := make([]string, len(events))
	for i, e := range events {
		eventStrs[i] = e.String()
	}

	contentType := "json"
	if hook.ContentType == webhook.ContentTypeForm {
		contentType = "form"
	}

	return &Webhook{
		Id:          hook.ID,
		Url:         hook.URL,
		ContentType: contentType,
		Events:      eventStrs,
		Active:      hook.Active,
		CreatedAt:   timestamppb.New(hook.CreatedAt),
		UpdatedAt:   timestamppb.New(hook.UpdatedAt),
	}
}
