package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bd-lite/internal/types"
)

// Store holds all issues in memory, backed by a JSONL file.
type Store struct {
	issues   map[string]*types.Issue
	path     string // path to issues.jsonl
	beadsDir string // path to .beads/
	prefix   string
}

// Load reads .beads/issues.jsonl and returns a Store.
func Load(beadsDir string) (*Store, error) {
	s := &Store{
		issues:   make(map[string]*types.Issue),
		beadsDir: beadsDir,
		path:     filepath.Join(beadsDir, "issues.jsonl"),
	}

	// Detect prefix
	prefix, err := s.detectPrefix()
	if err != nil {
		return nil, fmt.Errorf("detect prefix: %w", err)
	}
	s.prefix = prefix

	// Load issues from JSONL
	if _, err := os.Stat(s.path); err == nil {
		if err := s.loadFromFile(); err != nil {
			return nil, fmt.Errorf("load %s: %w", s.path, err)
		}
	}

	return s, nil
}

func (s *Store) loadFromFile() error {
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var issue types.Issue
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		s.issues[issue.ID] = &issue
	}
	return scanner.Err()
}

// Save writes all issues to issues.jsonl atomically.
func (s *Store) Save() error {
	// Sort issues by created_at for stable output
	issues := s.AllIssues()
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})

	tmpPath := s.path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	writer := bufio.NewWriter(file)
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)

	for _, issue := range issues {
		if err := enc.Encode(issue); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode issue %s: %w", issue.ID, err)
		}
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, s.path)
}

// SaveToFile writes issues to a specific file atomically (used for archive).
func (s *Store) SaveToFile(path string, issues []*types.Issue) error {
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	writer := bufio.NewWriter(file)
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)

	for _, issue := range issues {
		if err := enc.Encode(issue); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode issue %s: %w", issue.ID, err)
		}
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// Prefix returns the configured issue prefix.
func (s *Store) Prefix() string { return s.prefix }

// BeadsDir returns the .beads directory path.
func (s *Store) BeadsDir() string { return s.beadsDir }

// Get returns an issue by ID, or nil if not found.
func (s *Store) Get(id string) *types.Issue {
	return s.issues[id]
}

// Add inserts a new issue, generating its ID.
func (s *Store) Add(issue *types.Issue) {
	existingIDs := make(map[string]bool, len(s.issues))
	for id := range s.issues {
		existingIDs[id] = true
	}

	issue.ID = GenerateID(s.prefix, issue.Title, issue.Description, existingIDs)
	s.issues[issue.ID] = issue
}

// Put stores an issue directly (for updates).
func (s *Store) Put(issue *types.Issue) {
	s.issues[issue.ID] = issue
}

// Delete removes an issue by ID.
func (s *Store) Delete(id string) {
	delete(s.issues, id)
}

// AllIssues returns all issues as a slice.
func (s *Store) AllIssues() []*types.Issue {
	result := make([]*types.Issue, 0, len(s.issues))
	for _, issue := range s.issues {
		result = append(result, issue)
	}
	return result
}

// Count returns the number of issues.
func (s *Store) Count() int {
	return len(s.issues)
}

// IDs returns a map of all issue IDs (for collision checking).
func (s *Store) IDs() map[string]bool {
	ids := make(map[string]bool, len(s.issues))
	for id := range s.issues {
		ids[id] = true
	}
	return ids
}

