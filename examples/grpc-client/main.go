package main

import (
	"context"
	"fmt"
	"log"
	"time"

	grpcclient "github.com/Gyt-project/soft-serve/pkg/grpc"
)

func main() {
	// Create a new client
	client, err := grpcclient.NewClient("localhost:23234")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Example 1: Health Check
	fmt.Println("=== Health Check ===")
	health, err := client.HealthCheck(ctx)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Printf("Status: %s, Version: %s\n\n", health.Status, health.Version)

	// Example 2: List Repositories
	fmt.Println("=== List Repositories ===")
	repos, err := client.ListRepositories(ctx, &grpcclient.ListRepositoriesRequest{})
	if err != nil {
		log.Fatalf("Failed to list repositories: %v", err)
	}
	fmt.Printf("Found %d repositories:\n", len(repos.Repositories))
	for _, repo := range repos.Repositories {
		fmt.Printf("  - %s: %s\n", repo.Name, repo.Description)
	}
	fmt.Println()

	// Example 3: Get a specific repository
	if len(repos.Repositories) > 0 {
		repoName := repos.Repositories[0].Name
		fmt.Printf("=== Get Repository: %s ===\n", repoName)
		repo, err := client.GetRepository(ctx, &grpcclient.GetRepositoryRequest{
			Name: repoName,
		})
		if err != nil {
			log.Printf("Failed to get repository: %v", err)
		} else {
			fmt.Printf("Name: %s\n", repo.Name)
			fmt.Printf("Description: %s\n", repo.Description)
			fmt.Printf("Private: %v\n", repo.IsPrivate)
			fmt.Printf("Created: %s\n\n", repo.CreatedAt.AsTime().Format(time.RFC3339))
		}

		// Example 4: Get Branches
		fmt.Printf("=== Get Branches for %s ===\n", repoName)
		branches, err := client.GetBranches(ctx, &grpcclient.GetBranchesRequest{
			RepoName: repoName,
		})
		if err != nil {
			log.Printf("Failed to get branches: %v", err)
		} else {
			fmt.Printf("Found %d branches:\n", len(branches.Branches))
			for _, branch := range branches.Branches {
				fmt.Printf("  - %s (%s)\n", branch.Name, branch.CommitSha[:8])
			}
			fmt.Println()
		}

		// Example 5: List Recent Commits
		fmt.Printf("=== Recent Commits for %s ===\n", repoName)
		commits, err := client.ListCommits(ctx, &grpcclient.ListCommitsRequest{
			RepoName: repoName,
			Limit:    func() *int32 { l := int32(5); return &l }(),
		})
		if err != nil {
			log.Printf("Failed to list commits: %v", err)
		} else {
			fmt.Printf("Showing %d recent commits:\n", len(commits.Commits))
			for _, commit := range commits.Commits {
				fmt.Printf("  %s - %s (%s)\n",
					commit.Sha[:8],
					commit.Message[:min(50, len(commit.Message))],
					commit.Author.Name)
			}
			fmt.Println()
		}

		// Example 6: Browse Repository Tree
		fmt.Printf("=== Browse Root Directory of %s ===\n", repoName)
		tree, err := client.GetTree(ctx, &grpcclient.GetTreeRequest{
			RepoName: repoName,
		})
		if err != nil {
			log.Printf("Failed to get tree: %v", err)
		} else {
			fmt.Printf("Found %d entries:\n", len(tree.Entries))
			for _, entry := range tree.Entries {
				if entry.IsDir {
					fmt.Printf("  📁 %s/\n", entry.Name)
				} else {
					fmt.Printf("  📄 %s (%d bytes)\n", entry.Name, entry.Size)
				}
			}
			fmt.Println()
		}

		// Example 7: Get Clone URLs
		fmt.Printf("=== Clone URLs for %s ===\n", repoName)
		urls, err := client.GetCloneURLs(ctx, &grpcclient.GetCloneURLsRequest{
			RepoName: repoName,
		})
		if err != nil {
			log.Printf("Failed to get clone URLs: %v", err)
		} else {
			fmt.Printf("SSH:  %s\n", urls.SshUrl)
			fmt.Printf("HTTP: %s\n", urls.HttpUrl)
			fmt.Printf("Git:  %s\n", urls.GitUrl)
			fmt.Println()
		}

		// Example 8: Get Repository Stats
		fmt.Printf("=== Repository Statistics for %s ===\n", repoName)
		stats, err := client.GetRepositoryStats(ctx, &grpcclient.GetRepositoryStatsRequest{
			RepoName: repoName,
		})
		if err != nil {
			log.Printf("Failed to get stats: %v", err)
		} else {
			fmt.Printf("Commits: %d\n", stats.CommitCount)
			fmt.Printf("Branches: %d\n", stats.BranchCount)
			fmt.Printf("Tags: %d\n", stats.TagCount)
			if stats.LastCommit != nil {
				fmt.Printf("Last Commit: %s\n", stats.LastCommit.AsTime().Format(time.RFC3339))
			}
			fmt.Println()
		}
	}

	// Example 9: List Users
	fmt.Println("=== List Users ===")
	users, err := client.ListUsers(ctx, &grpcclient.ListUsersRequest{})
	if err != nil {
		log.Printf("Failed to list users: %v", err)
	} else {
		fmt.Printf("Found %d users:\n", len(users.Users))
		for _, user := range users.Users {
			adminStr := ""
			if user.IsAdmin {
				adminStr = " (admin)"
			}
			fmt.Printf("  - %s%s\n", user.Username, adminStr)
		}
		fmt.Println()
	}

	fmt.Println("Done!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
