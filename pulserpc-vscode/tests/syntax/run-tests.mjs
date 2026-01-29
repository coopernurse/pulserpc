#!/usr/bin/env node

/**
 * Simple test runner for Pulse IDL grammar validation.
 *
 * This script validates that the grammar JSON is well-formed
 * and provides instructions for manual testing.
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const grammarPath = path.join(__dirname, '../../syntaxes/pulse.tmGrammar.json');

console.log('Pulse IDL Grammar Validation\n');

// Validate grammar JSON
try {
  const grammarContent = fs.readFileSync(grammarPath, 'utf8');
  const grammar = JSON.parse(grammarContent);

  console.log('✓ Grammar JSON is valid');
  console.log(`  - Name: ${grammar.name}`);
  console.log(`  - Scope: ${grammar.scopeName}`);
  console.log(`  - File types: ${grammar.fileTypes.join(', ')}`);
  console.log(`  - Patterns: ${grammar.patterns.length} top-level`);

  if (grammar.repository) {
    const repoKeys = Object.keys(grammar.repository);
    console.log(`  - Repository patterns: ${repoKeys.length}`);
    console.log(`    ${repoKeys.join(', ')}`);
  }
} catch (error) {
  console.error('✗ Grammar validation failed:');
  console.error(`  ${error.message}`);
  process.exit(1);
}

console.log('\n--- Manual Testing Instructions ---\n');
console.log('1. Open VS Code');
console.log('2. Press F5 (or Run > Start Debugging)');
console.log('3. In the new window, open any .pulse file from /workspace/examples/');
console.log('4. Use "Developer: Inspect Editor Tokens and Scopes" to verify');
console.log('   (Cmd+Shift+P on Mac, Ctrl+Shift+P on Windows/Linux)');
console.log('\nExpected scopes:');
console.log('  - Keywords: keyword.control.*.pulse');
console.log('  - Types: storage.type.*.pulse');
console.log('  - Modifiers: storage.modifier.*.pulse');
console.log('  - Entities: entity.name.*.pulse');
console.log('  - Comments: comment.line.*.pulse');

console.log('\n--- Test Files ---\n');
const testDir = path.join(__dirname, './');
const testFiles = fs.readdirSync(testDir).filter(f => f.endsWith('.test'));
testFiles.forEach(file => {
  console.log(`  - ${file}`);
});

console.log('\n✓ Validation complete\n');
