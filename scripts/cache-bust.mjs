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

    // Generate hash for CSS file - handle file not found directly
    let cssContent;
    try {
      cssContent = await fs.readFile(cssPath);
    } catch (error) {
      if (error.code === "ENOENT") {
        console.warn("⚠️  styles.css not found, skipping cache-busting");
        return;
      }
      throw error;
    }
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

    // Optional: Clean up old hashed files (but never delete current)
    await cleanupOldHashedFiles(hashedCssName);
  } catch (error) {
    console.error("❌ Cache-busting failed:", error.message);
    process.exit(1);
  }
}

async function cleanupOldHashedFiles(currentHashedName) {
  try {
    const files = await fs.readdir(STATIC_DIR);
    const cssHashPattern = /^styles\.[a-f0-9]{8}\.css$/;

    // Get all hashed files except the current one
    const hashedFiles = files
      .filter((file) => cssHashPattern.test(file) && file !== currentHashedName)
      .map((file) => ({ name: file, path: path.join(STATIC_DIR, file) }));

    // Keep only the 2 most recent older files (plus current = 3 total)
    if (hashedFiles.length > 2) {
      // Sort by file modification time (newest first)
      const filesWithStats = await Promise.all(
        hashedFiles.map(async (file) => {
          const stats = await fs.stat(file.path);
          return { ...file, mtime: stats.mtime };
        })
      );
      filesWithStats.sort((a, b) => b.mtime - a.mtime);

      const filesToDelete = filesWithStats.slice(2);

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
