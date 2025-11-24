# Transcript Viewer Implementation Plan

## Overview
Build a comprehensive transcript viewer for confab that provides interactive, hierarchical viewing of Claude Code sessions with full message-level detail, agent nesting, and rich formatting.

## Goals
1. Display transcripts as conversational UI (not raw files)
2. Support recursive agent viewing (agents can spawn sub-agents)
3. Handle all Claude Code message types (text, tool use, tool results, thinking)
4. Provide excellent UX (search, copy, collapse, syntax highlighting)
5. Match or exceed Claude Code's own UI quality

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│ Session View                                             │
│  ├─ Run 1                                               │
│  │   ├─ Transcript (main conversation)                  │
│  │   │   ├─ Message 1 (user)                           │
│  │   │   ├─ Message 2 (assistant + tool use)           │
│  │   │   │   └─ Agent Transcript (nested)              │
│  │   │   │       ├─ Agent Message 1                    │
│  │   │   │       └─ Agent Message 2                    │
│  │   │   └─ Message 3 (assistant response)             │
│  │   └─ Todos                                           │
│  └─ Run 2                                               │
└─────────────────────────────────────────────────────────┘
```

---

## Phase 0: Schema Discovery & Documentation
**Duration:** 2-3 hours
**Status:** REQUIRED FIRST

### Objectives
- Reverse-engineer Claude Code transcript format
- Document all message types and structures
- Create TypeScript type definitions
- Identify edge cases and variations

### Tasks

#### 0.1: Collect Sample Transcripts
- [ ] Generate diverse Claude Code sessions with:
  - User/assistant text messages
  - Tool use (Bash, Read, Write, Edit, Grep, etc.)
  - Tool results (success, errors, large outputs)
  - Thinking blocks
  - Agent spawning (Task tool)
  - Multi-level agent nesting
  - Image handling (if applicable)
  - Error scenarios

#### 0.2: Analyze Transcript Structure
- [ ] Examine JSONL format (one JSON object per line)
- [ ] Document top-level message fields:
  - `role` (user, assistant, system?)
  - `type` (text, tool_use, tool_result, thinking?)
  - `content` (string or array?)
  - `timestamp` or `id`
  - Any metadata fields
- [ ] Document content block variations:
  - Text blocks
  - Tool use blocks (name, parameters)
  - Tool result blocks (tool_use_id, content, error?)
  - Thinking blocks
  - Image blocks (if any)

#### 0.3: Analyze Agent Relationships
- [ ] How are agent IDs referenced in main transcript?
- [ ] How to map `agent-{id}.jsonl` files to parent messages?
- [ ] Document the Task tool structure:
  - Parameters (description, prompt, subagent_type)
  - Tool result format (contains agent output?)
  - Agent ID in tool_use_id or elsewhere?

#### 0.4: Create Type Definitions
Create `frontend/src/lib/types/transcript.ts`:
```typescript
// Core message structure
export type TranscriptMessage = {
  role: 'user' | 'assistant' | 'system';
  content: ContentBlock[];
  // ... other fields discovered
};

export type ContentBlock =
  | TextBlock
  | ToolUseBlock
  | ToolResultBlock
  | ThinkingBlock;

export type ToolUseBlock = {
  type: 'tool_use';
  id: string;
  name: string;
  input: Record<string, any>;
};

export type ToolResultBlock = {
  type: 'tool_result';
  tool_use_id: string;
  content: string;
  is_error?: boolean;
};

