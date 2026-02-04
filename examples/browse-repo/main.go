// Example: Browse repository contents via gRPC
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	pb "github.com/charmbracelet/soft-serve/pkg/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr     = flag.String("addr", "localhost:23234", "gRPC server address")
	repoName = flag.String("repo", "", "Repository name")
	ref      = flag.String("ref", "", "Branch, tag, or commit (defaults to HEAD)")
	path     = flag.String("path", "", "Path within repository")
	command  = flag.String("cmd", "tree", "Command: tree or blob")
)

func main() {
	flag.Parse()

	if *repoName == "" {
		log.Fatal("Please specify a repository name with -repo")
	}

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGitServerManagementClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch *command {
	case "tree":
		listTree(ctx, client)
	case "blob":
		getBlob(ctx, client)
	default:
		log.Fatalf("Unknown command: %s (use 'tree' or 'blob')", *command)
	}
}

func listTree(ctx context.Context, client pb.GitServerManagementClient) {
	req := &pb.GetTreeRequest{
		RepoName: *repoName,
	}
	if *ref != "" {
		req.Ref = ref
	}
	if *path != "" {
		req.Path = path
	}

	resp, err := client.GetTree(ctx, req)
	if err != nil {
		log.Fatalf("GetTree failed: %v", err)
	}

	fmt.Printf("Repository: %s @ %s\n", *repoName, resp.Ref)
	if *path != "" {
		fmt.Printf("Path: %s\n", *path)
	}
	fmt.Println(strings.Repeat("-", 80))

	if len(resp.Entries) == 0 {
		fmt.Println("(empty directory)")
		return
	}

	// Find max name length for alignment
	maxLen := 0
	for _, entry := range resp.Entries {
		if len(entry.Name) > maxLen {
			maxLen = len(entry.Name)
		}
	}

	for _, entry := range resp.Entries {
		typeIndicator := " "
		if entry.IsDir {
			typeIndicator = "📁"
		} else if entry.IsSubmodule {
			typeIndicator = "📦"
		} else {
			typeIndicator = "📄"
		}

		size := formatSize(entry.Size)
		if entry.IsDir {
			size = "-"
		}

		fmt.Printf("%s %-*s  %8s  %s\n",
			typeIndicator,
			maxLen,
			entry.Name,
			size,
			entry.Mode,
		)
	}
}

func getBlob(ctx context.Context, client pb.GitServerManagementClient) {
	if *path == "" {
		log.Fatal("Please specify a file path with -path")
	}

	req := &pb.GetBlobRequest{
		RepoName: *repoName,
		Path:     *path,
	}
	if *ref != "" {
		req.Ref = ref
	}

	resp, err := client.GetBlob(ctx, req)
	if err != nil {
		log.Fatalf("GetBlob failed: %v", err)
	}

	fmt.Printf("Repository: %s\n", *repoName)
	if *ref != "" {
		fmt.Printf("Ref: %s\n", *ref)
	}
	fmt.Printf("Path: %s\n", resp.Path)
	fmt.Printf("Size: %d bytes\n", resp.Size)
	fmt.Printf("Binary: %t\n", resp.IsBinary)
	fmt.Println(strings.Repeat("-", 80))

	if resp.IsBinary {
		fmt.Println("Binary file content (base64 encoded):")
		fmt.Println(base64.StdEncoding.EncodeToString(resp.Content))
	} else {
		fmt.Println(string(resp.Content))
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
