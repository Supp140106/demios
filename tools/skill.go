package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"
)

type ListSkillsArgs struct {
}

type ReadSkillArgs struct {
	Name string `json:"name" jsonschema:"title=Name,description=Name of the skill to load (e.g. taste, shadcn)"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var ListSkills = Tool{
	ID:          "ListSkills",
	Description: "List all available skill files in the Skills/ directory. Each skill has a name and description. Call this to discover which skills you can use.",
	Schema:      jsonschema.Reflect(&ListSkillsArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		skills, err := scanSkills()
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to scan skills: %w", err)
		}
		if len(skills) == 0 {
			return ExecuteResult{
				Title:  "No skills found",
				Output: "No skills found in Skills/ directory.",
				Metadata: map[string]any{
					"skills": []SkillInfo{},
					"count":  0,
				},
			}, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d skill(s):\n\n", len(skills)))
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("  %s\n    %s\n\n", s.Name, s.Description))
		}
		sb.WriteString("Call ReadSkill with the skill name to load its instructions.")
		return ExecuteResult{
			Title:  fmt.Sprintf("Skills: %d available", len(skills)),
			Output: sb.String(),
			Metadata: map[string]any{
				"skills": skills,
				"count":  len(skills),
			},
		}, nil
	},
}

var ReadSkill = Tool{
	ID:          "ReadSkill",
	Description: "Load a skill's instructions from the Skills/ directory. Call ListSkills first to see available skills, then ReadSkill with the skill name to get its full instructions.",
	Schema:      jsonschema.Reflect(&ReadSkillArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args ReadSkillArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Name == "" {
			return ExecuteResult{}, fmt.Errorf("name is required")
		}

		content, err := readSkillFile(args.Name)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("skill %q not found: %w", args.Name, err)
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Skill: %s", args.Name),
			Output: content,
			Metadata: map[string]any{
				"name": args.Name,
			},
		}, nil
	},
}

func skillDirs() []string {
	var dirs []string
	for _, c := range []string{".agents/skills", "Skills", "../Skills", "../.agents/skills"} {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			dirs = append(dirs, abs)
		}
	}
	exe, err := os.Executable()
	if err == nil {
		for _, rel := range []string{"Skills", ".agents/skills"} {
			p := filepath.Join(filepath.Dir(exe), rel)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				abs, _ := filepath.Abs(p)
				seen := false
				for _, d := range dirs {
					if d == abs {
						seen = true
						break
					}
				}
				if !seen {
					dirs = append(dirs, abs)
				}
			}
		}
	}
	return dirs
}

func scanSkills() ([]SkillInfo, error) {
	dirs := skillDirs()
	if len(dirs) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool)
	var skills []SkillInfo
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if seen[e.Name()] {
				continue
			}
			skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
			info, err := readSkillFrontmatter(skillPath)
			if err != nil {
				continue
			}
			if info.Name == "" {
				info.Name = e.Name()
			}
			skills = append(skills, info)
			seen[e.Name()] = true
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func readSkillFile(name string) (string, error) {
	dirs := skillDirs()
	if len(dirs) == 0 {
		return "", fmt.Errorf("skills directory not found")
	}
	for _, dir := range dirs {
		skillPath := filepath.Join(dir, name, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err == nil {
			content := string(data)
			content = stripFrontmatter(content)
			return content, nil
		}
	}
	return "", fmt.Errorf("skill %q not found in any skills directory", name)
}

func readSkillFrontmatter(path string) (SkillInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillInfo{}, err
	}
	return parseFrontmatter(string(data)), nil
}

func parseFrontmatter(content string) SkillInfo {
	info := SkillInfo{}
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return info
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return info
	}
	frontRaw := content[3 : 3+end]
	for _, line := range strings.Split(frontRaw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
		if strings.HasPrefix(line, "description:") {
			info.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return info
}

func stripFrontmatter(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return content
	}
	body := content[3+end+3:]
	return strings.TrimSpace(body)
}
