# gRPC Management API

The Soft Serve gRPC Management API provides full administrative control over the git server. This API is designed for internal use and assumes all connections have full administrative privileges.

## Configuration

Enable and configure the gRPC server in your config file:

```yaml
grpc:
  enabled: true
  listen_addr: "localhost:23234"
```

Or via environment variables:

```bash
export SOFT_SERVE_GRPC_ENABLED=true
export SOFT_SERVE_GRPC_LISTEN_ADDR="localhost:23234"
```

## Default Ports

- **23231** - SSH server
- **23232** - HTTP server
- **23233** - Stats/Metrics server
- **23234** - gRPC Management API (new)

## API Overview

The gRPC API provides comprehensive management capabilities:

### Repository Management
- `CreateRepository` - Create a new repository
- `DeleteRepository` - Delete a repository
- `GetRepository` - Get repository details
- `ListRepositories` - List all repositories
- `RenameRepository` - Rename a repository
- `UpdateRepository` - Update repository settings (description, visibility, etc.)
- `ImportRepository` - Import a repository from a remote URL

### Repository Content Browsing
- `GetTree` - List directory contents at a specific path
- `GetBlob` - Get file contents at a specific path
- `GetBranches` - Get all branches for a repository
- `ListCommits` - Get commit history with pagination (GitHub-style)
- `GetCommit` - Get detailed commit information with diff/patch
- `ListUserRepositories` - List all repositories owned by a specific user

### Tags Management
- `ListTags` - Get all tags with commit information
- `GetTag` - Get detailed information about a specific tag
- `CreateTag` - Create a new tag (Note: Currently returns unimplemented - use git push)
- `DeleteTag` - Delete a tag (Note: Currently returns unimplemented - use git push)

### Compare & Diff
- `CompareBranches` - Compare two branches and get diff
- `CompareCommits` - Compare two commits and get diff

### Repository Information
- `GetDefaultBranch` - Get the repository's default branch
- `SetDefaultBranch` - Change the default branch
- `GetCloneURLs` - Get SSH, HTTP, and Git clone URLs
- `GetRepositoryStats` - Get repository statistics (size, commits, branches, tags, etc.)

### Advanced Operations
- `GetFileHistory` - Get commit history for a specific file path
- `SearchCommits` - Search commits by message or author
- `CheckPath` - Check if a file or directory exists at a reference

### User Management
- `CreateUser` - Create a new user with optional admin rights
- `DeleteUser` - Delete a user
- `GetUser` - Get user details
- `ListUsers` - List all users
- `UpdateUser` - Update user settings (username, admin status, password)

### Public Key Management
- `AddPublicKey` - Add an SSH public key to a user
- `RemovePublicKey` - Remove an SSH public key from a user
- `ListPublicKeys` - List all public keys for a user

### Collaborator Management
- `AddCollaborator` - Add a collaborator to a repository with specific access level
- `RemoveCollaborator` - Remove a collaborator from a repository
- `ListCollaborators` - List all collaborators for a repository

### Access Token Management
- `CreateAccessToken` - Create an access token for a user
- `DeleteAccessToken` - Delete an access token
- `ListAccessTokens` - List all access tokens for a user

### Webhook Management
- `CreateWebhook` - Create a webhook for a repository
- `DeleteWebhook` - Delete a webhook
- `GetWebhook` - Get webhook details
- `ListWebhooks` - List all webhooks for a repository
- `UpdateWebhook` - Update webhook settings

### Server Settings
- `GetSettings` - Get server settings
- `UpdateSettings` - Update server settings (allow keyless access, anonymous access level)

### Health Check
- `HealthCheck` - Check server health and version

## Usage Examples

### Using grpcurl

Install grpcurl:
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

List available services:
```bash
grpcurl -plaintext localhost:23234 list
```

List methods for the management service:
```bash
grpcurl -plaintext localhost:23234 list softserve.GitServerManagement
```

Describe a method:
```bash
grpcurl -plaintext localhost:23234 describe softserve.GitServerManagement.CreateRepository
```

### Example: Create a Repository

```bash
grpcurl -plaintext -d '{
  "name": "my-repo",
  "description": "My new repository",
  "private": false,
  "project_name": "My Project"
}' localhost:23234 softserve.GitServerManagement/CreateRepository
```

### Example: Create a User

```bash
grpcurl -plaintext -d '{
  "username": "alice",
  "admin": true,
  "password": "secure-password"
}' localhost:23234 softserve.GitServerManagement/CreateUser
```

### Example: Add Public Key

```bash
grpcurl -plaintext -d '{
  "username": "alice",
  "public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... alice@example.com"
}' localhost:23234 softserve.GitServerManagement/AddPublicKey
```

