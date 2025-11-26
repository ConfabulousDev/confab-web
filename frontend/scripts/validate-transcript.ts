#!/usr/bin/env npx tsx
/**
 * CLI tool to validate transcript JSONL files using the same Zod schemas as the frontend.
 *
 * Usage:
 *   npx tsx scripts/validate-transcript.ts <path-to-transcript.jsonl>
 *
 * Or from the frontend directory:
 *   npm run validate-transcript <path-to-transcript.jsonl>
 */

import * as fs from 'fs';
import * as path from 'path';
import { parseTranscriptLineWithError } from '../src/schemas/transcript';
import type { TranscriptValidationError, TranscriptLine } from '../src/schemas/transcript';

interface ValidationResult {
  messages: TranscriptLine[];
  errors: TranscriptValidationError[];
  totalLines: number;
  successCount: number;
  errorCount: number;
}

function parseJSONL(content: string): ValidationResult {
  const lines = content.split('\n').filter((line) => line.trim() !== '');
  const messages: TranscriptLine[] = [];
  const errors: TranscriptValidationError[] = [];

  for (let i = 0; i < lines.length; i++) {
    const result = parseTranscriptLineWithError(lines[i], i);
    if (result.success) {
      messages.push(result.data);
    } else {
      errors.push(result.error);
    }
  }

  return {
    messages,
    errors,
    totalLines: lines.length,
    successCount: messages.length,
    errorCount: errors.length,
  };
}

function formatError(error: TranscriptValidationError): string {
  const lines: string[] = [];

  // Header with line number and type
  const typeInfo = error.messageType ? ` (type: "${error.messageType}")` : '';
  lines.push(`\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`);
  lines.push(`Line ${error.line}${typeInfo}`);
  lines.push(`━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`);

  // Validation errors
  lines.push('\nValidation Errors:');
  for (const e of error.errors) {
    lines.push(`  • ${e.path}: ${e.message}`);
    if (e.expected && e.received) {
      lines.push(`    Expected: ${e.expected}`);
      lines.push(`    Received: ${e.received}`);
    }
  }

  // Pretty-printed raw JSON
  lines.push('\nRaw JSON:');
  try {
    const parsed = JSON.parse(error.rawJson);
    lines.push(JSON.stringify(parsed, null, 2).split('\n').map((l) => `  ${l}`).join('\n'));
  } catch {
    lines.push(`  ${error.rawJson}`);
  }

  return lines.join('\n');
}

function main() {
  const args = process.argv.slice(2);

  if (args.length === 0 || args.includes('--help') || args.includes('-h')) {
    console.log(`
Transcript JSONL Validator
==========================

Validates transcript JSONL files using the same Zod schemas as the Confab frontend.

Usage:
  npx tsx scripts/validate-transcript.ts <path-to-transcript.jsonl> [options]

Options:
  --json          Output results as JSON
  --summary       Only show summary (no individual errors)
  --first N       Show only first N errors (default: all)
  --help, -h      Show this help message

Examples:
  npx tsx scripts/validate-transcript.ts transcript.jsonl
  npx tsx scripts/validate-transcript.ts transcript.jsonl --first 5
  npx tsx scripts/validate-transcript.ts transcript.jsonl --json
`);
    process.exit(0);
  }

  const filePath = args[0];
  const jsonOutput = args.includes('--json');
  const summaryOnly = args.includes('--summary');
  const firstIndex = args.indexOf('--first');
  const maxErrors = firstIndex !== -1 && args[firstIndex + 1] ? parseInt(args[firstIndex + 1], 10) : Infinity;

  // Resolve the file path
  const resolvedPath = path.resolve(process.cwd(), filePath);

  if (!fs.existsSync(resolvedPath)) {
    console.error(`Error: File not found: ${resolvedPath}`);
    process.exit(1);
  }

  console.log(`Reading ${resolvedPath}...`);
  const content = fs.readFileSync(resolvedPath, 'utf-8');

  console.log('Validating transcript...\n');
  const result = parseJSONL(content);

  if (jsonOutput) {
    // Output as JSON for programmatic use
    const output = {
      file: resolvedPath,
      totalLines: result.totalLines,
      successCount: result.successCount,
      errorCount: result.errorCount,
      errors: result.errors.slice(0, maxErrors),
    };
    console.log(JSON.stringify(output, null, 2));
    process.exit(result.errorCount > 0 ? 1 : 0);
  }

  // Summary
  console.log('═══════════════════════════════════════════════════════════════════════════════');
  console.log('                            VALIDATION SUMMARY');
  console.log('═══════════════════════════════════════════════════════════════════════════════');
  console.log(`File:         ${resolvedPath}`);
  console.log(`Total lines:  ${result.totalLines}`);
  console.log(`Valid:        ${result.successCount} (${((result.successCount / result.totalLines) * 100).toFixed(1)}%)`);
  console.log(`Invalid:      ${result.errorCount} (${((result.errorCount / result.totalLines) * 100).toFixed(1)}%)`);
  console.log('═══════════════════════════════════════════════════════════════════════════════');

  if (result.errorCount === 0) {
    console.log('\n✅ All lines validated successfully!\n');
    process.exit(0);
  }

  if (summaryOnly) {
    console.log(`\n❌ ${result.errorCount} validation error(s) found. Use without --summary to see details.\n`);
    process.exit(1);
  }

  // Group errors by message type for analysis
  const errorsByType = new Map<string, number>();
  for (const error of result.errors) {
    const type = error.messageType ?? 'unknown';
    errorsByType.set(type, (errorsByType.get(type) ?? 0) + 1);
  }

  console.log('\nErrors by message type:');
  for (const [type, count] of Array.from(errorsByType.entries()).sort((a, b) => b[1] - a[1])) {
    console.log(`  ${type}: ${count}`);
  }

  // Show individual errors
  const errorsToShow = result.errors.slice(0, maxErrors);
  console.log(`\nShowing ${errorsToShow.length} of ${result.errorCount} error(s):`);

  for (const error of errorsToShow) {
    console.log(formatError(error));
  }

  if (result.errors.length > maxErrors) {
    console.log(`\n... and ${result.errors.length - maxErrors} more error(s). Use --first N to see more.\n`);
  }

  console.log(`\n❌ ${result.errorCount} validation error(s) found.\n`);
  process.exit(1);
}

main();
