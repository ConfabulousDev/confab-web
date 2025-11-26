import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import FormField from './FormField';

describe('FormField', () => {
  it('renders label correctly', () => {
    render(
      <FormField label="Email">
        <input type="email" />
      </FormField>
    );
    expect(screen.getByText('Email')).toBeInTheDocument();
  });

  it('renders children correctly', () => {
    render(
      <FormField label="Username">
        <input type="text" placeholder="Enter username" />
      </FormField>
    );
    expect(screen.getByPlaceholderText('Enter username')).toBeInTheDocument();
  });

  it('shows required indicator when required prop is true', () => {
    render(
      <FormField label="Password" required>
        <input type="password" />
      </FormField>
    );
    expect(screen.getByText('*')).toBeInTheDocument();
  });

  it('does not show required indicator when required prop is false', () => {
    render(
      <FormField label="Optional field">
        <input type="text" />
      </FormField>
    );
    expect(screen.queryByText('*')).not.toBeInTheDocument();
  });

  it('displays error message when error prop is provided', () => {
    render(
      <FormField label="Email" error="Invalid email address">
        <input type="email" />
      </FormField>
    );
    expect(screen.getByText('Invalid email address')).toBeInTheDocument();
  });

  it('does not display error when error prop is not provided', () => {
    render(
      <FormField label="Email">
        <input type="email" />
      </FormField>
    );
    // Check that error div is not present
    const errorDiv = document.querySelector('.error');
    expect(errorDiv).not.toBeInTheDocument();
  });

  it('renders with required indicator and error together', () => {
    render(
      <FormField label="Email" required error="This field is required">
        <input type="email" />
      </FormField>
    );
    expect(screen.getByText('*')).toBeInTheDocument();
    expect(screen.getByText('This field is required')).toBeInTheDocument();
  });
});