### Example: List Repositories

```bash
grpcurl -plaintext localhost:23234 softserve.GitServerManagement/ListRepositories
```

### Example: Add Collaborator

```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "username": "bob",
  "access_level": "READ_WRITE"
}' localhost:23234 softserve.GitServerManagement/AddCollaborator
```

### Example: Create Webhook

```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "url": "https://example.com/webhook",
  "content_type": "json",
  "events": ["push", "branch_tag_create"],
  "active": true
}' localhost:23234 softserve.GitServerManagement/CreateWebhook
```

### Example: Update Repository

```bash
grpcurl -plaintext -d '{
  "name": "my-repo",
  "description": "Updated description",
  "is_private": true
}' localhost:23234 softserve.GitServerManagement/UpdateRepository
```

### Example: Health Check

```bash
grpcurl -plaintext localhost:23234 softserve.GitServerManagement/HealthCheck
```

### Example: Browse Repository Tree

List files in the root directory:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/GetTree
```

List files in a specific directory:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "path": "cmd/soft"
}' localhost:23234 softserve.GitServerManagement/GetTree
```

List files at a specific branch or tag:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "ref": "main",
  "path": "pkg"
}' localhost:23234 softserve.GitServerManagement/GetTree
```

### Example: Get File Contents

Get file from default branch (HEAD):
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "path": "README.md"
}' localhost:23234 softserve.GitServerManagement/GetBlob
```

Get file from specific branch:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "ref": "develop",
  "path": "cmd/soft/main.go"
}' localhost:23234 softserve.GitServerManagement/GetBlob
```

Get file from specific commit:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "ref": "abc123def",
  "path": "config.yaml"
}' localhost:23234 softserve.GitServerManagement/GetBlob
```

### Example: Get Repository Branches

Get all branches for a repository:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/GetBranches
```

This returns all branches with their names, full reference names, and commit SHAs:
```json
{
  "branches": [
    {
      "name": "main",
      "fullName": "refs/heads/main",
      "commitSha": "abc123..."
    },
    {
      "name": "develop",
      "fullName": "refs/heads/develop",
      "commitSha": "def456..."
    }
  ]
}
```

### Example: List User Repositories

Get all repositories owned by a specific user:
```bash
grpcurl -plaintext -d '{
  "username": "john"
}' localhost:23234 softserve.GitServerManagement/ListUserRepositories
```

This returns the same format as `ListRepositories` but filtered to only repos owned by the specified user.

### Example: List Commits (Paginated)

Get the latest 30 commits from the default branch (HEAD):
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/ListCommits
```

Get commits from a specific branch with custom limit:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "ref": "main",
  "limit": 10
}' localhost:23234 softserve.GitServerManagement/ListCommits
```

Get second page of commits (pagination):
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "ref": "develop",
  "limit": 20,
  "page": 2
}' localhost:23234 softserve.GitServerManagement/ListCommits
```

Response includes commit details and pagination info:
```json
{
  "commits": [
    {
      "sha": "abc123...",
      "message": "Add new feature",
      "author": {
        "name": "John Doe",
        "email": "john@example.com",
        "when": "2026-02-04T10:30:00Z"
      },
      "committer": {
        "name": "John Doe",
        "email": "john@example.com",
        "when": "2026-02-04T10:30:00Z"
      },
      "parentShas": ["def456..."]
    }
  ],
  "page": 1,
  "perPage": 30,
  "hasMore": true
}
```

**Pagination Parameters:**
- `limit`: Number of commits per page (default: 30, max: 100)
- `page`: Page number starting from 1 (default: 1)
- `ref`: Branch, tag, or commit SHA to start from (default: HEAD)
- `hasMore`: Indicates if more commits are available

### Example: Get Commit Details with Diff

Get full commit information including file changes and patch:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "sha": "abc123def456"
}' localhost:23234 softserve.GitServerManagement/GetCommit
```

Response includes commit info, file diffs, and patch:
```json
{
  "commit": {
    "sha": "abc123...",
    "message": "Add new feature",
    "author": {...},
    "committer": {...}
  },
  "files": [
    {
      "path": "src/main.go",
      "additions": 15,
      "deletions": 3,
      "status": "modified"
    }
  ],
  "totalAdditions": 15,
  "totalDeletions": 3,
  "filesChanged": 1,
  "patch": "diff --git a/src/main.go..."
}
```

### Example: Tags Management

List all tags:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/ListTags
```

Get tag details:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "tag_name": "v1.0.0"
}' localhost:23234 softserve.GitServerManagement/GetTag
```

### Example: Compare Branches

Compare two branches to see differences:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "base_branch": "main",
  "head_branch": "develop"
}' localhost:23234 softserve.GitServerManagement/CompareBranches
```

