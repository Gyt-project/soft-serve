package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client wraps the gRPC client for easy use
type Client struct {
	conn   *grpc.ClientConn
	client GitServerManagementClient
	addr   string
}

// NewClient creates a new gRPC client connected to the specified address
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Client{
		conn:   conn,
		client: NewGitServerManagementClient(conn),
		addr:   addr,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Repository Management

func (c *Client) CreateRepository(ctx context.Context, req *CreateRepositoryRequest) (*Repository, error) {
	return c.client.CreateRepository(ctx, req)
}

func (c *Client) DeleteRepository(ctx context.Context, req *DeleteRepositoryRequest) error {
	_, err := c.client.DeleteRepository(ctx, req)
	return err
}

func (c *Client) GetRepository(ctx context.Context, req *GetRepositoryRequest) (*Repository, error) {
	return c.client.GetRepository(ctx, req)
}

func (c *Client) ListRepositories(ctx context.Context, req *ListRepositoriesRequest) (*ListRepositoriesResponse, error) {
	return c.client.ListRepositories(ctx, req)
}

func (c *Client) RenameRepository(ctx context.Context, req *RenameRepositoryRequest) (*Repository, error) {
	return c.client.RenameRepository(ctx, req)
}

func (c *Client) UpdateRepository(ctx context.Context, req *UpdateRepositoryRequest) (*Repository, error) {
	return c.client.UpdateRepository(ctx, req)
}

func (c *Client) ImportRepository(ctx context.Context, req *ImportRepositoryRequest) (*Repository, error) {
	return c.client.ImportRepository(ctx, req)
}

// Repository Content Browsing

func (c *Client) GetTree(ctx context.Context, req *GetTreeRequest) (*GetTreeResponse, error) {
	return c.client.GetTree(ctx, req)
}

func (c *Client) GetBlob(ctx context.Context, req *GetBlobRequest) (*GetBlobResponse, error) {
	return c.client.GetBlob(ctx, req)
}

func (c *Client) GetBranches(ctx context.Context, req *GetBranchesRequest) (*GetBranchesResponse, error) {
	return c.client.GetBranches(ctx, req)
}

func (c *Client) ListCommits(ctx context.Context, req *ListCommitsRequest) (*ListCommitsResponse, error) {
	return c.client.ListCommits(ctx, req)
}

func (c *Client) GetCommit(ctx context.Context, req *GetCommitRequest) (*CommitDetail, error) {
	return c.client.GetCommit(ctx, req)
}

func (c *Client) ListUserRepositories(ctx context.Context, req *ListUserRepositoriesRequest) (*ListRepositoriesResponse, error) {
	return c.client.ListUserRepositories(ctx, req)
}

// Tags

func (c *Client) ListTags(ctx context.Context, req *ListTagsRequest) (*ListTagsResponse, error) {
	return c.client.ListTags(ctx, req)
}

func (c *Client) GetTag(ctx context.Context, req *GetTagRequest) (*TagDetail, error) {
	return c.client.GetTag(ctx, req)
}

func (c *Client) CreateTag(ctx context.Context, req *CreateTagRequest) (*TagDetail, error) {
	return c.client.CreateTag(ctx, req)
}

func (c *Client) DeleteTag(ctx context.Context, req *DeleteTagRequest) error {
	_, err := c.client.DeleteTag(ctx, req)
	return err
}

// Diff & Compare

func (c *Client) CompareBranches(ctx context.Context, req *CompareBranchesRequest) (*CompareResponse, error) {
	return c.client.CompareBranches(ctx, req)
}

func (c *Client) CompareCommits(ctx context.Context, req *CompareCommitsRequest) (*CompareResponse, error) {
	return c.client.CompareCommits(ctx, req)
}

// Repository Info

func (c *Client) GetDefaultBranch(ctx context.Context, req *GetDefaultBranchRequest) (*DefaultBranchResponse, error) {
	return c.client.GetDefaultBranch(ctx, req)
}

func (c *Client) SetDefaultBranch(ctx context.Context, req *SetDefaultBranchRequest) (*DefaultBranchResponse, error) {
	return c.client.SetDefaultBranch(ctx, req)
}

func (c *Client) GetCloneURLs(ctx context.Context, req *GetCloneURLsRequest) (*CloneURLsResponse, error) {
	return c.client.GetCloneURLs(ctx, req)
}

func (c *Client) GetRepositoryStats(ctx context.Context, req *GetRepositoryStatsRequest) (*RepositoryStatsResponse, error) {
	return c.client.GetRepositoryStats(ctx, req)
}

// Advanced Operations

func (c *Client) GetFileHistory(ctx context.Context, req *GetFileHistoryRequest) (*GetFileHistoryResponse, error) {
	return c.client.GetFileHistory(ctx, req)
}

func (c *Client) SearchCommits(ctx context.Context, req *SearchCommitsRequest) (*ListCommitsResponse, error) {
	return c.client.SearchCommits(ctx, req)
}

func (c *Client) CheckPath(ctx context.Context, req *CheckPathRequest) (*CheckPathResponse, error) {
	return c.client.CheckPath(ctx, req)
}

// User Management

func (c *Client) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	return c.client.CreateUser(ctx, req)
}

