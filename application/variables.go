package application

import "strings"

func mergeVars(globalVars, stepVars map[string]string) map[string]string {
	merged := make(map[string]string, len(globalVars)+len(stepVars))

	for key, value := range globalVars {
		merged[key] = value
	}
	for key, value := range stepVars {
		merged[key] = value
	}

	return merged
}

func expandCommands(cmds []string, vars map[string]string) []string {
	expanded := make([]string, len(cmds))
	for i, cmd := range cmds {
		expanded[i] = expandCommand(cmd, vars)
	}

	return expanded
}

func expandCommand(command string, vars map[string]string) string {
	var builder strings.Builder
	builder.Grow(len(command))

	for i := 0; i < len(command); {
		if i+1 >= len(command) || command[i] != '@' || command[i+1] != '{' {
			builder.WriteByte(command[i])
			i++
			continue
		}

		end := strings.IndexByte(command[i+2:], '}')
		if end < 0 {
			builder.WriteString(command[i:])
			break
		}

		key := command[i+2 : i+2+end]
		if key != "" {
			if value, ok := vars[key]; ok {
				builder.WriteString(value)
				i += end + 3
				continue
			}
		}

		builder.WriteString(command[i : i+end+3])
		i += end + 3
	}

	return builder.String()
}
