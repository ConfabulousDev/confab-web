import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminSettingsPageContent, type AdminSettingsPageContentProps } from './AdminSettingsPage';

const MOCK_DEFAULT_INSTRUCTIONS =
  `You are a senior software engineering observer analyzing a transcript of a Claude Code session.\n\nAnalyze the transcript to understand what happened during the session. Focus on:\n- What was the developer trying to accomplish?\n- What went well and what didn't?\n- Were there suggestions or patterns that could improve future sessions?\n\nBe concise and specific. Reference actual events from the transcript.`;

const MOCK_CUSTOM_INSTRUCTIONS =
  `You are a staff-level engineering mentor reviewing a Claude Code session for our team.\n\nFocus specifically on:\n- Code quality patterns and anti-patterns observed\n- Security implications of any changes\n- Test coverage suggestions\n- Performance considerations\n\nUse our team's coding standards as the baseline for evaluation. Be direct and actionable.`;

const MOCK_INPUT_FORMAT =
  `The transcript is provided as XML:\n<transcript>\n  Each line is a JSON object representing a message in the conversation.\n  Messages have roles: "human" (user) or "assistant" (Claude).\n</transcript>\n\n<session_stats>\n  Structured metadata about the session including token counts,\n  tool usage, duration, and other metrics.\n</session_stats>`;

const MOCK_OUTPUT_SCHEMA =
  `Output a JSON object with these fields:\n- recap: string (2-4 sentence summary of the session)\n- went_well: string[] (things that worked effectively)\n- went_bad: string[] (issues, errors, or inefficiencies)\n- human_suggestions: string[] (ways the human could improve their workflow)\n- environment_suggestions: string[] (tool/config improvements)\n- default_context_suggestions: string[] (CLAUDE.md or system prompt improvements)`;

const MOCK_EXAMPLE =
  `Example output:\n{\n  "recap": "The developer implemented a new authentication flow...",\n  "went_well": ["Iterative approach with testing"],\n  "went_bad": ["Initial approach had a security gap"],\n  "human_suggestions": ["Consider writing tests first"],\n  "environment_suggestions": ["Add ESLint security plugin"],\n  "default_context_suggestions": ["Document auth patterns in CLAUDE.md"]\n}\n\nOutput ONLY valid JSON. No markdown, no explanation.`;

// Wrapper to make the stories interactive
function InteractiveWrapper(baseProps: AdminSettingsPageContentProps) {
  const [edited, setEdited] = useState(baseProps.editedInstructions);
  const isDirty = edited !== baseProps.instructions;
  return (
    <AdminSettingsPageContent
      {...baseProps}
      editedInstructions={edited}
      onEditedInstructionsChange={setEdited}
      isDirty={isDirty}
    />
  );
}

const baseDefaultProps: AdminSettingsPageContentProps = {
  instructions: MOCK_DEFAULT_INSTRUCTIONS,
  isCustom: false,
  updatedAt: undefined,
  inputFormat: MOCK_INPUT_FORMAT,
  outputSchema: MOCK_OUTPUT_SCHEMA,
  example: MOCK_EXAMPLE,
  defaultInstructions: MOCK_DEFAULT_INSTRUCTIONS,
  editedInstructions: MOCK_DEFAULT_INSTRUCTIONS,
  onEditedInstructionsChange: () => {},
  isDirty: false,
  onSave: () => {},
  isSaving: false,
  onResetToDefault: () => {},
  onRegenerateClick: () => {},
  feedback: null,
  onFeedbackClose: () => {},
  showResetModal: false,
  onResetModalClose: () => {},
  onResetModalConfirm: () => {},
  isResetting: false,
  showRegenerateModal: false,
  onRegenerateModalClose: () => {},
  onRegenerateModalConfirm: () => {},
  isRegenerating: false,
  regenerateCount: null,
  regenerateCountLoading: false,
};

const meta: Meta<typeof AdminSettingsPageContent> = {
  title: 'Pages/Admin/AdminSettingsPage',
  component: AdminSettingsPageContent,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminSettingsPageContent>;

export const DefaultState: Story = {
  render: () => <InteractiveWrapper {...baseDefaultProps} />,
};

export const CustomState: Story = {
  render: () => (
    <InteractiveWrapper
      {...baseDefaultProps}
      instructions={MOCK_CUSTOM_INSTRUCTIONS}
      isCustom={true}
      updatedAt={new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString()}
      editedInstructions={MOCK_CUSTOM_INSTRUCTIONS}
    />
  ),
};

export const ExpandedFixedSections: Story = {
  name: 'Expanded Fixed Sections',
  render: () => (
    <AdminSettingsPageContent
      {...baseDefaultProps}
    />
  ),
};

export const ResetConfirmationModal: Story = {
  render: () => (
    <AdminSettingsPageContent
      {...baseDefaultProps}
      instructions={MOCK_CUSTOM_INSTRUCTIONS}
      isCustom={true}
      updatedAt={new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString()}
      editedInstructions={MOCK_CUSTOM_INSTRUCTIONS}
      showResetModal={true}
    />
  ),
};

export const RegenerateConfirmationModal: Story = {
  render: () => (
    <AdminSettingsPageContent
      {...baseDefaultProps}
      showRegenerateModal={true}
      regenerateCount={247}
      regenerateCountLoading={false}
    />
  ),
};
