import { describe, it, expect } from 'vitest';
import {
  computeKeyFingerprint,
  buildUnknownReportIssue,
  buildUnknownReportUrl,
  type UnknownDescriptor,
} from './reportUnknown';

describe('computeKeyFingerprint', () => {
  it('lists top-level key names', () => {
    expect(computeKeyFingerprint({ type: 'x', timestamp: 't' }).sort()).toEqual([
      'timestamp',
      'type',
    ]);
  });

  it('includes one level of nesting for object-valued keys', () => {
    const fp = computeKeyFingerprint({
      type: 'response_item',
      payload: { type: 'foo', call_id: '1' },
    });
    expect(fp).toContain('payload.type');
    expect(fp).toContain('payload.call_id');
  });

  it('does not recurse arrays into index keys', () => {
    const fp = computeKeyFingerprint({ items: [1, 2, 3] });
    expect(fp).toContain('items');
    expect(fp).not.toContain('items.0');
  });

  it('returns [] for null and non-object inputs', () => {
    expect(computeKeyFingerprint(null)).toEqual([]);
    expect(computeKeyFingerprint('str')).toEqual([]);
    expect(computeKeyFingerprint(42)).toEqual([]);
    expect(computeKeyFingerprint(undefined)).toEqual([]);
  });
});

const baseDescriptor: UnknownDescriptor = {
  provider: 'codex',
  surface: 'line',
  type: 'future_payload_type',
  reason: 'unrecognized response_item payload type',
  keyFingerprint: ['type', 'payload', 'payload.type'],
};

describe('buildUnknownReportIssue', () => {
  it('title carries the parser-gap prefix, provider and type', () => {
    const { title } = buildUnknownReportIssue(baseDescriptor);
    expect(title).toContain('[parser-gap]');
    expect(title).toContain('codex');
    expect(title).toContain('future_payload_type');
  });

  it('body contains provider, type, reason and key names', () => {
    const { body } = buildUnknownReportIssue(baseDescriptor);
    expect(body).toContain('codex');
    expect(body).toContain('future_payload_type');
    expect(body).toContain('unrecognized response_item payload type');
    expect(body).toContain('payload.type');
  });

  it('includes the Confab version only when a non-empty version is supplied', () => {
    expect(buildUnknownReportIssue(baseDescriptor, '1.2.3').body).toContain('1.2.3');
    expect(buildUnknownReportIssue(baseDescriptor, '').body.toLowerCase()).not.toContain(
      'confab version',
    );
    expect(buildUnknownReportIssue(baseDescriptor).body.toLowerCase()).not.toContain(
      'confab version',
    );
  });

  it('omits the reason line when reason is absent but keeps the type', () => {
    const { body } = buildUnknownReportIssue({ ...baseDescriptor, reason: undefined });
    expect(body).not.toContain('unrecognized response_item payload type');
    expect(body).toContain('future_payload_type');
  });

  it('does not include a "what were you doing" free-text prompt', () => {
    const { body } = buildUnknownReportIssue(baseDescriptor, '1.2.3');
    expect(body.toLowerCase()).not.toContain('what were you doing');
  });
});

describe('redaction contract', () => {
  it('never leaks payload values into the URL — only key names', () => {
    const descriptor: UnknownDescriptor = {
      provider: 'claude',
      surface: 'message',
      type: 'unknown_type',
      keyFingerprint: computeKeyFingerprint({
        type: 'unknown_type',
        secretText: 'SENSITIVE_PAYLOAD_VALUE',
        nested: { apiKey: 'sk-LEAK-12345' },
      }),
    };
    const url = buildUnknownReportUrl(descriptor, '1.0.0');

    expect(url).not.toContain('SENSITIVE_PAYLOAD_VALUE');
    expect(url).not.toContain('sk-LEAK-12345');

    const body = new URL(url).searchParams.get('body') ?? '';
    expect(body).toContain('secretText');
    expect(body).toContain('nested.apiKey');
  });

  it('produces a URL with no labels param (CF-574 sets none)', () => {
    expect(new URL(buildUnknownReportUrl(baseDescriptor)).searchParams.has('labels')).toBe(false);
  });

  it('points the URL at issues/new', () => {
    expect(buildUnknownReportUrl(baseDescriptor)).toContain('/issues/new?');
  });
});
