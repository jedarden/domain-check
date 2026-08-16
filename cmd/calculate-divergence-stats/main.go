package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

// AuthorStats holds commit statistics per author
type AuthorStats struct {
	Author  string `json:"author"`
	Commits int    `json:"commits"`
}

// BranchStats holds statistics for a single branch
type BranchStats struct {
	Branch         string       `json:"branch"`
	TotalCommits   int          `json:"total_commits"`
	UniqueCommits  int          `json:"unique_commits"`
	FirstCommit    string       `json:"first_commit,omitempty"`
	LastCommit     string       `json:"last_commit,omitempty"`
	LastCommitDate string       `json:"last_commit_date,omitempty"`
	TopAuthors     []AuthorStats `json:"top_authors,omitempty"`
}

// DivergenceStats holds all divergence statistics
type DivergenceStats struct {
	AnalysisTimestamp      string      `json:"analysis_timestamp"`
	Branch1                string      `json:"branch1"`
	Branch2                string      `json:"branch2"`
	CommonAncestor         string      `json:"common_ancestor"`
	CommonAncestorDate     string      `json:"common_ancestor_date,omitempty"`
	TimeSinceDivergence    string      `json:"time_since_divergence,omitempty"`
	DaysSinceDivergence    int         `json:"days_since_divergence,omitempty"`
	Branch1Stats           BranchStats `json:"branch1_stats"`
	Branch2Stats           BranchStats `json:"branch2_stats"`
	TotalUniqueCommits     int         `json:"total_unique_commits"`
	DivergenceType         string      `json:"divergence_type"`
}

func main() {
	// Default branch names
	branch1 := "github/main"
	branch2 := "origin/main"
	outputFile := ".divergence-stats.json"

	// Allow custom branch names via args
	if len(os.Args) > 1 {
		branch1 = os.Args[1]
	}
	if len(os.Args) > 2 {
		branch2 = os.Args[2]
	}
	if len(os.Args) > 3 {
		outputFile = os.Args[3]
	}

	fmt.Printf("Calculating divergence statistics...\n")
	fmt.Printf("Branch 1: %s\n", branch1)
	fmt.Printf("Branch 2: %s\n", branch2)
	fmt.Printf("Output file: %s\n", outputFile)

	// Calculate statistics
	stats, err := calculateDivergenceStats(branch1, branch2)
	if err != nil {
		fmt.Printf("Error calculating statistics: %v\n", err)
		os.Exit(1)
	}

	// Save to output file
	if err := saveStats(outputFile, stats); err != nil {
		fmt.Printf("Error saving stats: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	printSummary(stats)
}

func calculateDivergenceStats(branch1, branch2 string) (*DivergenceStats, error) {
	stats := &DivergenceStats{
		AnalysisTimestamp: time.Now().Format(time.RFC3339),
		Branch1:          branch1,
		Branch2:          branch2,
	}

	// Find common ancestor
	fmt.Println("\n1. Finding common ancestor...")
	commonAncestor, err := findMergeBase(branch1, branch2)
	if err != nil {
		return nil, fmt.Errorf("finding merge base: %w", err)
	}
	stats.CommonAncestor = commonAncestor
	fmt.Printf("   Common ancestor: %s\n", commonAncestor[:8])

	// Get common ancestor date
	fmt.Println("\n2. Getting common ancestor date...")
	commonDate, err := getCommitDate(commonAncestor)
	if err == nil {
		stats.CommonAncestorDate = commonDate
		if parsedTime, err := time.Parse(time.RFC3339, commonDate); err == nil {
			duration := time.Since(parsedTime)
			stats.TimeSinceDivergence = duration.String()
			stats.DaysSinceDivergence = int(duration.Hours() / 24)
			fmt.Printf("   Common ancestor date: %s\n", commonDate)
			fmt.Printf("   Time since divergence: %s (%d days)\n", duration.String(), stats.DaysSinceDivergence)
		}
	}

	// Calculate branch 1 statistics
	fmt.Println("\n3. Calculating branch 1 statistics...")
	b1Stats, err := calculateBranchStats(branch1, commonAncestor, branch2)
	if err != nil {
		return nil, fmt.Errorf("calculating branch 1 stats: %w", err)
	}
	stats.Branch1Stats = *b1Stats
	printBranchStats("Branch 1", b1Stats)

	// Calculate branch 2 statistics
	fmt.Println("\n4. Calculating branch 2 statistics...")
	b2Stats, err := calculateBranchStats(branch2, commonAncestor, branch1)
	if err != nil {
		return nil, fmt.Errorf("calculating branch 2 stats: %w", err)
	}
	stats.Branch2Stats = *b2Stats
	printBranchStats("Branch 2", b2Stats)

	// Calculate total unique commits
	stats.TotalUniqueCommits = stats.Branch1Stats.UniqueCommits + stats.Branch2Stats.UniqueCommits

	// Determine divergence type
	stats.DivergenceType = determineDivergenceType(stats)

	return stats, nil
}

func calculateBranchStats(branch, commonAncestor, otherBranch string) (*BranchStats, error) {
	stats := &BranchStats{
		Branch: branch,
	}

	// Get total commits on branch
	totalCommits, err := getCommitCount(branch)
	if err != nil {
		return nil, fmt.Errorf("getting commit count: %w", err)
	}
	stats.TotalCommits = totalCommits

	// Get unique commits (commits on this branch but not on other branch)
	uniqueCommits, err := getUniqueCommitCount(commonAncestor, branch)
	if err != nil {
		return nil, fmt.Errorf("getting unique commit count: %w", err)
	}
	stats.UniqueCommits = uniqueCommits

	// Get last commit info
	lastCommit, lastDate, err := getLastCommit(branch)
	if err == nil {
		stats.LastCommit = lastCommit
		stats.LastCommitDate = lastDate
	}

	// Get author distribution for unique commits
	authors, err := getAuthorDistribution(commonAncestor, branch)
	if err == nil && len(authors) > 0 {
		// Sort by commit count descending
		sort.Slice(authors, func(i, j int) bool {
			return authors[i].Commits > authors[j].Commits
		})
		// Take top 10
		if len(authors) > 10 {
			authors = authors[:10]
		}
		stats.TopAuthors = authors
	}

	return stats, nil
}

func findMergeBase(branch1, branch2 string) (string, error) {
	cmd := exec.Command("git", "merge-base", branch1, branch2)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base: %w", err)
	}
	return string(output[:len(output)-1]), nil
}

func getCommitDate(commit string) (string, error) {
	cmd := exec.Command("git", "show", "-s", "--format=%ci", commit)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	dateStr := string(output[:len(output)-1])
	// Parse and reformat as RFC3339
	if parsed, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr); err == nil {
		return parsed.Format(time.RFC3339), nil
	}
	return dateStr, nil
}

