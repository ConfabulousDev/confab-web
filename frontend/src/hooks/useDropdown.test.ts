import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDropdown } from './useDropdown';

describe('useDropdown', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    // Clean up any event listeners
    vi.restoreAllMocks();
  });

  it('initializes with closed state', () => {
    const { result } = renderHook(() => useDropdown());

    expect(result.current.isOpen).toBe(false);
    expect(result.current.containerRef.current).toBeNull();
  });

  it('opens dropdown with setIsOpen(true)', () => {
    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });

    expect(result.current.isOpen).toBe(true);
  });

  it('closes dropdown with setIsOpen(false)', () => {
    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });
    expect(result.current.isOpen).toBe(true);

    act(() => {
      result.current.setIsOpen(false);
    });
    expect(result.current.isOpen).toBe(false);
  });

  it('toggles dropdown state', () => {
    const { result } = renderHook(() => useDropdown());

    expect(result.current.isOpen).toBe(false);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.isOpen).toBe(true);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.isOpen).toBe(false);
  });

  it('closes on escape key when open', () => {
    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });
    expect(result.current.isOpen).toBe(true);

    // Simulate escape key press
    act(() => {
      const event = new KeyboardEvent('keydown', { key: 'Escape' });
      document.dispatchEvent(event);
    });

    expect(result.current.isOpen).toBe(false);
  });

  it('does not close on other keys', () => {
    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });
    expect(result.current.isOpen).toBe(true);

    // Simulate other key press
    act(() => {
      const event = new KeyboardEvent('keydown', { key: 'Enter' });
      document.dispatchEvent(event);
    });

    expect(result.current.isOpen).toBe(true);
  });

  it('closes on click outside when open', () => {
    const { result } = renderHook(() => useDropdown<HTMLDivElement>());

    // Create a container element and attach ref
    const container = document.createElement('div');
    document.body.appendChild(container);

    // Manually set the ref (since we're not rendering a real component)
    Object.defineProperty(result.current.containerRef, 'current', {
      value: container,
      writable: true,
    });

    act(() => {
      result.current.setIsOpen(true);
    });
    expect(result.current.isOpen).toBe(true);

    // Simulate click outside the container
    act(() => {
      const event = new MouseEvent('mousedown', { bubbles: true });
      document.body.dispatchEvent(event);
    });

    expect(result.current.isOpen).toBe(false);

    // Cleanup
    document.body.removeChild(container);
  });

  it('does not close on click inside container', () => {
    const { result } = renderHook(() => useDropdown<HTMLDivElement>());

    // Create a container element and attach ref
    const container = document.createElement('div');
    document.body.appendChild(container);

    // Manually set the ref
    Object.defineProperty(result.current.containerRef, 'current', {
      value: container,
      writable: true,
    });

    act(() => {
      result.current.setIsOpen(true);
    });
    expect(result.current.isOpen).toBe(true);

    // Simulate click inside the container
    act(() => {
      const event = new MouseEvent('mousedown', { bubbles: true });
      container.dispatchEvent(event);
    });

    expect(result.current.isOpen).toBe(true);

    // Cleanup
    document.body.removeChild(container);
  });

  it('does not add event listeners when closed', () => {
    const addEventListenerSpy = vi.spyOn(document, 'addEventListener');

    renderHook(() => useDropdown());

    // No listeners should be added when dropdown is closed
    expect(addEventListenerSpy).not.toHaveBeenCalledWith('mousedown', expect.any(Function));
    expect(addEventListenerSpy).not.toHaveBeenCalledWith('keydown', expect.any(Function));
  });

  it('adds event listeners when opened', () => {
    const addEventListenerSpy = vi.spyOn(document, 'addEventListener');

    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });

    expect(addEventListenerSpy).toHaveBeenCalledWith('mousedown', expect.any(Function));
    expect(addEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function));
  });

  it('removes event listeners when closed', () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener');

    const { result } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });

    act(() => {
      result.current.setIsOpen(false);
    });

    expect(removeEventListenerSpy).toHaveBeenCalledWith('mousedown', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function));
  });

  it('cleans up event listeners on unmount', () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener');

    const { result, unmount } = renderHook(() => useDropdown());

    act(() => {
      result.current.setIsOpen(true);
    });

    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith('mousedown', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function));
  });
});
