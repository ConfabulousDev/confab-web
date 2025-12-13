import type { Meta, StoryObj } from '@storybook/react-vite';
import FormField from './FormField';

const meta: Meta<typeof FormField> = {
  title: 'Components/FormField',
  component: FormField,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '300px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof FormField>;

export const Default: Story = {
  args: {
    label: 'Email',
    children: <input type="email" placeholder="you@example.com" style={{ width: '100%', padding: '8px' }} />,
  },
};

export const Required: Story = {
  args: {
    label: 'Username',
    required: true,
    children: <input type="text" placeholder="Enter username" style={{ width: '100%', padding: '8px' }} />,
  },
};

export const WithError: Story = {
  args: {
    label: 'Email',
    error: 'Please enter a valid email address',
    children: <input type="email" defaultValue="invalid-email" style={{ width: '100%', padding: '8px' }} />,
  },
};

export const RequiredWithError: Story = {
  args: {
    label: 'Password',
    required: true,
    error: 'Password must be at least 8 characters',
    children: <input type="password" defaultValue="short" style={{ width: '100%', padding: '8px' }} />,
  },
};

export const WithTextarea: Story = {
  args: {
    label: 'Description',
    children: (
      <textarea
        placeholder="Enter a description..."
        rows={4}
        style={{ width: '100%', padding: '8px', resize: 'vertical' }}
      />
    ),
  },
};

export const WithSelect: Story = {
  args: {
    label: 'Country',
    required: true,
    children: (
      <select style={{ width: '100%', padding: '8px' }}>
        <option value="">Select a country</option>
        <option value="us">United States</option>
        <option value="uk">United Kingdom</option>
        <option value="ca">Canada</option>
      </select>
    ),
  },
};

export const MultipleFields: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <FormField label="First Name" required>
        <input type="text" placeholder="John" style={{ width: '100%', padding: '8px' }} />
      </FormField>
      <FormField label="Last Name" required>
        <input type="text" placeholder="Doe" style={{ width: '100%', padding: '8px' }} />
      </FormField>
      <FormField label="Email" required error="This email is already registered">
        <input type="email" defaultValue="john@example.com" style={{ width: '100%', padding: '8px' }} />
      </FormField>
    </div>
  ),
};