// ... etc
```

#### 0.5: Document Findings
Create `docs/claude-code-transcript-format.md` with:
- Schema documentation
- Example messages for each type
- Known variations and edge cases
- Agent nesting model
- Potential breaking changes to watch for

**Deliverable:** Complete schema documentation + TypeScript types

---

## Phase 1: Core Infrastructure
**Duration:** 6-8 hours
**Dependencies:** Phase 0

### Objectives
- Set up transcript fetching and parsing
- Build basic hierarchical data model
- Create skeleton UI components

### Tasks

#### 1.1: Transcript Fetching Service
Create `frontend/src/lib/services/transcriptService.ts`:
- [ ] Fetch transcript content from backend API
- [ ] Parse JSONL (split by newline, JSON.parse each line)
- [ ] Handle parse errors gracefully
- [ ] Cache fetched transcripts (avoid re-fetching)

```typescript
export async function fetchTranscript(
  runId: number,
  fileId: number
): Promise<TranscriptMessage[]> {
  const response = await fetch(
    `/api/v1/runs/${runId}/files/${fileId}/content`,
    { credentials: 'include' }
  );

  const text = await response.text();
  const lines = text.split('\n').filter(l => l.trim());
  return lines.map(line => JSON.parse(line));
}
```

#### 1.2: Agent Tree Builder
Create `frontend/src/lib/services/agentTreeBuilder.ts`:
- [ ] Build tree structure from flat file list
- [ ] Link agent transcripts to parent messages
- [ ] Support arbitrary nesting depth

```typescript
export type AgentNode = {
  agentId: string;
  transcript: TranscriptMessage[];
  parentMessageId?: string;
  children: AgentNode[];
};

export function buildAgentTree(
  mainTranscript: TranscriptMessage[],
  agentFiles: FileDetail[]
): AgentNode {
  // Algorithm:
  // 1. Parse all agent transcripts
  // 2. Extract agent IDs from Task tool_use blocks
  // 3. Match agent files to tool_use_id
  // 4. Recursively build tree
}
```

#### 1.3: Component Structure
Create skeleton components:
- [ ] `TranscriptViewer.tsx` - Top-level viewer
- [ ] `MessageList.tsx` - List of messages
- [ ] `Message.tsx` - Single message renderer
- [ ] `ContentBlock.tsx` - Renders different content types
- [ ] `AgentPanel.tsx` - Nested agent view

**Deliverable:** Working infrastructure to fetch and parse transcripts

---

## Phase 2: Basic Message Display
**Duration:** 8-10 hours
**Dependencies:** Phase 1

### Objectives
- Render user and assistant text messages
- Basic conversational layout
- Message metadata (timestamp, role)

### Tasks

#### 2.1: Message Component
Implement `Message.tsx`:
- [ ] Detect message role (user vs assistant)
- [ ] Apply different styling for each role
  - User: Blue/purple, left-aligned
  - Assistant: Gray, right-aligned
- [ ] Display timestamp if available
- [ ] Handle multi-part content (array of blocks)

```tsx
// Message.tsx
import React from 'react';
import type { TranscriptMessage } from '../types/transcript';
import { ContentBlock } from './ContentBlock';

interface MessageProps {
  message: TranscriptMessage;
  index: number;
}

const formatTimestamp = (ts: string) => {
  // Format timestamp implementation
  return new Date(ts).toLocaleTimeString();
};

export const Message: React.FC<MessageProps> = ({ message, index }) => {
  return (
    <div className={`message message-${message.role}`}>
      <div className="message-header">
        <span className="role">{message.role}</span>
        {message.timestamp && (
          <span className="timestamp">{formatTimestamp(message.timestamp)}</span>
        )}
      </div>

      <div className="message-content">
        {message.content.map((block, idx) => (
          <ContentBlock key={idx} block={block} />
        ))}
      </div>
    </div>
  );
};
```

#### 2.2: Text Content Block
Implement basic text rendering in `ContentBlock.tsx`:
- [ ] Handle plain text blocks
- [ ] Preserve formatting (whitespace, newlines)
- [ ] Support markdown rendering (optional)
- [ ] Auto-link URLs

#### 2.3: Message List Layout
Implement `MessageList.tsx`:
- [ ] Render messages in chronological order
- [ ] Add visual separators between messages
- [ ] Implement virtualization for large transcripts (optional)
- [ ] Add "scroll to bottom" behavior

#### 2.4: Integration with RunCard
Update `RunCard.tsx`:
- [ ] Add "View Transcript" button/toggle
- [ ] Load and display TranscriptViewer
- [ ] Handle loading states
- [ ] Show message count

**Deliverable:** Can view basic user/assistant conversations

---

## Phase 3: Tool Use & Results
**Duration:** 10-12 hours
**Dependencies:** Phase 2

### Objectives
- Display tool calls with syntax highlighting
- Show tool results with proper formatting
- Handle success/error states
- Collapsible tool sections

### Tasks

#### 3.1: Tool Use Block Renderer
Create `ToolUseBlock.tsx`:
- [ ] Display tool name prominently
- [ ] Render input parameters as formatted JSON
- [ ] Add syntax highlighting (use Prism.js or Shiki)
- [ ] Make collapsible (start collapsed for large inputs)
- [ ] Add "Copy" button for parameters

```tsx
import React, { useState } from 'react';
import { CodeBlock } from './CodeBlock';