// Filter returns issues matching the given criteria.
func (s *Store) Filter(opts FilterOpts) []*types.Issue {
	var result []*types.Issue
	for _, issue := range s.issues {
		if opts.matches(issue) {
			result = append(result, issue)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// FilterOpts controls which issues are returned by Filter.
type FilterOpts struct {
	Status    *types.Status
	Priority  *int
	IssueType *types.IssueType
	Assignee  *string
	Labels    []string
	Content   string // search title + description
}

func (f *FilterOpts) matches(issue *types.Issue) bool {
	if f.Status != nil && issue.Status != *f.Status {
		return false
	}
	if f.Priority != nil && issue.Priority != *f.Priority {
		return false
	}
	if f.IssueType != nil && issue.IssueType != *f.IssueType {
		return false
	}
	if f.Assignee != nil && issue.Assignee != *f.Assignee {
		return false
	}
	if f.Content != "" {
		needle := strings.ToLower(f.Content)
		if !strings.Contains(strings.ToLower(issue.Title), needle) &&
			!strings.Contains(strings.ToLower(issue.Description), needle) {
			return false
		}
	}
	if len(f.Labels) > 0 {
		labelSet := make(map[string]bool, len(issue.Labels))
		for _, l := range issue.Labels {
			labelSet[l] = true
		}
		for _, required := range f.Labels {
			if !labelSet[required] {
				return false
			}
		}
	}
	return true
}

// Ready returns issues that are open/in_progress with no unresolved blocking dependencies.
func (s *Store) Ready() []*types.Issue {
	// Build set of non-closed issue IDs
	openIDs := make(map[string]bool)
	for _, issue := range s.issues {
		if issue.Status != types.StatusClosed {
			openIDs[issue.ID] = true
		}
	}

	// Find issues blocked by open dependencies
	blocked := make(map[string]bool)
	for _, issue := range s.issues {
		for _, dep := range issue.Dependencies {
			if dep.Type == types.DepBlocks && openIDs[dep.DependsOnID] {
				// This issue is blocked by dep.DependsOnID which is still open
				blocked[dep.IssueID] = true
			}
		}
	}

	var result []*types.Issue
	for _, issue := range s.issues {
		if (issue.Status == types.StatusOpen || issue.Status == types.StatusInProgress) && !blocked[issue.ID] {
			result = append(result, issue)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// DepTree builds a dependency tree starting from the given issue.
type TreeNode struct {
	Issue    *types.Issue
	Children []*TreeNode
}

func (s *Store) DepTree(rootID string) (*TreeNode, error) {
	issue := s.Get(rootID)
	if issue == nil {
		return nil, fmt.Errorf("issue %s not found", rootID)
	}
	visited := make(map[string]bool)
	return s.buildTree(issue, visited), nil
}

func (s *Store) buildTree(issue *types.Issue, visited map[string]bool) *TreeNode {
	if visited[issue.ID] {
		return &TreeNode{Issue: issue}
	}
	visited[issue.ID] = true

	node := &TreeNode{Issue: issue}

	// Find all issues that depend on this issue (this issue blocks them)
	for _, other := range s.issues {
		for _, dep := range other.Dependencies {
			if dep.DependsOnID == issue.ID && dep.Type == types.DepBlocks {
				child := s.buildTree(other, visited)
				node.Children = append(node.Children, child)
			}
		}
	}

	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Issue.ID < node.Children[j].Issue.ID
	})
	return node
}

// AddDependency adds a blocking dependency: issueID depends on dependsOnID.
func (s *Store) AddDependency(issueID, dependsOnID string) error {
	issue := s.Get(issueID)
	if issue == nil {
		return fmt.Errorf("issue %s not found", issueID)
	}
	if s.Get(dependsOnID) == nil {
		return fmt.Errorf("issue %s not found", dependsOnID)
	}

	// Check for duplicate
	for _, dep := range issue.Dependencies {
		if dep.DependsOnID == dependsOnID && dep.Type == types.DepBlocks {
			return fmt.Errorf("dependency already exists")
		}
	}

	issue.Dependencies = append(issue.Dependencies, &types.Dependency{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
		Type:        types.DepBlocks,
		CreatedAt:   time.Now(),
	})
	issue.UpdatedAt = time.Now()
	return nil
}

// RemoveDependency removes a blocking dependency.
func (s *Store) RemoveDependency(issueID, dependsOnID string) error {
	issue := s.Get(issueID)
	if issue == nil {
		return fmt.Errorf("issue %s not found", issueID)
	}

	found := false
	deps := make([]*types.Dependency, 0, len(issue.Dependencies))
	for _, dep := range issue.Dependencies {
		if dep.DependsOnID == dependsOnID && dep.Type == types.DepBlocks {
			found = true
			continue
		}
		deps = append(deps, dep)
	}

	if !found {
		return fmt.Errorf("no blocking dependency from %s to %s", issueID, dependsOnID)
	}

	issue.Dependencies = deps
	issue.UpdatedAt = time.Now()
	return nil
}

// AddComment adds a comment to an issue.
func (s *Store) AddComment(issueID, text, author string) error {
	issue := s.Get(issueID)
	if issue == nil {
		return fmt.Errorf("issue %s not found", issueID)
	}

	// Auto-increment comment ID
	var maxID int64
	for _, c := range issue.Comments {
		if c.ID > maxID {
			maxID = c.ID
		}
	}

	issue.Comments = append(issue.Comments, &types.Comment{
		ID:        maxID + 1,
		IssueID:   issueID,
		Author:    author,
		Text:      text,
		CreatedAt: time.Now(),
	})
	issue.UpdatedAt = time.Now()
	return nil
}

// LoadArchive reads the archive.jsonl file.
func (s *Store) LoadArchive() ([]*types.Issue, error) {
	archivePath := filepath.Join(s.beadsDir, "archive.jsonl")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var issues []*types.Issue
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var issue types.Issue
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return nil, err
		}
		issues = append(issues, &issue)
	}
	return issues, scanner.Err()
}

// Prefix detection

func (s *Store) detectPrefix() (string, error) {
	// 1. Check config.yaml
	configPath := filepath.Join(s.beadsDir, "config.yaml")
	if prefix := readPrefixFromConfig(configPath); prefix != "" {
		return prefix, nil
	}

	// 2. Check existing issues
	if err := s.loadFromFile(); err == nil && len(s.issues) > 0 {
		var firstPrefix string
		allSame := true
		for _, issue := range s.issues {
			p := extractIssuePrefix(issue.ID)
			if firstPrefix == "" {
				firstPrefix = p
			} else if p != firstPrefix {
				allSame = false
				break
			}
		}
		// Clear loaded issues - they'll be reloaded properly
		s.issues = make(map[string]*types.Issue)

		if allSame && firstPrefix != "" {
			return firstPrefix, nil
		}
		if !allSame {
			return "", fmt.Errorf("issues have mixed prefixes, set issue-prefix in .beads/config.yaml")
		}
	} else {
		// Reset issues map if load failed
		s.issues = make(map[string]*types.Issue)
	}

	// 3. Fallback to directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "bd", nil
	}
	prefix := filepath.Base(cwd)
	prefix = SanitizePrefix(prefix)
	if prefix == "" {
		prefix = "bd"
	}
	return prefix, nil
}

func readPrefixFromConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Simple YAML parsing for issue-prefix: value
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "issue-prefix:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "issue-prefix:"))
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

func extractIssuePrefix(issueID string) string {
	lastIdx := strings.LastIndex(issueID, "-")
	if lastIdx <= 0 {
		return ""
	}

	suffix := issueID[lastIdx+1:]
	if len(suffix) > 0 {
		numPart := suffix
		if dotIdx := strings.Index(suffix, "."); dotIdx > 0 {
			numPart = suffix[:dotIdx]
		}
		var num int
		if _, err := fmt.Sscanf(numPart, "%d", &num); err == nil {
			return issueID[:lastIdx]
		}
	}

	firstIdx := strings.Index(issueID, "-")
	if firstIdx <= 0 {
		return ""
	}
	return issueID[:firstIdx]
}

// SanitizePrefix cleans a string for use as an issue ID prefix.
func SanitizePrefix(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return -1
	}, s)
}
