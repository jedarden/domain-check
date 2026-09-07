package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// CommitInfo represents the extracted commit information
type CommitInfo struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// GitHubCommitsData represents the structure saved to the state file
type GitHubCommitsData struct {
	CommonAncestor    string       `json:"common_ancestor"`
	GitHubBranch      string       `json:"github_branch"`
	ForgejoBranch     string       `json:"forgejo_branch"`
	ExtractedAt       string       `json:"extracted_at"`
	TotalCommits      int          `json:"total_commits"`
	GitHubCommits     []CommitInfo `json:"github_commits"`
}

func main() {
	// Default branch names
	githubBranch := "github/main"
	forgejoBranch := "origin/main"
	stateFile := ".github-commits-state.json"

	// Allow custom branch names via args
	if len(os.Args) > 1 {
		githubBranch = os.Args[1]
	}
	if len(os.Args) > 2 {
		forgejoBranch = os.Args[2]
	}
	if len(os.Args) > 3 {
		stateFile = os.Args[3]
	}

	fmt.Printf("Extracting GitHub-specific commits...\n")
	fmt.Printf("GitHub branch: %s\n", githubBranch)
	fmt.Printf("Forgejo branch: %s\n", forgejoBranch)
	fmt.Printf("State file: %s\n", stateFile)

	// Step 1: Fetch both remotes
	fmt.Println("\n1. Fetching remotes...")
	// Parse remote names from branch specs
	githubRemote := parseRemote(githubBranch)
	forgejoRemote := parseRemote(forgejoBranch)
	runGit("fetch", githubRemote)
	runGit("fetch", forgejoRemote)

	// Step 2: Find common ancestor
	fmt.Println("\n2. Finding common ancestor...")
	commonAncestor := findMergeBase(forgejoBranch, githubBranch)
	fmt.Printf("   Common ancestor: %s\n", commonAncestor)

	// Step 3: Extract GitHub-specific commits
	fmt.Println("\n3. Extracting GitHub-specific commits...")
	commits := getGitHubCommits(commonAncestor, githubBranch)
	fmt.Printf("   Found %d GitHub-specific commits\n", len(commits))

	// Step 4: Create data structure
	data := GitHubCommitsData{
		CommonAncestor: commonAncestor,
		GitHubBranch:   githubBranch,
		ForgejoBranch:  forgejoBranch,
		ExtractedAt:    time.Now().Format(time.RFC3339),
		TotalCommits:   len(commits),
		GitHubCommits:  commits,
	}

	// Step 5: Save to state file
	fmt.Println("\n4. Saving to state file...")
	if err := saveStateFile(stateFile, data); err != nil {
		fmt.Printf("   Error saving state file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Saved to: %s\n", stateFile)

	// Step 6: Print summary
	fmt.Println("\n✓ Extraction complete!")
	fmt.Printf("  Total GitHub-specific commits: %d\n", len(commits))
	if len(commits) > 0 {
		fmt.Println("\n  Sample commits:")
		for i, commit := range commits {
			if i >= 3 {
				fmt.Printf("  ... and %d more\n", len(commits)-3)
				break
			}
			fmt.Printf("  - %s: %s\n", commit.SHA[:8], commit.Message)
		}
	} else {
		fmt.Println("  No GitHub-specific commits found (branches are in sync)")
	}
}

func runGit(args ...string) string {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running git %v: %v\n", args, err)
		fmt.Printf("Output: %s\n", string(output))
		os.Exit(1)
	}
	return string(output)
}

func findMergeBase(branch1, branch2 string) string {
	cmd := exec.Command("git", "merge-base", branch1, branch2)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error finding merge base: %v\n", err)
		os.Exit(1)
	}
	return string(output[:len(output)-1]) // Remove trailing newline
}

func getGitHubCommits(commonAncestor, githubBranch string) []CommitInfo {
	cmd := exec.Command("git", "log",
		fmt.Sprintf("%s..%s", commonAncestor, githubBranch),
		"--format=%H|%an|%ai|%s",
	)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting commits: %v\n", err)
		return []CommitInfo{}
	}

	var commits []CommitInfo
	lines := splitLines(output)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := splitByFirst(line, "|")
		if len(parts) == 4 {
			commits = append(commits, CommitInfo{
				SHA:     parts[0],
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		}
	}

	return commits
}

func saveStateFile(filename string, data GitHubCommitsData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

func splitLines(data []byte) []string {
	str := string(data)
	lines := []string{}
	for _, line := range splitByChar(str, '\n') {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByFirst(s, sep string) []string {
	parts := []string{}
	for i, c := range s {
		if string(c) == sep {
			parts = append(parts, s[:i], s[i+1:])
			return parts
		}
	}
	parts = append(parts, s)
	return parts
}

func splitByChar(s string, c rune) []string {
	parts := []string{}
	current := ""
	for _, ch := range s {
		if ch == c {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	parts = append(parts, current)
	return parts
}

func parseRemote(branchSpec string) string {
	// Extract remote name from branch spec like "github/main" or "origin/main"
	parts := splitByFirst(branchSpec, "/")
	return parts[0]
}