func getCommitCount(branch string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", branch)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git rev-list: %w", err)
	}
	var count int
	fmt.Sscanf(string(output[:len(output)-1]), "%d", &count)
	return count, nil
}

func getUniqueCommitCount(commonAncestor, branch string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", commonAncestor, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git rev-list for unique commits: %w", err)
	}
	var count int
	fmt.Sscanf(string(output[:len(output)-1]), "%d", &count)
	return count, nil
}

func getLastCommit(branch string) (string, string, error) {
	// Get commit SHA
	cmd := exec.Command("git", "rev-parse", branch)
	shaOutput, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	sha := string(shaOutput[:len(shaOutput)-1])

	// Get commit date
	cmd = exec.Command("git", "show", "-s", "--format=%ci", branch)
	dateOutput, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	dateStr := string(dateOutput[:len(dateOutput)-1])

	// Format date as RFC3339
	if parsed, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr); err == nil {
		return sha, parsed.Format(time.RFC3339), nil
	}
	return sha, dateStr, nil
}

func getAuthorDistribution(commonAncestor, branch string) ([]AuthorStats, error) {
	cmd := exec.Command("git", "log",
		fmt.Sprintf("%s..%s", commonAncestor, branch),
		"--format=%an",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Count commits per author
	authorCounts := make(map[string]int)
	lines := splitLines(string(output))
	for _, line := range lines {
		if line != "" {
			authorCounts[line]++
		}
	}

	// Convert to slice
	var authors []AuthorStats
	for author, count := range authorCounts {
		authors = append(authors, AuthorStats{
			Author:  author,
			Commits: count,
		})
	}

	return authors, nil
}

func determineDivergenceType(stats *DivergenceStats) string {
	if stats.TotalUniqueCommits == 0 {
		return "synchronized"
	}
	if stats.Branch1Stats.UniqueCommits > 0 && stats.Branch2Stats.UniqueCommits > 0 {
		return "diverged"
	}
	if stats.Branch1Stats.UniqueCommits > 0 {
		return "branch1_ahead"
	}
	return "branch2_ahead"
}

func saveStats(filename string, stats *DivergenceStats) error {
	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

func printSummary(stats *DivergenceStats) {
	fmt.Println("\n✓ Divergence statistics calculated!")
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Branch 1 (%s):\n", stats.Branch1)
	fmt.Printf("    Total commits: %d\n", stats.Branch1Stats.TotalCommits)
	fmt.Printf("    Unique commits: %d\n", stats.Branch1Stats.UniqueCommits)
	fmt.Printf("    Last commit: %s\n", stats.Branch1Stats.LastCommit[:8])

	fmt.Printf("  Branch 2 (%s):\n", stats.Branch2)
	fmt.Printf("    Total commits: %d\n", stats.Branch2Stats.TotalCommits)
	fmt.Printf("    Unique commits: %d\n", stats.Branch2Stats.UniqueCommits)
	fmt.Printf("    Last commit: %s\n", stats.Branch2Stats.LastCommit[:8])

	fmt.Printf("  Divergence:\n")
	fmt.Printf("    Total unique commits: %d\n", stats.TotalUniqueCommits)
	fmt.Printf("    Type: %s\n", stats.DivergenceType)
	if stats.CommonAncestorDate != "" {
		fmt.Printf("    Common ancestor: %s (%s)\n", stats.CommonAncestor[:8], stats.CommonAncestorDate)
	}
	if stats.DaysSinceDivergence > 0 {
		fmt.Printf("    Days since divergence: %d\n", stats.DaysSinceDivergence)
	}

	fmt.Printf("\n  Output saved to: .divergence-stats.json\n")
}

func printBranchStats(label string, stats *BranchStats) {
	fmt.Printf("   %s:\n", label)
	fmt.Printf("     Total commits: %d\n", stats.TotalCommits)
	fmt.Printf("     Unique commits: %d\n", stats.UniqueCommits)
	if len(stats.TopAuthors) > 0 {
		fmt.Printf("     Top contributors:\n")
		for i, author := range stats.TopAuthors {
			if i >= 5 {
				break
			}
			fmt.Printf("       - %s: %d commits\n", author.Author, author.Commits)
		}
	}
}

func splitLines(s string) []string {
	lines := []string{}
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