interface ToolUseBlockProps {
  block: {
    name: string;
    input: any;
  };
}

export const ToolUseBlock: React.FC<ToolUseBlockProps> = ({ block }) => {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="tool-use">
      <div className="tool-header" onClick={() => setExpanded(!expanded)}>
        <span className="tool-icon">🛠️</span>
        <span className="tool-name">{block.name}</span>
        <span className="expand-icon">{expanded ? '▼' : '▶'}</span>
      </div>

      {expanded && (
        <div className="tool-input">
          <CodeBlock
            language="json"
            code={JSON.stringify(block.input, null, 2)}
          />
        </div>
      )}
    </div>
  );
};
```

#### 3.2: Tool Result Block Renderer
Create `ToolResultBlock.tsx`:
- [ ] Match result to corresponding tool use (by tool_use_id)
- [ ] Display result content with syntax highlighting
- [ ] Distinguish success vs error states
  - Success: Green border/icon
  - Error: Red border/icon
- [ ] Handle large outputs (truncate with "Show more")
- [ ] Special handling for common tools:
  - Bash: Terminal-style output
  - Read: File preview
  - Grep: Highlighted matches

#### 3.3: Code Syntax Highlighting
- [ ] Install syntax highlighting library (`npm install shiki`)
- [ ] Create `CodeBlock.tsx` component
- [ ] Support multiple languages (typescript, python, bash, json, etc.)
- [ ] Add line numbers
- [ ] Add copy button

#### 3.4: Tool-Specific Renderers
Create specialized renderers for common tools:
- [ ] `BashOutput.tsx` - Terminal-style display
- [ ] `FileContent.tsx` - File viewer with line numbers
- [ ] `GrepResults.tsx` - Search results with highlights
- [ ] `ImageBlock.tsx` - Image viewer (if images in transcripts)

**Deliverable:** Full tool use/result visualization

---

## Phase 4: Thinking Blocks
**Duration:** 4-6 hours
**Dependencies:** Phase 2

### Objectives
- Display thinking blocks distinctly
- Allow toggling thinking visibility
- Maintain readability

### Tasks

#### 4.1: Thinking Block Renderer
Create `ThinkingBlock.tsx`:
- [ ] Distinctive visual style (italic, muted color)
- [ ] Collapsible by default (optional to show)
- [ ] Add icon (🤔 or brain emoji)
- [ ] Support markdown in thinking content

```tsx
import React, { useState } from 'react';
import { Markdown } from './Markdown';

interface ThinkingBlockProps {
  block: {
    content: string;
  };
}

export const ThinkingBlock: React.FC<ThinkingBlockProps> = ({ block }) => {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="thinking-block">
      <div className="thinking-header" onClick={() => setExpanded(!expanded)}>
        <span className="thinking-icon">💭</span>
        <span>Thinking...</span>
        <span>{expanded ? 'Hide' : 'Show'}</span>
      </div>

      {expanded && (
        <div className="thinking-content">
          <Markdown content={block.content} />
        </div>
      )}
    </div>
  );
};
```

#### 4.2: Global Thinking Toggle
Add viewer-level setting:
- [ ] "Show/Hide All Thinking" toggle in TranscriptViewer header
- [ ] Persist preference in localStorage
- [ ] Update all thinking blocks when toggled

**Deliverable:** Thinking blocks properly rendered and controllable

---

## Phase 5: Agent Nesting
**Duration:** 12-15 hours
**Dependencies:** Phase 1, 2, 3

### Objectives
- Display nested agent transcripts inline
- Support arbitrary nesting depth
- Clear visual hierarchy
- Link agents to parent tool calls

### Tasks

#### 5.1: Agent Panel Component
Create `AgentPanel.tsx`:
- [ ] Recursive component (can contain nested AgentPanels)
- [ ] Visual nesting (indentation, border, background color)
- [ ] Header showing agent ID and type
- [ ] Collapsible (start collapsed for deep nesting)
- [ ] Contains full MessageList

```tsx
import React, { useState } from 'react';
import type { AgentNode } from '../services/agentTreeBuilder';
import MessageList from './MessageList';

