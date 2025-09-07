#!/usr/bin/env node

import fs from "fs/promises";
import path from "path";
import crypto from "crypto";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const STATIC_DIR = path.join(__dirname, "../internal/web/static");
const TEMPLATES_DIR = path.join(__dirname, "../internal/web/templates");
const MANIFEST_PATH = path.join(STATIC_DIR, "manifest.json");

async function generateCacheBustedAssets() {
  try {
    console.log("🔄 Starting cache-busting process...");

    const cssPath = path.join(STATIC_DIR, "styles.css");
    const cssExists = await fs
      .access(cssPath)
      .then(() => true)
      .catch(() => false);

    if (!cssExists) {
      console.warn("⚠️  styles.css not found, skipping cache-busting");
      return;
    }

    // Generate hash for CSS file
    const cssContent = await fs.readFile(cssPath);
    const cssHash = crypto.createHash("md5").update(cssContent).digest("hex").substring(0, 8);
    const hashedCssName = `styles.${cssHash}.css`;
    const hashedCssPath = path.join(STATIC_DIR, hashedCssName);

    // Copy CSS with hash
    await fs.copyFile(cssPath, hashedCssPath);

    // Create/update manifest
    const manifest = {
      "styles.css": hashedCssName,
      generated: new Date().toISOString(),
      hash: cssHash
    };

    await fs.writeFile(MANIFEST_PATH, JSON.stringify(manifest, null, 2));

    console.log(`✅ Created hashed CSS: ${hashedCssName}`);
    console.log(`📝 Updated manifest: ${MANIFEST_PATH}`);

    // Optional: Clean up old hashed files
    await cleanupOldHashedFiles();
  } catch (error) {
    console.error("❌ Cache-busting failed:", error.message);
    process.exit(1);
  }
}

async function cleanupOldHashedFiles() {
  try {
    const files = await fs.readdir(STATIC_DIR);
    const cssHashPattern = /^styles\.[a-f0-9]{8}\.css$/;

    // Keep only the most recent 3 hashed CSS files
    const hashedFiles = files
      .filter((file) => cssHashPattern.test(file))
      .map((file) => ({ name: file, path: path.join(STATIC_DIR, file) }));

    if (hashedFiles.length > 3) {
      const filesToDelete = hashedFiles.slice(0, -3); // Keep last 3

      for (const file of filesToDelete) {
        try {
          await fs.unlink(file.path);
          console.log(`🗑️  Cleaned up old file: ${file.name}`);
        } catch (error) {
          console.warn(`⚠️  Could not delete ${file.name}:`, error.message);
        }
      }
    }
  } catch (error) {
    console.warn("⚠️  Cleanup warning:", error.message);
  }
}

// Run cache-busting
generateCacheBustedAssets();
