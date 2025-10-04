# AGENTS.md — scripts/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2025-10-02 -->

## Overview

Utility scripts for build automation, asset optimization, and development workflows.

**Scripts:**

- `cache-bust.mjs` — CSS cache-busting with MD5 hashing
- `analyze-css.mjs` — CSS bundle analysis and performance reporting

## Setup & Environment

Scripts are Node.js ESM modules requiring:

```bash
# Node.js 18+ with ESM support
node --version  # Should be 18+

# No additional dependencies - uses native Node.js modules
# Scripts use: fs/promises, path, crypto
```

Scripts are automatically called by build processes:

- `cache-bust.mjs` — Called by `pnpm css:build:prod` (see package.json)
- `analyze-css.mjs` — Called by `pnpm css:analyze`

## Build & Tests

### Running Scripts

```bash
# Cache-busting (production CSS builds)
node scripts/cache-bust.mjs
# OR via pnpm script
pnpm css:build:prod  # Builds CSS + runs cache-bust

# CSS analysis and reporting
node scripts/analyze-css.mjs
# OR via pnpm script
pnpm css:analyze     # Analyzes built CSS bundle
```

### Testing Scripts

```bash
# Test cache-bust.mjs
pnpm css:build:prod
ls -lh internal/web/static/styles.*.css  # Should show hashed files
cat internal/web/static/manifest.json    # Should show mapping

# Test analyze-css.mjs
pnpm css:analyze
cat claudedocs/css-analysis.md           # Should show analysis report
```

## Code Style

### Node.js Script Standards

- Run `make format-js` (prettier) before commit
- Use ESM syntax (`import/export`), not CommonJS
- Always include shebang: `#!/usr/bin/env node`
- Use native Node.js modules when possible (avoid external deps)
- Proper error handling with exit codes (`process.exit(1)` on error)

### Script Structure

```javascript
#!/usr/bin/env node

import fs from "fs/promises";
import path from "path";
import { fileURLToPath } from "url";

// ESM __dirname equivalent
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

async function main() {
  try {
    // ... script logic
    console.log("✅ Success message");
  } catch (error) {
    console.error("❌ Error:", error.message);
    process.exit(1);
  }
}

main();
```

## Security

### Script Security Rules

- **Path validation**: Always use `path.join()` to prevent path traversal
- **Input sanitization**: Validate any external input or environment variables
- **No secrets**: Never hardcode credentials or tokens in scripts
- **File operations**: Use `fs.promises` for proper error handling
- **Exit codes**: Return non-zero exit code on errors for CI/CD detection

### File Operations Safety

```javascript
// ✅ Good: Safe path construction
const cssPath = path.join(__dirname, "../internal/web/static/styles.css");

// ❌ Bad: String concatenation (path traversal risk)
const cssPath = __dirname + "/../internal/web/static/" + filename;

// ✅ Good: Error handling with specific codes
try {
  await fs.readFile(cssPath);
} catch (error) {
  if (error.code === "ENOENT") {
    console.warn("File not found, skipping");
    return;
  }
  throw error; // Re-throw other errors
}
```

## PR & Commit Checklist

- [ ] Script has proper shebang (`#!/usr/bin/env node`)
- [ ] Uses ESM syntax (import/export)
- [ ] Proper error handling with exit codes
- [ ] Console output uses emojis/formatting for clarity
- [ ] Safe file operations (path.join, error handling)
- [ ] No hardcoded paths or secrets
- [ ] `make format-js` - code formatted
- [ ] Script tested manually before commit
- [ ] Updated package.json if adding new script command

## Examples: Good vs Bad

### ✅ Good: Safe file operations

```javascript
#!/usr/bin/env node

import fs from "fs/promises";
import path from "path";

async function processCss() {
  const cssPath = path.join(__dirname, "../internal/web/static/styles.css");

  try {
    const content = await fs.readFile(cssPath, "utf-8");
    console.log(`✅ Processed ${content.length} bytes`);
  } catch (error) {
    if (error.code === "ENOENT") {
      console.warn("⚠️  CSS file not found, skipping");
      return;
    }
    console.error("❌ Failed:", error.message);
    process.exit(1);
  }
}
```

### ❌ Bad: Unsafe operations

```javascript
// No shebang
// No error handling
// String concatenation for paths
const fs = require("fs"); // CommonJS instead of ESM
const path = __dirname + "/../static/" + process.argv[2]; // Unsafe path
const content = fs.readFileSync(path); // Sync operation, can block
console.log(content); // No success/error indication
```

### ✅ Good: Clean console output

```javascript
console.log("🔄 Starting cache-busting process...");
console.log(`✅ Created hashed CSS: ${hashedCssName}`);
console.warn("⚠️  Large bundle size detected");
console.error("❌ Cache-busting failed:", error.message);
```

### ❌ Bad: Unclear output

```javascript
console.log("starting"); // No context or visual indicators
console.log(hashedCssName); // Just dumps values
```

## When You're Stuck

1. **Script not executing**: Check shebang and file permissions (`chmod +x scripts/*.mjs`)
2. **Import errors**: Verify using ESM syntax and package.json has `"type": "module"`
3. **Path issues**: Use `path.join(__dirname, ...)` for all file paths
4. **Testing**: Run scripts directly with `node scripts/script-name.mjs`
5. **Integration**: Check package.json scripts for how scripts are called
6. **File not found**: Ensure required files exist before running (e.g., styles.css must exist before cache-bust)

## House Rules

- **ESM only**: All scripts use import/export, not require()
- **Native modules**: Prefer built-in Node.js modules over npm packages
- **Error codes**: Always exit with code 1 on errors for CI/CD detection
- **Console clarity**: Use emojis and formatting for clear output
- **Async/await**: Use modern async patterns, avoid callbacks
- **Safe paths**: Always use `path.join()` for file operations
- **Executable**: Scripts must have shebang and be chmod +x
