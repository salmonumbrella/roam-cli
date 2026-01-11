package cmd

// markdownEntry represents a parsed line with a hierarchy level.
type markdownEntry struct {
	level   int
	content string
}

// buildMarkdownTree turns a flat list of entries into a nested block tree.
func buildMarkdownTree(entries []markdownEntry) []*MarkdownBlock {
	var blocks []*MarkdownBlock
	var stack []*MarkdownBlock
	currentLevel := 0

	for _, entry := range entries {
		block := &MarkdownBlock{Content: entry.content, Level: entry.level}
		level := entry.level

		if level == 0 {
			blocks = append(blocks, block)
			stack = []*MarkdownBlock{block}
			currentLevel = 0
			continue
		}

		if level > currentLevel {
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, block)
				stack = append(stack, block)
				currentLevel = level
				continue
			}
			blocks = append(blocks, block)
			stack = []*MarkdownBlock{block}
			currentLevel = level
			continue
		}

		if level == currentLevel {
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, block)
				stack = append(stack, block)
			} else {
				blocks = append(blocks, block)
				stack = []*MarkdownBlock{block}
			}
			continue
		}

		for len(stack) > 0 && stack[len(stack)-1].Level >= level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, block)
			stack = append(stack, block)
		} else {
			blocks = append(blocks, block)
			stack = []*MarkdownBlock{block}
		}
		currentLevel = level
	}

	return blocks
}
