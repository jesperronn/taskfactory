// Package skills embeds the agent skill so the binary can install it.
package skills

import _ "embed"

// SkillMD is the text of skills/taskfactory/SKILL.md, the canonical file.
//
//go:embed taskfactory/SKILL.md
var SkillMD string
