package gitbind

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Result struct {
	RepoURL      string       `json:"repoUrl"`
	Branch       string       `json:"branch,omitempty"`
	BaseCommit   string       `json:"baseCommit"`
	HeadCommit   string       `json:"headCommit"`
	Patch        Digest       `json:"patch"`
	ChangedFiles []FileDigest `json:"changedFiles"`
}

type Digest struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

type FileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Status string `json:"status"`
}

func Compute(ctx context.Context, repoDir string) (Result, error) {
	root, err := gitOutput(ctx, repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Result{}, err
	}
	root = strings.TrimSpace(root)

	remote, err := gitOutput(ctx, root, "config", "--get", "remote.origin.url")
	if err != nil {
		return Result{}, fmt.Errorf("read origin remote: %w", err)
	}
	repoURL, err := NormalizeRepoURL(strings.TrimSpace(remote))
	if err != nil {
		return Result{}, err
	}

	branch, _ := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" {
		branch = ""
	}

	head, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, err
	}
	head = strings.TrimSpace(head)

	base := head
	upstream, err := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && strings.TrimSpace(upstream) != "" {
		if mergeBase, mergeErr := gitOutput(ctx, root, "merge-base", "HEAD", strings.TrimSpace(upstream)); mergeErr == nil {
			base = strings.TrimSpace(mergeBase)
		}
	}

	patch, err := gitBytes(ctx, root, "diff", "--binary", "--no-color")
	if err != nil {
		return Result{}, err
	}
	patchSum := sha256.Sum256(patch)

	files, err := changedFiles(ctx, root)
	if err != nil {
		return Result{}, err
	}

	return Result{
		RepoURL:    repoURL,
		Branch:     branch,
		BaseCommit: base,
		HeadCommit: head,
		Patch: Digest{
			Algorithm: "sha256",
			Digest:    hex.EncodeToString(patchSum[:]),
		},
		ChangedFiles: files,
	}, nil
}

func NormalizeRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("origin remote URL is empty")
	}
	if match := scpLikeGitHub.FindStringSubmatch(raw); match != nil {
		return trimGitSuffix("https://" + match[1] + "/" + match[2]), nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse origin remote URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin remote URL %q is not HTTP(S) or GitHub SSH form", raw)
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, ".git")
	return parsed.String(), nil
}

var scpLikeGitHub = regexp.MustCompile(`^git@([^:]+):(.+)$`)

func trimGitSuffix(value string) string {
	return strings.TrimSuffix(value, ".git")
}

func changedFiles(ctx context.Context, root string) ([]FileDigest, error) {
	raw, err := gitBytes(ctx, root, "diff", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(raw, []byte{0})
	files := make([]FileDigest, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		rel := filepath.Clean(string(part))
		displayPath := filepath.ToSlash(rel)
		full := filepath.Join(root, rel)
		content, err := os.ReadFile(full)
		if errors.Is(err, os.ErrNotExist) {
			files = append(files, FileDigest{Path: displayPath, Status: "deleted"})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("hash changed file %s: %w", displayPath, err)
		}
		sum := sha256.Sum256(content)
		files = append(files, FileDigest{Path: displayPath, SHA256: hex.EncodeToString(sum[:]), Status: "present"})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitBytes(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return out, nil
}
