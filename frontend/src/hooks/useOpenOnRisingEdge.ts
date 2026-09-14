import { useState } from 'react';

/**
 * Controlled open state for a `<details>` that auto-opens on the rising edge of
 * `trigger` (e.g. becoming the active search match) while staying
 * user-toggleable. Uses React's "adjust state on prop change" pattern, so the
 * open state lands in the same render as the trigger — no effect.
 */
export function useOpenOnRisingEdge(trigger: boolean): [boolean, (open: boolean) => void] {
  const [open, setOpen] = useState(false);
  const [prevTrigger, setPrevTrigger] = useState(false);
  if (trigger !== prevTrigger) {
    setPrevTrigger(trigger);
    if (trigger) setOpen(true);
  }
  return [open, setOpen];
}
