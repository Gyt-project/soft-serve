package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/soft-serve/pkg/access"
	"github.com/charmbracelet/soft-serve/pkg/backend"
	"github.com/charmbracelet/soft-serve/pkg/proto"
	"github.com/charmbracelet/soft-serve/pkg/version"
	"github.com/charmbracelet/soft-serve/pkg/webhook"
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
	ctx     context.Context
}

// NewServer creates a new gRPC server instance
func NewServer(ctx context.Context, be *backend.Backend) *Server {
	return &Server{
		backend: be,
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
