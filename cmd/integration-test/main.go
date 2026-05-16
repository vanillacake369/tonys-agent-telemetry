package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
)

func main() {
	failed := 0

	// Test 1: Session discovery
	fmt.Print("Session discovery... ")
	sessions, err := data.DiscoverSessions()
	if err != nil || len(sessions) == 0 {
		fmt.Printf("FAIL (sessions=%d, err=%v)\n", len(sessions), err)
		failed++
	} else {
		fmt.Printf("OK (%d sessions)\n", len(sessions))
	}

	// Test 2: DAG parsing for session with subagents
	fmt.Print("DAG parsing... ")
	dagFound := false
	for _, s := range sessions {
		dag, err := data.ParseDAG(s.FilePath)
		if err == nil && dag != nil && len(dag.Children) > 0 {
			fmt.Printf("OK (session %s, %d children)\n", s.ID[:8], len(dag.Children))
			dagFound = true
			break
		}
	}
	if !dagFound {
		fmt.Println("WARN (no sessions with subagents found)")
	}

	// Test 3: Agent discovery
	fmt.Print("Agent discovery... ")
	agents, err := data.DiscoverAgents()
	if err != nil || len(agents) == 0 {
		fmt.Printf("FAIL (agents=%d, err=%v)\n", len(agents), err)
		failed++
	} else {
		fmt.Printf("OK (%d agents)\n", len(agents))
		for _, a := range agents {
			if a.Description == "" || a.Description == "---" {
				fmt.Printf("  WARN: agent %s has bad description: %q\n", a.Name, a.Description)
			}
		}
	}

	// Test 4: Local skill scan
	fmt.Print("Local skill scan... ")
	skills, err := skill.ScanLocal()
	if err != nil || len(skills) == 0 {
		fmt.Printf("FAIL (skills=%d, err=%v)\n", len(skills), err)
		failed++
	} else {
		fmt.Printf("OK (%d skills)\n", len(skills))
		for _, s := range skills {
			if s.Description == "" || s.Description == "---" {
				fmt.Printf("  WARN: skill %s has bad description: %q\n", s.Name, s.Description)
				failed++
			}
		}
	}

	// Test 5: GitHub skill search (if gh available)
	fmt.Print("GitHub skill search... ")
	ghSkills, err := skill.SearchGitHub(context.Background(), "security", "stars", 5)
	if err != nil {
		fmt.Printf("SKIP (gh not available or error: %v)\n", err)
	} else {
		fmt.Printf("OK (%d results)\n", len(ghSkills))
	}

	// Test 6: Platform detection
	fmt.Print("Platform detection... ")
	info := platform.Detect()
	fmt.Printf("OK (os=%s, emulator=%s, mux=%s)\n", info.OS, info.Emulator, info.Multiplexer)

	// Test 7: Conversation preview
	fmt.Print("Conversation preview... ")
	if len(sessions) > 0 {
		turns, err := data.ParseConversationPreview(sessions[0].FilePath, 3)
		if err != nil || len(turns) == 0 {
			fmt.Printf("FAIL (turns=%d, err=%v)\n", len(turns), err)
			failed++
		} else {
			fmt.Printf("OK (%d turns)\n", len(turns))
		}
	}

	fmt.Printf("\n=== Results: %d failures ===\n", failed)
	if failed > 0 {
		os.Exit(1)
	}
}
