# CSS Build Optimization Implementation Guide

## Overview

This document describes the comprehensive CSS build optimization pipeline implemented for the LDAP Manager Go/Templ/TailwindCSS application. The optimizations address frontend performance issues with CSS purging, production minification, asset versioning, and cache-busting strategies.

## Optimization Results

### Build Size Comparison

- **Development Build**: ~21 KB (readable formatting)
- **Production Build**: ~16 KB (minified and optimized)
- **Improvement**: ~22% size reduction

### Key Metrics

- **Classes Detected**: 107 utility classes
- **Media Queries**: 7 responsive breakpoints
- **CSS Layers**: 4 (@layer theme, base, components, utilities)
- **Custom Properties**: 39 CSS variables

## Build Pipeline Architecture

### Environment-Specific Builds

#### Development Mode

```bash
# Readable CSS with source maps and verbose output
pnpm css:build:dev
NODE_ENV=development postcss ./internal/web/tailwind.css -o ./internal/web/static/styles.css
```

#### Production Mode

```bash
# Optimized CSS with minification and cache-busting
pnpm css:build:prod
NODE_ENV=production postcss ./internal/web/tailwind.css -o ./internal/web/static/styles.css && node scripts/cache-bust.mjs
```

### PostCSS Configuration

**File**: `postcss.config.mjs`

The configuration automatically switches between development and production modes:

**Development**:

- Readable formatting
- Error reporting
- No minification

**Production**:

- Advanced CSS minification via cssnano
- Comment removal
- Whitespace normalization
- Color optimization
- Selector merging
- SVG optimization

### Tailwind Configuration Enhancements

**File**: `tailwind.config.js`

**Content Detection**:

- Templ templates: `internal/web/templates/*.templ`
- Go class constants: Custom Go extractor
- Enhanced templ extraction with class attribute parsing

**Purging Strategy**:

- Dynamic class safelist for status colors
- Content-based class detection
- Framework-specific extractors

### Cache-Busting System

**Script**: `scripts/cache-bust.mjs`

**Features**:

- MD5 hash generation for CSS files
- Asset manifest creation
- Automatic cleanup of old hashed files
- Integration with Go asset loading

**Manifest Format**:

```json
{
  "styles.css": "styles.1e22ce25.css",
  "generated": "2025-09-07T13:01:25.077Z",
  "hash": "1e22ce25"
}
```

### Go Asset Integration

**File**: `internal/web/assets.go`

**Asset Loading**:

```go
// Load manifest and get hashed asset paths
manifest := GetCachedManifest("/static/manifest.json")
stylesPath := manifest.GetStylesPath() // Returns "styles.1e22ce25.css"
```

**Template Integration**:

```go
// Enhanced base template with asset versioning support
templ baseWithAssets(title, stylesPath string) {
    // Uses dynamic asset path from manifest
    <link rel="stylesheet" href={ "/static/" + stylesPath }>
}
```

## Build Scripts

### Core Commands

```json
{
  "css:build:dev": "Development build with readable output",
  "css:build:prod": "Production build with optimization + cache-busting",
  "css:analyze": "Production build + size analysis report",
  "css:dev": "Development build with watch mode"
}
```

### Integrated Workflows

```json
{
  "build:assets:dev": "Dev build for all assets (CSS + Templ)",
  "build:assets:prod": "Production build for all assets",
  "dev": "Development server with watch mode",
  "build": "Full production build"
}
```

## Analysis and Monitoring

### CSS Size Analysis

**Script**: `scripts/analyze-css.mjs`

**Generated Report**: `claudedocs/css-analysis.md`

**Metrics Tracked**:

- File size and compression ratios
- Class count and usage patterns
- Media query optimization
- Performance assessment
- Size history tracking
- Optimization recommendations

### Performance Assessment Levels

- **Excellent**: < 10 KB
- **Good**: 10-25 KB
- **Fair**: 25-50 KB
- **Poor**: > 50 KB

## Deployment Integration

### Container Builds

```bash
# Production asset build in Dockerfile
RUN pnpm build:assets:prod
```

### Asset Serving

```go
// Go server uses manifest for cache-busted assets
http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("internal/web/static/"))))
```

### Template Rendering

```go
// Templates receive asset paths from manifest
manifest := web.GetCachedManifest("internal/web/static/manifest.json")
templates.BaseWithAssets(title, manifest.GetStylesPath())
```

## Development Workflow

### Local Development

1. `pnpm dev` - Start dev server with watch mode
2. CSS rebuilds automatically on changes
3. No cache-busting in development for faster rebuilds

### Production Builds

1. `pnpm build` - Full production build
2. Assets are hashed and manifest generated
3. Old hashed files automatically cleaned up

### Size Monitoring

1. `pnpm css:analyze` - Generate size report
2. Check `claudedocs/css-analysis.md` for metrics
3. Monitor bundle growth over time

## Performance Impact

### Frontend Loading

- **22% smaller CSS bundles** improve page load times
- **Cache-busting** ensures users get latest assets
- **Asset versioning** enables aggressive caching strategies

### Build Performance

- **Development builds**: ~500ms (fast iteration)
- **Production builds**: ~900ms (full optimization)
- **Watch mode**: Incremental rebuilds < 100ms

## Maintenance Notes

### Regular Tasks

- Monitor CSS bundle growth via analysis reports
- Review unused classes during major refactors
- Update PostCSS plugins for newer optimizations

### Troubleshooting

- Check `claudedocs/css-analysis.md` for size warnings
- Verify manifest.json generation in production
- Ensure Go asset loader handles manifest correctly

### Future Enhancements

- Implement critical CSS extraction
- Add CSS source map generation for production debugging
- Consider implementing CSS modules for component isolation
- Explore automated performance budgets

## Files Modified/Created

### Configuration Files

- ✅ `package.json` - Enhanced build scripts
- ✅ `postcss.config.mjs` - Environment-specific optimization
- ✅ `tailwind.config.js` - Enhanced content detection

### Build Scripts

- ✅ `scripts/cache-bust.mjs` - Asset versioning system
- ✅ `scripts/analyze-css.mjs` - Size monitoring and reporting

### Go Integration

- ✅ `internal/web/assets.go` - Asset manifest loader
- ✅ `internal/web/templates/base.templ` - Dynamic asset loading

### Documentation

- ✅ `claudedocs/css-analysis.md` - Generated size reports
- ✅ `claudedocs/css-build-optimization-guide.md` - This guide

The optimization pipeline provides a robust, production-ready CSS build system that balances performance, maintainability, and developer experience.
