package analytics

import "strings"

func extractOpenCodeSearchText(r *opencodeRollout) string {
	var b strings.Builder
	for _, msg := range r.Messages {
		parts := msg.Parts
		for _, p := range parts {
			switch p.Type {
			case "text":
				if p.Text != "" {
					b.WriteString(p.Text)
					b.WriteByte('\n')
				}
			case "tool":
				if p.Tool != "" {
					b.WriteString(p.Tool)
					b.WriteByte('\n')
				}
				state := p.State
				if state != nil && state.Output != "" {
					const maxLen = 500
					output := state.Output
					if len(output) > maxLen {
						output = output[:maxLen]
					}
					b.WriteString(output)
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String()
}