func (c *Client) DeleteUser(ctx context.Context, req *DeleteUserRequest) error {
	_, err := c.client.DeleteUser(ctx, req)
	return err
}

func (c *Client) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	return c.client.GetUser(ctx, req)
}

func (c *Client) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	return c.client.ListUsers(ctx, req)
}

func (c *Client) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*User, error) {
	return c.client.UpdateUser(ctx, req)
}

// User Public Keys

func (c *Client) AddPublicKey(ctx context.Context, req *AddPublicKeyRequest) error {
	_, err := c.client.AddPublicKey(ctx, req)
	return err
}

func (c *Client) RemovePublicKey(ctx context.Context, req *RemovePublicKeyRequest) error {
	_, err := c.client.RemovePublicKey(ctx, req)
	return err
}

func (c *Client) ListPublicKeys(ctx context.Context, req *ListPublicKeysRequest) (*ListPublicKeysResponse, error) {
	return c.client.ListPublicKeys(ctx, req)
}

// Collaborator Management

func (c *Client) AddCollaborator(ctx context.Context, req *AddCollaboratorRequest) error {
	_, err := c.client.AddCollaborator(ctx, req)
	return err
}

func (c *Client) RemoveCollaborator(ctx context.Context, req *RemoveCollaboratorRequest) error {
	_, err := c.client.RemoveCollaborator(ctx, req)
	return err
}

func (c *Client) ListCollaborators(ctx context.Context, req *ListCollaboratorsRequest) (*ListCollaboratorsResponse, error) {
	return c.client.ListCollaborators(ctx, req)
}

// Access Token Management

func (c *Client) CreateAccessToken(ctx context.Context, req *CreateAccessTokenRequest) (*AccessToken, error) {
	return c.client.CreateAccessToken(ctx, req)
}

func (c *Client) DeleteAccessToken(ctx context.Context, req *DeleteAccessTokenRequest) error {
	_, err := c.client.DeleteAccessToken(ctx, req)
	return err
}

func (c *Client) ListAccessTokens(ctx context.Context, req *ListAccessTokensRequest) (*ListAccessTokensResponse, error) {
	return c.client.ListAccessTokens(ctx, req)
}

// Webhook Management

func (c *Client) CreateWebhook(ctx context.Context, req *CreateWebhookRequest) (*Webhook, error) {
	return c.client.CreateWebhook(ctx, req)
}

func (c *Client) DeleteWebhook(ctx context.Context, req *DeleteWebhookRequest) error {
	_, err := c.client.DeleteWebhook(ctx, req)
	return err
}

func (c *Client) GetWebhook(ctx context.Context, req *GetWebhookRequest) (*Webhook, error) {
	return c.client.GetWebhook(ctx, req)
}

func (c *Client) ListWebhooks(ctx context.Context, req *ListWebhooksRequest) (*ListWebhooksResponse, error) {
	return c.client.ListWebhooks(ctx, req)
}

func (c *Client) UpdateWebhook(ctx context.Context, req *UpdateWebhookRequest) (*Webhook, error) {
	return c.client.UpdateWebhook(ctx, req)
}

// Server Settings

func (c *Client) GetSettings(ctx context.Context) (*ServerSettings, error) {
	return c.client.GetSettings(ctx, &emptypb.Empty{})
}

func (c *Client) UpdateSettings(ctx context.Context, req *UpdateSettingsRequest) (*ServerSettings, error) {
	return c.client.UpdateSettings(ctx, req)
}

// Health Check

func (c *Client) HealthCheck(ctx context.Context) (*HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, &emptypb.Empty{})
}
