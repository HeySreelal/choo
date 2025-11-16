package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	appName   = "genie-fun"
	version   = "1.0.0"
	geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
)

type GeminiRequest struct {
	Contents []Content `json:"contents"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
	Error      *ErrorInfo  `json:"error,omitempty"`
}

type Candidate struct {
	Content ContentResponse `json:"content"`
}

type ContentResponse struct {
	Parts []PartResponse `json:"parts"`
}

type PartResponse struct {
	Text string `json:"text"`
}

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	// Check if we're in a git repository
	if !isGitRepo() {
		fmt.Fprintln(os.Stderr, "❌ Not a git repository")
		os.Exit(1)
	}

	// Get API key from environment
	apiKey := os.Getenv("GOOGLE_AI_TOKEN")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "❌ GOOGLE_AI_TOKEN environment variable not set")
		fmt.Fprintln(os.Stderr, "   Get your API key from: https://aistudio.google.com/apikey")
		os.Exit(1)
	}

	// Get git diff
	diff, err := getGitDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error getting git diff: %v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("✨ No changes detected. Nothing to commit!")
		return
	}

	fmt.Println("🎲 Generating creative commit message...")

	// Generate commit message
	commitMsg, err := generateCreativeCommit(apiKey, diff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error generating commit: %v\n", err)
		os.Exit(1)
	}

	// Display the generated commit message
	fmt.Println("\n" + commitMsg + "\n")

	// Copy to clipboard
	err = copyToClipboard(commitMsg)
	if err != nil {
		fmt.Printf("📋 Could not copy to clipboard: %v\n", err)
	} else {
		fmt.Println("📋 Copied to clipboard!")
	}
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

func getGitDiff() (string, error) {
	// Try staged changes first
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	stagedDiff := strings.TrimSpace(string(output))
	if stagedDiff != "" {
		return stagedDiff, nil
	}

	// If no staged changes, get unstaged changes
	cmd = exec.Command("git", "diff")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	unstagedDiff := strings.TrimSpace(string(output))
	if unstagedDiff != "" {
		return unstagedDiff, nil
	}

	// Check for untracked files
	cmd = exec.Command("git", "ls-files", "--others", "--exclude-standard")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	untrackedFiles := strings.TrimSpace(string(output))
	if untrackedFiles != "" {
		files := strings.Split(untrackedFiles, "\n")
		var summary strings.Builder
		summary.WriteString("New untracked files:\n")
		for _, file := range files {
			if file != "" {
				summary.WriteString(fmt.Sprintf("+ %s\n", file))
			}
		}
		return summary.String(), nil
	}

	return "", nil
}

func generateCreativeCommit(apiKey, diff string) (string, error) {
	prompt := `You are a creative, witty, and slightly chaotic developer who treats commit messages as an art form. You make commits that are fun, random, and creative - but ALWAYS contextually relevant to the actual code changes.

YOUR MISSION:
Analyze the git diff and create a TWO-LINE commit message:
- Line 1: Random emoji + creative/funny/philosophical/lyrical message related to the change
- Line 2: Actual technical explanation of what changed

CREATIVE STYLES (pick randomly based on the vibe):
🎵 SONG LYRICS: Find a song lyric that metaphorically relates to the change
  Example: "🎸 I fought the law and the law won / Fixed authentication middleware to properly validate JWT tokens"

🧠 PHILOSOPHICAL: Drop some wisdom that somehow connects
  Example: "🌊 The only constant is change, except constants which I just changed / Refactored configuration values to environment variables"

😂 JOKES/PUNS: Make a programming joke or pun about the change
  Example: "🤡 Why did the function break up? It had too many arguments! / Simplified parameter passing in user service"

🎭 RANDOM FACTS: Share a random fact that loosely relates
  Example: "🦖 T-Rex couldn't clap but this code now can / Added applause animation to success notifications"

🎪 CHAOS: Just pure creative chaos that somehow makes sense
  Example: "🌮 Tacos are just sandwiches that think different / Implemented dependency injection for better testing"

💭 SHOWER THOUGHTS: Those weird thoughts that actually fit
  Example: "🚿 If you clean a vacuum cleaner, you're a vacuum cleaner / Removed unused imports and dead code"

🎨 METAPHORS: Poetic descriptions of mundane changes
  Example: "🌸 Like a butterfly emerging from its cache-rysalis / Optimized Redis caching strategy"

RULES:
1. MUST be contextually relevant to the actual code changes (even if loosely)
2. First line: emoji + creative message (can be funny, deep, random, whatever)
3. Second line: Clear technical explanation of what actually changed
4. Use a single random emoji that fits the vibe (not limited to common ones)
5. Be creative, be weird, be fun - but make it make sense when you squint
6. Maximum 72 characters per line
7. Don't use quotes around the output

Git Changes:
` + diff + `

Generate the creative two-line commit message now:`

	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", geminiURL+"?key="+apiKey, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", err
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini API")
	}

	commitMsg := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	commitMsg = strings.Trim(commitMsg, "\"'`")

	return commitMsg, nil
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("no clipboard utility found")
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