Compare specific commits:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "base_sha": "abc123",
  "head_sha": "def456"
}' localhost:23234 softserve.GitServerManagement/CompareCommits
```

### Example: Repository Information

Get default branch:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/GetDefaultBranch
```

Set default branch:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "branch_name": "main"
}' localhost:23234 softserve.GitServerManagement/SetDefaultBranch
```

Get clone URLs:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/GetCloneURLs
```

Get repository statistics:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo"
}' localhost:23234 softserve.GitServerManagement/GetRepositoryStats
```

### Example: Advanced Operations

Get file history (commits that modified a file):
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "path": "src/main.go",
  "limit": 20
}' localhost:23234 softserve.GitServerManagement/GetFileHistory
```

Search commits:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "query": "fix bug",
  "author": "john",
  "limit": 10
}' localhost:23234 softserve.GitServerManagement/SearchCommits
```

Check if path exists:
```bash
grpcurl -plaintext -d '{
  "repo_name": "my-repo",
  "path": "README.md",
  "ref": "main"
}' localhost:23234 softserve.GitServerManagement/CheckPath
```

## Access Levels

When managing collaborators, use these access levels:

- `NO_ACCESS` (1) - No access
- `READ_ONLY` (2) - Read-only access
- `READ_WRITE` (3) - Read and write access
- `ADMIN_ACCESS` (4) - Full admin access

## Webhook Events

Available webhook events:

- `push` - Push events
- `branch_tag_create` - Branch or tag creation
- `branch_tag_delete` - Branch or tag deletion
- `collaborator` - Collaborator changes
- `repository` - Repository create/delete/rename
- `repository_visibility_change` - Repository visibility changes

## Content Types

For webhooks:

- `json` - application/json
- `form` - application/x-www-form-urlencoded

## Client Implementation

### Go Client Example

```go
package main

import (
    "context"
    "log"

    pb "github.com/Gyt-project/soft-serve/pkg/grpc"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial("localhost:23234", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewGitServerManagementClient(conn)

    // Create a repository
    repo, err := client.CreateRepository(context.Background(), &pb.CreateRepositoryRequest{
        Name:        "test-repo",
        Description: "Test repository",
        Private:     false,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created repository: %s (ID: %d)", repo.Name, repo.Id)
}
```

### Python Client Example

```python
import grpc
from grpc_reflection.v1alpha import reflection_pb2
from grpc_reflection.v1alpha import reflection_pb2_grpc

# First, generate Python code from the proto file:
# python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. pkg/grpc/service.proto

import service_pb2
import service_pb2_grpc

def main():
    with grpc.insecure_channel('localhost:23234') as channel:
        client = service_pb2_grpc.GitServerManagementStub(channel)

        # Create a repository
        repo = client.CreateRepository(service_pb2.CreateRepositoryRequest(
            name='test-repo',
            description='Test repository',
            private=False
        ))

        print(f'Created repository: {repo.name} (ID: {repo.id})')

if __name__ == '__main__':
    main()
```

## Security Notes

**⚠️ IMPORTANT:** The gRPC API has NO AUTHENTICATION and assumes all requests have full admin privileges. This is designed for internal management use only.

**Security recommendations:**

1. **Never expose to the public internet** - Bind to localhost or use firewall rules
2. **Use in trusted networks only** - Behind VPN or private network
3. **Consider adding a reverse proxy** with authentication if needed (e.g., nginx with client certificates)
4. **Use network policies** in Kubernetes to restrict access
5. **Monitor access logs** for suspicious activity

## Generating Client Code

The proto file is located at `pkg/grpc/service.proto`. Generate client code for your language:

**Go:**
```bash
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    pkg/grpc/service.proto
```

**Python:**
```bash
python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. \
    pkg/grpc/service.proto
```

**Node.js:**
```bash
grpc_tools_node_protoc --js_out=import_style=commonjs,binary:. \
    --grpc_out=grpc_js:. \
    pkg/grpc/service.proto
```

**Java:**
```bash
protoc --java_out=. --grpc-java_out=. pkg/grpc/service.proto
```

## Troubleshooting

### gRPC server not starting

Check the config:
```bash
grep -A2 "grpc:" config.yaml
```

Check the logs for errors:
```bash
tail -f data/log/soft-serve.log | grep grpc
```

### Connection refused

Ensure the server is running and the port is correct:
```bash
netstat -an | grep 23234
```

### Permission denied errors

The gRPC API assumes admin privileges. There's no per-request authentication. Ensure your network security is properly configured.