interface AgentPanelProps {
  agent: AgentNode;
  depth?: number;
}

export const AgentPanel: React.FC<AgentPanelProps> = ({ agent, depth = 0 }) => {
  const [expanded, setExpanded] = useState(depth < 2); // Auto-expand first 2 levels

  return (
    <div className="agent-panel" style={{ marginLeft: `${depth * 20}px` }}>
      <div className="agent-header" onClick={() => setExpanded(!expanded)}>
        <span className="agent-icon">🤖</span>
        <span className="agent-id">Agent {agent.agentId}</span>
        <span className="message-count">{agent.transcript.length} messages</span>
      </div>

      {expanded && (
        <div className="agent-content">
          <MessageList messages={agent.transcript} />

          {agent.children.map((child, index) => (
            <AgentPanel key={index} agent={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  );
};
```

#### 5.2: Agent Linking
- [ ] Identify Task tool_use blocks in main transcript
- [ ] Extract agent ID from tool parameters or result
- [ ] Load corresponding agent transcript file
- [ ] Insert AgentPanel at correct location in message flow
- [ ] Handle missing agent files gracefully

#### 5.3: Visual Hierarchy
Design visual system for nesting:
- [ ] Color-coded borders (depth 0: blue, depth 1: purple, depth 2: green, etc.)
- [ ] Progressive indentation
- [ ] Breadcrumb showing agent path (optional)
- [ ] "Collapse all agents" button

#### 5.4: Performance Optimization
- [ ] Lazy load agent transcripts (only when expanded)
- [ ] Virtualize long agent transcripts
- [ ] Memoize parsed transcripts
- [ ] Debounce expand/collapse animations

**Deliverable:** Full recursive agent viewing

---

## Phase 6: Search & Navigation
**Duration:** 8-10 hours
**Dependencies:** Phase 2, 3, 4, 5

### Objectives
- Full-text search across transcripts
- Jump to specific messages
- Keyboard navigation
- Highlight search results

### Tasks

#### 6.1: Search UI
- [ ] Add search bar to TranscriptViewer header
- [ ] Keyboard shortcut (Cmd+F / Ctrl+F)
- [ ] Show result count
- [ ] Next/Previous navigation buttons

#### 6.2: Search Implementation
Create `frontend/src/lib/services/transcriptSearch.ts`:
- [ ] Full-text search across:
  - Message text content
  - Tool names and parameters
  - Tool results
  - Thinking blocks (optional)
- [ ] Case-insensitive matching
- [ ] Regex support (optional)
- [ ] Search across all agents recursively

```typescript
export type SearchResult = {
  messageIndex: number;
  agentPath: string[]; // Path to nested agent
  matchText: string;
  context: string; // Surrounding text
};

export function searchTranscript(
  transcript: TranscriptMessage[],
  agents: AgentNode[],
  query: string
): SearchResult[] {
  // Search implementation
}
```

#### 6.3: Result Highlighting
- [ ] Highlight matching text in messages
- [ ] Auto-scroll to highlighted result
- [ ] Auto-expand collapsed sections containing results
- [ ] Visual indicator on collapsed messages with matches

#### 6.4: Keyboard Navigation
- [ ] Arrow keys to navigate between messages
- [ ] Enter to expand/collapse
- [ ] Escape to close search
- [ ] Tab to navigate between UI elements

**Deliverable:** Full search and navigation system

---

## Phase 7: Polish & UX Enhancements
**Duration:** 10-12 hours
**Dependencies:** All previous phases

### Objectives
- Professional-quality UI
- Smooth interactions
- Accessibility
- Mobile responsiveness

### Tasks

#### 7.1: Copy Functionality
- [ ] Copy individual messages
- [ ] Copy entire conversation
- [ ] Copy tool inputs/outputs
- [ ] Copy code blocks
- [ ] Show "Copied!" feedback

#### 7.2: Expand/Collapse All
- [ ] "Expand All" / "Collapse All" buttons
- [ ] Smart defaults (expand recent, collapse old)
- [ ] Persist expansion state in sessionStorage
- [ ] Animate transitions

#### 7.3: Timestamp Display
- [ ] Relative timestamps ("2 minutes ago")
- [ ] Absolute timestamps on hover
- [ ] Message duration (time between messages)
- [ ] Total conversation duration

#### 7.4: Message Linking
- [ ] Generate shareable links to specific messages
- [ ] Jump to message via URL hash (#message-123)
- [ ] Highlight linked message

#### 7.5: Accessibility
- [ ] ARIA labels for all interactive elements
- [ ] Keyboard-only navigation support
- [ ] Screen reader announcements
- [ ] Focus management
- [ ] Color contrast compliance (WCAG AA)

#### 7.6: Mobile Responsiveness
- [ ] Responsive message layout
- [ ] Touch-friendly expand/collapse
- [ ] Mobile-optimized search
- [ ] Reduced nesting indentation on small screens

#### 7.7: Loading States
- [ ] Skeleton screens while loading
- [ ] Progress indicators for large transcripts
- [ ] Error boundaries for parse failures
- [ ] Retry mechanisms

#### 7.8: Styling & Theming
- [ ] Consistent color palette
- [ ] Typography hierarchy
- [ ] Smooth animations
- [ ] Dark mode support (optional)
- [ ] Custom scrollbars

**Deliverable:** Production-ready transcript viewer

---

## Phase 8: Advanced Features (Optional)
**Duration:** 8-12 hours
**Dependencies:** Phase 7

### Optional Enhancements
- [ ] Export transcript as markdown
- [ ] Export as HTML
- [ ] Print view
- [ ] Diff view (compare two runs)
- [ ] Filter by message type
- [ ] Filter by tool name
- [ ] Annotate messages (comments/notes)
- [ ] Share specific message ranges
- [ ] Minimap/overview panel
- [ ] Token count display (if available)

---

## Technical Specifications

### Frontend Dependencies
```json
{
  "dependencies": {
    "shiki": "^1.0.0",           // Syntax highlighting
    "marked": "^11.0.0",          // Markdown rendering (optional)
    "date-fns": "^3.0.0"          // Date formatting
  }
}
```

### Component File Structure
```
frontend/src/lib/
├── components/
│   ├── transcript/
│   │   ├── TranscriptViewer.tsx
│   │   ├── MessageList.tsx
│   │   ├── Message.tsx
│   │   ├── ContentBlock.tsx
│   │   ├── blocks/
│   │   │   ├── TextBlock.tsx
│   │   │   ├── ToolUseBlock.tsx
│   │   │   ├── ToolResultBlock.tsx
│   │   │   ├── ThinkingBlock.tsx
│   │   │   └── CodeBlock.tsx
│   │   ├── AgentPanel.tsx
│   │   └── SearchBar.tsx
│   └── RunCard.tsx (update)
├── services/
│   ├── transcriptService.ts
│   ├── agentTreeBuilder.ts
│   └── transcriptSearch.ts
├── types/
│   └── transcript.ts
└── utils/
    └── transcriptUtils.ts
```

### Styling Approach
- Use CSS modules or scoped styles
- Consistent spacing scale (4px, 8px, 16px, 24px, 32px)
- Color variables for roles/states
- Responsive breakpoints (mobile: 640px, tablet: 768px, desktop: 1024px)

---

## Risk Assessment

### High Risk
1. **Schema Changes:** Claude Code transcript format may change
   - **Mitigation:** Version detection, graceful degradation
   - **Contingency:** Schema adapter layer

2. **Performance:** Large transcripts (1000+ messages) may be slow
   - **Mitigation:** Virtualization, lazy loading
   - **Contingency:** Pagination or "Load more" pattern

3. **Agent Complexity:** Deep nesting (5+ levels) may be confusing
   - **Mitigation:** Visual limits, "View in isolation" feature
   - **Contingency:** Flatten agent view option

### Medium Risk
4. **Browser Compatibility:** Modern features may not work everywhere
   - **Mitigation:** Target modern browsers only
   - **Contingency:** Polyfills or feature detection

5. **Content Security:** User-generated content in transcripts
   - **Mitigation:** Sanitize rendered content
   - **Contingency:** Strict CSP headers

### Low Risk
6. **Mobile UX:** Complex UI may not translate well to mobile
   - **Mitigation:** Mobile-first design
   - **Contingency:** Simplified mobile view

---

## Success Criteria

### Functional Requirements
- ✅ Display all message types correctly
- ✅ Support recursive agent viewing
- ✅ Search works across entire transcript tree
- ✅ Copy functionality works for all content types
- ✅ Performance acceptable for transcripts up to 1000 messages

### UX Requirements
- ✅ Intuitive navigation (users find what they need quickly)
- ✅ Professional appearance (matches or exceeds Claude Code UI)
- ✅ Responsive on mobile and desktop
- ✅ Accessible (keyboard navigation, screen readers)

### Technical Requirements
- ✅ Type-safe TypeScript implementation
- ✅ Component reusability (DRY)
- ✅ Test coverage for core parsing logic
- ✅ Performance monitoring (no jank, smooth scrolling)

---

## Timeline Summary

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 0: Schema Discovery | 2-3 hours | 3 hours |
| Phase 1: Core Infrastructure | 6-8 hours | 11 hours |
| Phase 2: Basic Messages | 8-10 hours | 21 hours |
| Phase 3: Tool Use & Results | 10-12 hours | 33 hours |
| Phase 4: Thinking Blocks | 4-6 hours | 39 hours |
| Phase 5: Agent Nesting | 12-15 hours | 54 hours |
| Phase 6: Search & Navigation | 8-10 hours | 64 hours |
| Phase 7: Polish & UX | 10-12 hours | 76 hours |
| **Total (Phases 0-7)** | **60-76 hours** | **~2 weeks full-time** |
| Phase 8: Advanced (Optional) | 8-12 hours | 88 hours |

---

## Getting Started

### Immediate Next Steps
1. **Run Phase 0:** Collect and analyze sample transcripts
2. **Create schema documentation**
3. **Define TypeScript types**
4. **Review plan with team/stakeholders**
5. **Begin Phase 1 implementation**

### Questions to Answer (Phase 0)
- What does a typical transcript message look like?
- How many different message types exist?
- How are agents linked to parent messages?
- What tools are most commonly used?
- What's the deepest agent nesting observed?
- Are there any undocumented message types?

---

## Maintenance Plan

### Ongoing Tasks
- Monitor Claude Code changelog for format changes
- Update schema documentation when changes detected
- Add new tool renderers as Claude Code adds tools
- Performance profiling and optimization
- User feedback incorporation

### Breaking Change Protocol
1. Detect format version (add to schema if missing)
2. Maintain backward compatibility for 2 versions
3. Show warning for deprecated formats
4. Provide migration guide for users

---

## Open Questions

1. Does Claude Code expose a transcript format version number?
2. Are there official docs for the transcript schema?
3. Should we support exporting/importing transcripts?
4. Should we cache parsed transcripts in IndexedDB?
5. How to handle extremely large transcripts (10,000+ messages)?
6. Should thinking blocks be visible by default?
7. What's the UI for comparing two transcripts (diff view)?

---

## Appendix: Example Code Snippets

### Message Parser
```typescript
// services/transcriptService.ts
export function parseTranscript(jsonl: string): TranscriptMessage[] {
  try {
    return jsonl
      .split('\n')
      .filter(line => line.trim())
      .map((line, index) => {
        try {
          return JSON.parse(line);
        } catch (e) {
          console.error(`Failed to parse line ${index + 1}:`, e);
          return null;
        }
      })
      .filter((msg): msg is TranscriptMessage => msg !== null);
  } catch (e) {
    console.error('Failed to parse transcript:', e);
    return [];
  }
}
```

### Agent Tree Builder (Conceptual)
```typescript
// services/agentTreeBuilder.ts
export async function buildAgentTree(
  run: RunDetail
): Promise<AgentNode> {
  // 1. Load main transcript
  const mainTranscriptFile = run.files.find(f => f.file_type === 'transcript');
  const mainMessages = await fetchAndParseTranscript(run.id, mainTranscriptFile.id);

  // 2. Find all Task tool uses
  const agentRefs = extractAgentReferences(mainMessages);

  // 3. Load agent transcripts
  const agentFiles = run.files.filter(f => f.file_type === 'agent');
  const agents = await Promise.all(
    agentRefs.map(ref => loadAgent(run.id, ref, agentFiles))
  );

  // 4. Build tree recursively
  return {
    agentId: 'main',
    transcript: mainMessages,
    children: agents
  };
}
```

---

**End of Implementation Plan**
